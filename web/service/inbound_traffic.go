package service

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"x-ui/database"
	"x-ui/database/model"
	"x-ui/logger"
	"x-ui/xray"

	"gorm.io/gorm"
)

// =============================================================================
// 流量添加与更新
// =============================================================================

func (s *InboundService) AddTraffic(inboundTraffics []*xray.Traffic, clientTraffics []*xray.ClientTraffic) (error, bool) {
	needRestart, err := database.WithTxResult(func(tx *gorm.DB) (bool, error) {
		if err := s.addInboundTraffic(tx, inboundTraffics); err != nil {
			return false, err
		}
		if err := s.addClientTraffic(tx, clientTraffics); err != nil {
			return false, err
		}

		needRestart0, count0, err := s.autoRenewClients(tx)
		if err != nil {
			logger.Warning("autoRenewClients error:", err)
		} else if count0 > 0 {
			logger.Debugf("autoRenewClients: %d", count0)
		}
		needRestart1, count1, err := s.disableInvalidInbounds(tx)
		if err != nil {
			logger.Warning("disableInvalidInbounds error:", err)
		} else if count1 > 0 {
			logger.Debugf("disableInvalidInbounds: %d", count1)
		}
		needRestart2, count2, err := s.disableInvalidClients(tx)
		if err != nil {
			logger.Warning("disableInvalidClients error:", err)
		} else if count2 > 0 {
			logger.Debugf("disableInvalidClients: %d", count2)
		}

		return needRestart0 || needRestart1 || needRestart2, nil
	})
	return err, needRestart
}

func (s *InboundService) addInboundTraffic(tx *gorm.DB, traffics []*xray.Traffic) error {
	if len(traffics) == 0 {
		return nil
	}

	var tags []string
	for _, traffic := range traffics {
		if !traffic.IsInbound {
			continue
		}
		if traffic.Up == 0 && traffic.Down == 0 {
			continue
		}

		err := tx.Model(&model.Inbound{}).
			Where("tag = ?", traffic.Tag).
			Updates(map[string]interface{}{
				"up":       gorm.Expr("up + ?", traffic.Up),
				"down":     gorm.Expr("down + ?", traffic.Down),
				"all_time": gorm.Expr("all_time + ?", traffic.Up+traffic.Down),
			}).Error
		if err != nil {
			logger.Warning("Error updating inbound traffic for tag", traffic.Tag, ":", err)
			return err
		}

		tags = append(tags, traffic.Tag)
	}

	if len(tags) > 0 {
		query := `UPDATE inbounds SET enable = false
					WHERE tag IN (?)
					AND enable = true
					AND total > 0
					AND (up + down) >= total`
		return tx.Exec(query, tags).Error
	}

	return nil
}

func (s *InboundService) addClientTraffic(tx *gorm.DB, traffics []*xray.ClientTraffic) (err error) {
	if len(traffics) == 0 {
		// Empty onlineUsers
		if s.xrayService != nil {
			s.xrayService.SetOnlineClients(nil)
		}
		return nil
	}

	var onlineUsers []string
	for _, traffic := range traffics {
		onlineUsers = append(onlineUsers, traffic.Email)
	}
	if s.xrayService != nil {
		s.xrayService.SetOnlineClients(onlineUsers)
	}

	dbClientTraffics, err := s.adjustTraffics(tx, traffics)
	if err != nil {
		return err
	}

	for _, traffic := range dbClientTraffics {
		if traffic.Up == 0 && traffic.Down == 0 {
			continue
		}
		err := tx.Model(&xray.ClientTraffic{}).
			Where("email = ?", traffic.Email).
			Updates(map[string]interface{}{
				"up":          gorm.Expr("up + ?", traffic.Up),
				"down":        gorm.Expr("down + ?", traffic.Down),
				"all_time":    gorm.Expr("all_time + ?", traffic.Up+traffic.Down),
				"last_online": time.Now().Unix(),
			}).Error
		if err != nil {
			logger.Warning("Error updating client traffic for email", traffic.Email, ":", err)
			return err
		}
	}

	return nil
}

