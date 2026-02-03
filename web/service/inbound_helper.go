package service

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"x-ui/database/model"
	"x-ui/logger"
	"x-ui/xray"

	"gorm.io/gorm"
)

// =============================================================================
// 缓存管理
// =============================================================================

func (s *InboundService) getParsedSettings(inboundId int, settingsStr string) map[string]any {
	s.cacheMutex.RLock()
	if s.settingsCache != nil {
		cached, exists := s.settingsCache[inboundId]
		if exists {
			s.cacheMutex.RUnlock()
			return cached
		}
	}
	s.cacheMutex.RUnlock()

	s.cacheMutex.Lock()
	defer s.cacheMutex.Unlock()

	if s.settingsCache == nil {
		s.settingsCache = make(map[int]map[string]any)
	}

	var settings map[string]any
	err := json.Unmarshal([]byte(settingsStr), &settings)
	if err != nil {
		return nil
	}

	s.settingsCache[inboundId] = settings

	return settings
}

func (s *InboundService) invalidateSettingsCache(inboundId int) {
	s.cacheMutex.Lock()
	delete(s.settingsCache, inboundId)
	s.cacheMutex.Unlock()
}

// =============================================================================
// 验证函数
// =============================================================================

func (s *InboundService) checkPortExist(listen string, port int, ignoreId int) (bool, error) {
	return s.getInboundRepo().CheckPortExist(listen, port, ignoreId)
}

func (s *InboundService) getAllEmails() ([]string, error) {
	return s.getInboundRepo().GetAllEmails()
}

func (s *InboundService) contains(slice []string, str string) bool {
	lowerStr := strings.ToLower(str)
	for _, s := range slice {
		if strings.ToLower(s) == lowerStr {
			return true
		}
	}
	return false
}

func (s *InboundService) checkEmailsExistForClients(clients []model.Client) (string, error) {
	allEmails, err := s.getAllEmails()
	if err != nil {
		return "", err
	}
	var emails []string
	for _, client := range clients {
		if client.Email != "" {
			if s.contains(emails, client.Email) {
				return client.Email, nil
			}
			if s.contains(allEmails, client.Email) {
				return client.Email, nil
			}
			emails = append(emails, client.Email)
		}
	}
	return "", nil
}

func (s *InboundService) checkEmailExistForInbound(inbound *model.Inbound) (string, error) {
	clients, err := s.GetClients(inbound)
	if err != nil {
		return "", err
	}
	allEmails, err := s.getAllEmails()
	if err != nil {
		return "", err
	}
	var emails []string
	for _, client := range clients {
		if client.Email != "" {
			if s.contains(emails, client.Email) {
				return client.Email, nil
			}
			if s.contains(allEmails, client.Email) {
				return client.Email, nil
			}
			emails = append(emails, client.Email)
		}
	}
	return "", nil
}

// =============================================================================
// 数据库迁移
// =============================================================================

func (s *InboundService) MigrationRemoveOrphanedTraffics() {
	db := s.getInboundRepo().GetDB()
	db.Exec(`
		DELETE FROM client_traffics
		WHERE email NOT IN (
			SELECT JSON_EXTRACT(client.value, '$.email')
			FROM inbounds,
				JSON_EACH(JSON_EXTRACT(inbounds.settings, '$.clients')) AS client
		)
	`)
}