func (s *InboundService) adjustTraffics(tx *gorm.DB, dbClientTraffics []*xray.ClientTraffic) ([]*xray.ClientTraffic, error) {
	inboundIds := make([]int, 0, len(dbClientTraffics))
	for _, dbClientTraffic := range dbClientTraffics {
		if dbClientTraffic.ExpiryTime < 0 {
			inboundIds = append(inboundIds, dbClientTraffic.InboundId)
		}
	}
	if len(inboundIds) == 0 {
		return dbClientTraffics, nil
	}
	// Fetch inbounds where Id in inboundIds
	var inbounds []*model.Inbound
	err := tx.Model(model.Inbound{}).Where("id IN (?)", inboundIds).Find(&inbounds).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}
	// Map from inbound id to expiry time
	inboundExpiryMap := make(map[int]int64)
	for _, inbound := range inbounds {
		inboundExpiryMap[inbound.Id] = inbound.ExpiryTime
	}
	// Update dbClientTraffics
	for _, dbClientTraffic := range dbClientTraffics {
		if dbClientTraffic.ExpiryTime < 0 {
			inboundExpiry := inboundExpiryMap[dbClientTraffic.InboundId]
			// Calculate reset time in days based on negative expiry time of dbClientTraffic
			resetDays := int64(-dbClientTraffic.ExpiryTime / 86400000)
			if resetDays <= 0 {
				resetDays = 1
			}
			expiryTimeAdjusted := inboundExpiry
			if expiryTimeAdjusted > 0 {
				// Calculate how many days since inboundExpiry
				now := time.Now().Unix() * 1000
				diff := now - inboundExpiry
				if diff > 0 {
					// Adjust expiry time by adding resetDays to inboundExpiry until we pass now
					addMs := resetDays * 86400000
					for expiryTimeAdjusted < now {
						expiryTimeAdjusted += addMs
					}
				}
			}
			dbClientTraffic.ExpiryTime = expiryTimeAdjusted
		}
	}
	return dbClientTraffics, nil
}

// =============================================================================
// 自动化任务
// =============================================================================

func (s *InboundService) autoRenewClients(tx *gorm.DB) (bool, int64, error) {
	// check for time expired
	var traffics []*xray.ClientTraffic
	now := time.Now().Unix() * 1000

	err := tx.Model(xray.ClientTraffic{}).Where("reset > 0 and expiry_time > 0 and expiry_time <= ?", now).Find(&traffics).Error
	if err != nil {
		return false, 0, err
	}

	// nothing to renew
	if len(traffics) == 0 {
		return false, 0, nil
	}

	needRestart := false
	for _, traffic := range traffics {
		inbound, err := s.GetInbound(traffic.InboundId)
		if err != nil {
			return false, 0, err
		}
		clients, err := s.GetClients(inbound)
		if err != nil {
			return false, 0, err
		}

		// get settings
		var settings map[string]any
		err = json.Unmarshal([]byte(inbound.Settings), &settings)
		if err != nil {
			return false, 0, err
		}
		settingsClients := settings["clients"].([]any)

		for _, client := range clients {
			if client.Email == traffic.Email {
				// If traffic used

				var newExpiryTime int64
				var newUp int64
				var newDown int64
				var newEnable bool
				changed := false

				if traffic.Total > 0 && (traffic.Up+traffic.Down) >= traffic.Total {
					// if client is not enabled but needs to be reset
					if !traffic.Enable {
						newExpiryTime = 0
						newUp = 0
						newDown = 0
						newEnable = true
						changed = true
					}
				} else if traffic.ExpiryTime > 0 && traffic.ExpiryTime <= now {
					// if time expired
					newExpiryTime = traffic.ExpiryTime + (int64(traffic.Reset) * 86400000)
					newUp = 0
					newDown = 0
					newEnable = true
					changed = true
				}

				if changed {
					// Update traffic
					updateMap := map[string]any{
						"expiry_time": newExpiryTime,
						"up":          newUp,
						"down":        newDown,
						"enable":      newEnable,
					}
					err = tx.Model(xray.ClientTraffic{}).Where("email = ?", traffic.Email).Updates(updateMap).Error
					if err != nil {
						return false, 0, err
					}

					// Update client settings
					for client_index := range settingsClients {
						c := settingsClients[client_index].(map[string]any)
						if c["email"] == client.Email {
							c["expiryTime"] = newExpiryTime
							c["updated_at"] = time.Now().Unix() * 1000
							settingsClients[client_index] = c
							break
						}
					}
					settings["clients"] = settingsClients
					modifiedSettings, err := json.MarshalIndent(settings, "", "  ")
					if err != nil {
						return false, 0, err
					}
					inbound.Settings = string(modifiedSettings)
					s.invalidateSettingsCache(inbound.Id)

					// Send notification
					if s.tgService != nil && s.tgService.IsRunning() {
						msg := fmt.Sprintf("🔄 Client Renewed:\nEmail: %s\nNew Expiry: %s",
							traffic.Email,
							time.Unix(newExpiryTime/1000, 0).Format("2006-01-02 15:04:05"))
						_ = s.tgService.SendMessage(msg)
					}

					needRestart = true
				}
				break
			}
		}
		// Save inbound changes
		err = tx.Save(inbound).Error
		if err != nil {
			return false, 0, err
		}
	}

	return needRestart, int64(len(traffics)), nil
}

func (s *InboundService) disableInvalidInbounds(tx *gorm.DB) (bool, int64, error) {
	now := time.Now().Unix() * 1000
	needRestart := false

	result := tx.Model(model.Inbound{}).
		Where("enable = ? and expiry_time > 0 and expiry_time <= ?", true, now).
		Update("enable", false)
	err := result.Error
	if err != nil {
		return false, 0, err
	}
	count := result.RowsAffected
	if count > 0 {
		needRestart = true
	}

	return needRestart, count, err
}

func (s *InboundService) disableInvalidClients(tx *gorm.DB) (bool, int64, error) {
	now := time.Now().Unix() * 1000
	needRestart := false

	result := tx.Model(xray.ClientTraffic{}).
		Where("enable = ? and reset = 0 and ((total > 0 and (up + down) >= total) or (expiry_time > 0 and expiry_time <= ?))", true, now).
		Update("enable", false)
	err := result.Error
	if err != nil {
		return false, 0, err
	}
	count := result.RowsAffected
	if count > 0 {
		needRestart = true
	}

	return needRestart, count, err
}

// =============================================================================
// 流量查询
// =============================================================================

func (s *InboundService) GetClientTrafficByEmail(email string) (traffic *xray.ClientTraffic, err error) {
	traffic, err = s.getClientTrafficRepo().FindByEmail(email)
	if err != nil {
		logger.Warningf("Error retrieving ClientTraffic with email %s: %v", email, err)
		return nil, err
	}
	return traffic, nil
}

func (s *InboundService) UpdateClientTrafficByEmail(email string, upload int64, download int64) error {
	err := s.getClientTrafficRepo().UpdateTraffic(email, upload, download)
	if err != nil {
		logger.Warningf("Error updating ClientTraffic with email %s: %v", email, err)
		return err
	}
	return nil
}

func (s *InboundService) GetClientTrafficByID(id string) ([]xray.ClientTraffic, error) {
	db := s.getInboundRepo().GetDB()
	var traffics []xray.ClientTraffic

	err := db.Model(xray.ClientTraffic{}).Where(`email IN(
		SELECT JSON_EXTRACT(client.value, '$.email') as email
		FROM inbounds,
	  	JSON_EACH(JSON_EXTRACT(inbounds.settings, '$.clients')) AS client
		WHERE
	  	JSON_EXTRACT(client.value, '$.id') in (?)
		)`, id).Find(&traffics).Error
	if err != nil {
		logger.Debug(err)
		return nil, err
	}
	return traffics, err
}