func (s *InboundService) MigrationRequirements() {
	db := s.getInboundRepo().GetDB()
	tx := db.Begin()
	var err error
	defer func() {
		if err == nil {
			tx.Commit()
			if dbErr := db.Exec(`VACUUM "main"`).Error; dbErr != nil {
				logger.Warningf("VACUUM failed: %v", dbErr)
			}
		} else {
			tx.Rollback()
		}
	}()

	// Calculate and backfill all_time from up+down for inbounds and clients
	err = tx.Exec(`
		UPDATE inbounds
		SET all_time = IFNULL(up, 0) + IFNULL(down, 0)
		WHERE IFNULL(all_time, 0) = 0 AND (IFNULL(up, 0) + IFNULL(down, 0)) > 0
	`).Error
	if err != nil {
		return
	}
	err = tx.Exec(`
		UPDATE client_traffics
		SET all_time = IFNULL(up, 0) + IFNULL(down, 0)
		WHERE IFNULL(all_time, 0) = 0 AND (IFNULL(up, 0) + IFNULL(down, 0)) > 0
	`).Error
	if err != nil {
		return
	}

	// Fix inbounds based problems
	var inbounds []*model.Inbound
	err = tx.Model(model.Inbound{}).Where("protocol IN (?)", []string{"vmess", "vless", "trojan"}).Find(&inbounds).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return
	}
	for inbound_index := range inbounds {
		settings := map[string]any{}
		_ = json.Unmarshal([]byte(inbounds[inbound_index].Settings), &settings)
		clients, ok := settings["clients"].([]any)
		if ok {
			// Fix Client configuration problems
			var newClients []any
			for client_index := range clients {
				c := clients[client_index].(map[string]any)

				// Add email='' if it is not exists
				if _, ok := c["email"]; !ok {
					c["email"] = ""
				}

				// Convert string tgId to int64
				if _, ok := c["tgId"]; ok {
					tgId := c["tgId"]
					if tgIdStr, ok2 := tgId.(string); ok2 {
						tgIdInt64, err := strconv.ParseInt(strings.ReplaceAll(tgIdStr, " ", ""), 10, 64)
						if err == nil {
							c["tgId"] = tgIdInt64
						}
					}
				}

				// Update VLESS flow to xtls-rprx-vision if deprecated or empty
				isVLESS := inbounds[inbound_index].Protocol == model.VLESS
				isTCP := false
				isTLSOrReality := false

				if isVLESS && len(inbounds[inbound_index].StreamSettings) > 0 {
					var stream map[string]any
					if err := json.Unmarshal([]byte(inbounds[inbound_index].StreamSettings), &stream); err == nil {
						if net, ok := stream["network"].(string); ok && net == "tcp" {
							isTCP = true
						}
						if sec, ok := stream["security"].(string); ok {
							if sec == "tls" || sec == "reality" {
								isTLSOrReality = true
							}
						}
					}
				}

				// Remove "flow": "xtls-rprx-direct" logic updated to:
				if isVLESS && isTCP && isTLSOrReality {
					flow, _ := c["flow"].(string)
					// If flow is empty or one of the deprecated values
					if flow == "" || flow == "xtls-rprx-direct" || flow == "xtls-rprx-origin" {
						c["flow"] = "xtls-rprx-vision"
					}
				} else if _, ok := c["flow"]; ok {
					// For other protocols/transports, clear deprecated flow if present
					if c["flow"] == "xtls-rprx-direct" || c["flow"] == "xtls-rprx-origin" {
						c["flow"] = ""
					}
				}
				// Backfill created_at and updated_at
				if _, ok := c["created_at"]; !ok {
					c["created_at"] = time.Now().Unix() * 1000
				}
				c["updated_at"] = time.Now().Unix() * 1000

				// 回填 speedLimit，如果不存在设为 0，确保旧数据有字段，避免显示和配置问题
				if _, ok := c["speedLimit"]; !ok {
					c["speedLimit"] = 0
				}

				newClients = append(newClients, any(c))
			}
			settings["clients"] = newClients
			modifiedSettings, err := json.MarshalIndent(settings, "", "  ")
			if err != nil {
				return
			}

			inbounds[inbound_index].Settings = string(modifiedSettings)
		}

		// Add client traffic row for all clients which has email
		modelClients, err := s.GetClients(inbounds[inbound_index])
		if err != nil {
			return
		}
		for _, modelClient := range modelClients {
			if len(modelClient.Email) > 0 {
				var count int64
				tx.Model(xray.ClientTraffic{}).Where("email = ?", modelClient.Email).Count(&count)
				if count == 0 {
					_ = s.AddClientStat(tx, inbounds[inbound_index].Id, &modelClient)
				}
			}
		}
	}
	tx.Save(inbounds)
	for _, inbound := range inbounds {
		s.invalidateSettingsCache(inbound.Id)
	}

	// Remove orphaned traffics
	tx.Where("inbound_id = 0").Delete(xray.ClientTraffic{})

	// Migrate old MultiDomain to External Proxy
	var externalProxy []struct {
		Id             int
		Port           int
		StreamSettings []byte
	}
	err = tx.Raw(`select id, port, stream_settings
	from inbounds
	WHERE protocol in ('vmess','vless','trojan')
	  AND json_extract(stream_settings, '$.security') = 'tls'
	  AND json_extract(stream_settings, '$.tlsSettings.settings.domains') IS NOT NULL`).Scan(&externalProxy).Error
	if err != nil || len(externalProxy) == 0 {
		return
	}

	for _, ep := range externalProxy {
		var reverses any
		var stream map[string]any
		_ = json.Unmarshal(ep.StreamSettings, &stream)
		if tlsSettings, ok := stream["tlsSettings"].(map[string]any); ok {
			if settings, ok := tlsSettings["settings"].(map[string]any); ok {
				if domains, ok := settings["domains"].([]any); ok {
					for _, domain := range domains {
						if domainMap, ok := domain.(map[string]any); ok {
							domainMap["forceTls"] = "same"
							domainMap["port"] = ep.Port
							domainMap["dest"] = domainMap["domain"].(string)
							delete(domainMap, "domain")
						}
					}
				}
				reverses = settings["domains"]
				delete(settings, "domains")
			}
		}
		stream["externalProxy"] = reverses
		newStream, _ := json.MarshalIndent(stream, " ", "  ")
		tx.Model(model.Inbound{}).Where("id = ?", ep.Id).Update("stream_settings", newStream)
	}

	err = tx.Raw(`UPDATE inbounds
	SET tag = REPLACE(tag, '0.0.0.0:', '')
	WHERE INSTR(tag, '0.0.0.0:') > 0;`).Error
	if err != nil {
		return
	}
}

func (s *InboundService) MigrateDB() {
	s.MigrationRequirements()
	s.MigrationRemoveOrphanedTraffics()
}