func (s *InboundService) GetClientTrafficTgBot(tgId int64) ([]*xray.ClientTraffic, error) {
	db := s.getInboundRepo().GetDB()
	var inbounds []*model.Inbound

	// Retrieve inbounds where settings contain the given tgId
	err := db.Model(model.Inbound{}).Where("settings LIKE ?", fmt.Sprintf(`%%"tgId": %d%%`, tgId)).Find(&inbounds).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		logger.Errorf("Error retrieving inbounds with tgId %d: %v", tgId, err)
		return nil, err
	}

	var emails []string
	for _, inbound := range inbounds {
		clients, err := s.GetClients(inbound)
		if err != nil {
			logger.Errorf("Error retrieving clients for inbound %d: %v", inbound.Id, err)
			continue
		}
		for _, client := range clients {
			if client.TgID == tgId {
				emails = append(emails, client.Email)
			}
		}
	}

	var traffics []*xray.ClientTraffic
	err = db.Model(xray.ClientTraffic{}).Where("email IN ?", emails).Find(&traffics).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			logger.Warning("No ClientTraffic records found for emails:", emails)
			return nil, nil
		}
		logger.Errorf("Error retrieving ClientTraffic for emails %v: %v", emails, err)
		return nil, err
	}

	return traffics, nil
}

func (s *InboundService) SearchClientTraffic(query string) (traffic *xray.ClientTraffic, err error) {
	db := s.getInboundRepo().GetDB()
	inbound := &model.Inbound{}
	traffic = &xray.ClientTraffic{}

	// Search for inbound settings that contain the query
	err = db.Model(model.Inbound{}).Where("settings LIKE ?", "%\""+query+"\"%").First(inbound).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			logger.Warningf("Inbound settings containing query %s not found: %v", query, err)
			return nil, err
		}
		logger.Errorf("Error searching for inbound settings with query %s: %v", query, err)
		return nil, err
	}

	traffic.InboundId = inbound.Id

	// Unmarshal settings to get clients
	settings := map[string][]model.Client{}
	if err := json.Unmarshal([]byte(inbound.Settings), &settings); err != nil {
		logger.Errorf("Error unmarshalling inbound settings for inbound ID %d: %v", inbound.Id, err)
		return nil, err
	}

	clients := settings["clients"]
	for _, client := range clients {
		if (client.ID == query || client.Password == query) && client.Email != "" {
			traffic.Email = client.Email
			break
		}
	}

	if traffic.Email == "" {
		logger.Warningf("No client found with query %s in inbound ID %d", query, inbound.Id)
		return nil, gorm.ErrRecordNotFound
	}

	// Retrieve ClientTraffic based on the found email
	err = db.Model(xray.ClientTraffic{}).Where("email = ?", traffic.Email).First(traffic).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			logger.Warningf("ClientTraffic for email %s not found: %v", traffic.Email, err)
			return nil, err
		}
		logger.Errorf("Error retrieving ClientTraffic for email %s: %v", traffic.Email, err)
		return nil, err
	}

	return traffic, nil
}

// =============================================================================
// 流量重置
// =============================================================================

func (s *InboundService) ResetClientTrafficByEmail(clientEmail string) error {
	return s.getClientTrafficRepo().ResetByEmail(clientEmail)
}

func (s *InboundService) ResetClientTraffic(id int, clientEmail string) (bool, error) {
	needRestart := false

	traffic, err := s.GetClientTrafficByEmail(clientEmail)
	if err != nil {
		return false, err
	}

	if !traffic.Enable {
		inbound, err := s.GetInbound(id)
		if err != nil {
			return false, err
		}
		clients, err := s.GetClients(inbound)
		if err != nil {
			return false, err
		}
		for _, client := range clients {
			if client.Email == clientEmail && client.Enable {
				if s.xrayService != nil {
					_ = s.xrayApi.Init(s.xrayService.GetApiPort())
				}
				cipher := ""
				if string(inbound.Protocol) == "shadowsocks" {
					var oldSettings map[string]any
					err = json.Unmarshal([]byte(inbound.Settings), &oldSettings)
					if err != nil {
						return false, err
					}
					cipher = oldSettings["method"].(string)
				}
				err1 := s.xrayApi.AddUser(string(inbound.Protocol), inbound.Tag, map[string]any{
					"email":    client.Email,
					"id":       client.ID,
					"security": client.Security,
					"flow":     client.Flow,
					"password": client.Password,
					"cipher":   cipher,
				})
				if err1 == nil {
					logger.Debug("Client enabled due to reset traffic:", clientEmail)
				} else {
					logger.Debug("Error in enabling client by api:", err1)
					needRestart = true
				}
				s.xrayApi.Close()
				break
			}
		}
	}

	traffic.Up = 0
	traffic.Down = 0
	traffic.Enable = true

	err = s.getClientTrafficRepo().Update(traffic)
	if err != nil {
		return false, err
	}

	return needRestart, nil
}

func (s *InboundService) ResetAllClientTraffics(id int) error {
	return s.getClientTrafficRepo().ResetByInboundID(id)
}

func (s *InboundService) ResetAllTraffics() error {
	return s.getInboundRepo().ResetAllTraffics()
}

// =============================================================================
// 客户端统计
// =============================================================================

func (s *InboundService) AddClientStat(tx *gorm.DB, inboundId int, client *model.Client) error {
	clientTraffic := xray.ClientTraffic{}
	clientTraffic.InboundId = inboundId
	clientTraffic.Email = client.Email
	clientTraffic.Total = client.TotalGB
	clientTraffic.ExpiryTime = client.ExpiryTime
	clientTraffic.Enable = true
	clientTraffic.Up = 0
	clientTraffic.Down = 0
	clientTraffic.Reset = client.Reset
	err := tx.Create(&clientTraffic).Error
	return err
}

func (s *InboundService) UpdateClientStat(tx *gorm.DB, email string, client *model.Client) error {
	result := tx.Model(xray.ClientTraffic{}).
		Where("email = ?", email).
		Updates(map[string]any{
			"email":       client.Email,
			"total":       client.TotalGB,
			"expiry_time": client.ExpiryTime,
			"reset":       client.Reset,
		})
	err := result.Error
	return err
}

func (s *InboundService) DelClientStat(tx *gorm.DB, email string) error {
	return tx.Where("email = ?", email).Delete(xray.ClientTraffic{}).Error
}

func (s *InboundService) GetClientsLastOnline() (map[string]int64, error) {
	return s.getClientTrafficRepo().GetLastOnline()
}

func (s *InboundService) FilterAndSortClientEmails(emails []string) ([]string, []string, error) {
	db := s.getInboundRepo().GetDB()

	// Step 1: Get ClientTraffic records for emails in the input list
	var clients []xray.ClientTraffic
	err := db.Where("email IN ?", emails).Find(&clients).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, nil, err
	}

	// Step 2: Sort clients by (Up + Down) descending
	sort.Slice(clients, func(i, j int) bool {
		return (clients[i].Up + clients[i].Down) > (clients[j].Up + clients[j].Down)
	})

	// Step 3: Extract sorted valid emails and track found ones
	validEmails := make([]string, 0, len(clients))
	found := make(map[string]bool)
	for _, client := range clients {
		validEmails = append(validEmails, client.Email)
		found[client.Email] = true
	}

	// Step 4: Identify emails that were not found in the database
	extraEmails := make([]string, 0)
	for _, email := range emails {
		if !found[email] {
			extraEmails = append(extraEmails, email)
		}
	}

	return validEmails, extraEmails, nil
}

func (s *InboundService) GetOnlineClients() []string {
	if s.xrayService == nil {
		return nil
	}
	return s.xrayService.GetOnlineClients()
}
