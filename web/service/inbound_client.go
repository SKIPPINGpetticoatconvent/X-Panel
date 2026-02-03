package service

import (
	"encoding/json"
	"strings"
	"time"

	"x-ui/database/model"
	"x-ui/logger"
	"x-ui/util/common"
	"x-ui/xray"

	"gorm.io/gorm"
)

// =============================================================================
// 客户端查询
// =============================================================================

func (s *InboundService) GetClients(inbound *model.Inbound) ([]model.Client, error) {
	settings := s.getParsedSettings(inbound.Id, inbound.Settings)
	if settings == nil {
		return nil, common.ErrInvalidInput
	}

	clientsInterface, ok := settings["clients"]
	if !ok {
		return nil, nil
	}
	clientsAny, ok := clientsInterface.([]any)
	if !ok {
		return nil, common.ErrInvalidInput
	}
	clients := make([]model.Client, len(clientsAny))
	for i, c := range clientsAny {
		cMap, ok := c.(map[string]any)
		if !ok {
			continue
		}
		jsonBytes, _ := json.Marshal(cMap)
		_ = json.Unmarshal(jsonBytes, &clients[i])
	}
	return clients, nil
}

func (s *InboundService) GetClientInboundByTrafficID(trafficId int) (traffic *xray.ClientTraffic, inbound *model.Inbound, err error) {
	traffic, err = s.getClientTrafficRepo().FindByID(trafficId)
	if err != nil {
		logger.Warningf("Error retrieving ClientTraffic with trafficId %d: %v", trafficId, err)
		return nil, nil, err
	}
	if traffic != nil {
		inbound, err = s.GetInbound(traffic.InboundId)
		return traffic, inbound, err
	}
	return nil, nil, nil
}

func (s *InboundService) GetClientInboundByEmail(email string) (traffic *xray.ClientTraffic, inbound *model.Inbound, err error) {
	traffic, err = s.getClientTrafficRepo().FindByEmail(email)
	if err != nil {
		logger.Warningf("Error retrieving ClientTraffic with email %s: %v", email, err)
		return nil, nil, err
	}
	if traffic != nil {
		inbound, err = s.GetInbound(traffic.InboundId)
		return traffic, inbound, err
	}
	return nil, nil, nil
}

func (s *InboundService) GetClientByEmail(clientEmail string) (*xray.ClientTraffic, *model.Client, error) {
	traffic, inbound, err := s.GetClientInboundByEmail(clientEmail)
	if err != nil {
		return nil, nil, err
	}
	if inbound == nil {
		return nil, nil, common.NewError("Inbound Not Found For Email:", clientEmail)
	}

	clients, err := s.GetClients(inbound)
	if err != nil {
		return nil, nil, err
	}

	for _, client := range clients {
		if client.Email == clientEmail {
			return traffic, &client, nil
		}
	}

	return nil, nil, common.NewError("Client Not Found In Inbound For Email:", clientEmail)
}

func (s *InboundService) checkIsEnabledByEmail(clientEmail string) (bool, error) {
	_, inbound, err := s.GetClientInboundByEmail(clientEmail)
	if err != nil {
		return false, err
	}
	if inbound == nil {
		return false, common.NewError("Inbound Not Found For Email:", clientEmail)
	}

	clients, err := s.GetClients(inbound)
	if err != nil {
		return false, err
	}

	isEnable := false

	for _, client := range clients {
		if client.Email == clientEmail {
			isEnable = client.Enable
			break
		}
	}

	return isEnable, err
}

// =============================================================================
// 客户端 CRUD
// =============================================================================

func (s *InboundService) AddInboundClient(data *model.Inbound) (bool, error) {
	clients, err := s.GetClients(data)
	if err != nil {
		return false, err
	}

	existingEmails, err := s.checkEmailsExistForClients(clients)
	if err != nil {
		return false, err
	}
	if existingEmails != "" {
		return false, common.NewError("Duplicate email: ", existingEmails)
	}

	db := s.getInboundRepo().GetDB()
	tx := db.Begin()

	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()

	oldInbound, err := s.GetInbound(data.Id)
	if err != nil {
		return false, err
	}

	var oldSettings map[string]any
	err = json.Unmarshal([]byte(oldInbound.Settings), &oldSettings)
	if err != nil {
		return false, err
	}

	oldClients := oldSettings["clients"].([]any)

	var newSettings map[string]any
	err = json.Unmarshal([]byte(data.Settings), &newSettings)
	if err != nil {
		return false, err
	}

	newClients := newSettings["clients"].([]any)

	// Merge clients
	allClients := append(oldClients, newClients...)
	oldSettings["clients"] = allClients

	modifiedSettings, err := json.MarshalIndent(oldSettings, "", "  ")
	if err != nil {
		return false, err
	}

	oldInbound.Settings = string(modifiedSettings)
	s.invalidateSettingsCache(oldInbound.Id)

	needRestart := false

	// Add client stats for new clients
	for i := range clients {
		err = s.AddClientStat(tx, oldInbound.Id, &clients[i])
		if err != nil {
			return false, err
		}

		// Add user using xray api
		if s.IsXrayApiAvailable() {
			cipher := ""
			if string(oldInbound.Protocol) == "shadowsocks" {
				cipher = oldSettings["method"].(string)
			}
			err1 := s.xrayApi.AddUser(string(oldInbound.Protocol), oldInbound.Tag, map[string]any{
				"email":    clients[i].Email,
				"id":       clients[i].ID,
				"security": clients[i].Security,
				"flow":     clients[i].Flow,
				"password": clients[i].Password,
				"cipher":   cipher,
			})
			if err1 != nil {
				logger.Debug("Error in adding client by xray api:", err1)
				needRestart = true
			}
		} else {
			needRestart = true
		}
	}

	err = tx.Save(oldInbound).Error
	if err != nil {
		return false, err
	}

	return needRestart, err
}

func (s *InboundService) DelInboundClient(inboundId int, clientId string) (bool, error) {
	oldInbound, err := s.GetInbound(inboundId)
	if err != nil {
		logger.Error("Load Old Data Error")
		return false, err
	}
	var oldSettings map[string]any
	err = json.Unmarshal([]byte(oldInbound.Settings), &oldSettings)
	if err != nil {
		return false, err
	}

	oldClients := oldSettings["clients"].([]any)

	db := s.getInboundRepo().GetDB()
	tx := db.Begin()

	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()

	needRestart := false

	var newClients []any
	for _, client := range oldClients {
		c := client.(map[string]any)
		id := ""
		switch oldInbound.Protocol {
		case "trojan":
			id = c["password"].(string)
		case "shadowsocks":
			id = c["email"].(string)
		default:
			id = c["id"].(string)
		}
		if id == clientId {
			email := c["email"].(string)

			// Remove client stat
			err = s.DelClientStat(tx, email)
			if err != nil {
				return false, err
			}

			// Remove client IPs
			err = s.DelClientIPs(tx, email)
			if err != nil {
				return false, err
			}

			// Remove client using xray api
			if s.IsXrayApiAvailable() {
				err1 := s.xrayApi.RemoveUser(oldInbound.Tag, email)
				if err1 != nil {
					logger.Debug("Error in removing client by xray api:", err1)
					needRestart = true
				}
			} else {
				needRestart = true
			}
		} else {
			newClients = append(newClients, client)
		}
	}

	if len(newClients) == 0 {
		return needRestart, common.NewError("Cannot delete all clients. Please delete the inbound instead.")
	}

	oldSettings["clients"] = newClients

	modifiedSettings, err := json.MarshalIndent(oldSettings, "", "  ")
	if err != nil {
		return false, err
	}

	oldInbound.Settings = string(modifiedSettings)
	s.invalidateSettingsCache(oldInbound.Id)

	err = tx.Save(oldInbound).Error
	if err != nil {
		return false, err
	}

	return needRestart, err
}

func (s *InboundService) UpdateInboundClient(data *model.Inbound, clientId string) (bool, error) {
	clients, err := s.GetClients(data)
	if err != nil {
		return false, err
	}

	oldInbound, err := s.GetInbound(data.Id)
	if err != nil {
		return false, err
	}

	oldClients, err := s.GetClients(oldInbound)
	if err != nil {
		return false, err
	}

	// Find old email
	oldEmail := ""
	for _, oldClient := range oldClients {
		switch data.Protocol {
		case "trojan":
			if oldClient.Password == clientId {
				oldEmail = oldClient.Email
			}
		case "shadowsocks":
			if oldClient.Email == clientId {
				oldEmail = oldClient.Email
			}
		default:
			if oldClient.ID == clientId {
				oldEmail = oldClient.Email
			}
		}
	}

	if oldEmail == "" {
		return false, common.NewError("Client not found")
	}

	// Check for duplicate email if email changed
	for _, client := range clients {
		if client.Email != oldEmail {
			existingEmails, err := s.checkEmailsExistForClients([]model.Client{client})
			if err != nil {
				return false, err
			}
			if existingEmails != "" {
				return false, common.NewError("Duplicate email: ", existingEmails)
			}
		}
	}

	db := s.getInboundRepo().GetDB()
	tx := db.Begin()

	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()

	var oldSettings map[string]any
	err = json.Unmarshal([]byte(oldInbound.Settings), &oldSettings)
	if err != nil {
		return false, err
	}

	var newSettings map[string]any
	err = json.Unmarshal([]byte(data.Settings), &newSettings)
	if err != nil {
		return false, err
	}

	settingsClients := oldSettings["clients"].([]any)
	newClients := newSettings["clients"].([]any)

	needRestart := false

	for i, client := range settingsClients {
		c := client.(map[string]any)
		id := ""
		switch data.Protocol {
		case "trojan":
			id = c["password"].(string)
		case "shadowsocks":
			id = c["email"].(string)
		default:
			id = c["id"].(string)
		}
		if id == clientId {
			// Update client
			if len(newClients) > 0 {
				settingsClients[i] = newClients[0]

				// Update client stat
				for _, newClient := range clients {
					err = s.UpdateClientStat(tx, oldEmail, &newClient)
					if err != nil {
						return false, err
					}

					// Update client IPs if email changed
					if oldEmail != newClient.Email {
						err = s.UpdateClientIPs(tx, oldEmail, newClient.Email)
						if err != nil {
							return false, err
						}
					}

					// Update using xray api
					if s.IsXrayApiAvailable() {
						// Remove old user first
						_ = s.xrayApi.RemoveUser(oldInbound.Tag, oldEmail)

						// Add new user
						cipher := ""
						if string(oldInbound.Protocol) == "shadowsocks" {
							cipher = oldSettings["method"].(string)
						}
						err1 := s.xrayApi.AddUser(string(oldInbound.Protocol), oldInbound.Tag, map[string]any{
							"email":    newClient.Email,
							"id":       newClient.ID,
							"security": newClient.Security,
							"flow":     newClient.Flow,
							"password": newClient.Password,
							"cipher":   cipher,
						})
						if err1 != nil {
							logger.Debug("Error in updating client by xray api:", err1)
							needRestart = true
						}
					} else {
						needRestart = true
					}
				}
			}
			break
		}
	}

	oldSettings["clients"] = settingsClients
	modifiedSettings, err := json.MarshalIndent(oldSettings, "", "  ")
	if err != nil {
		return false, err
	}

	oldInbound.Settings = string(modifiedSettings)
	s.invalidateSettingsCache(oldInbound.Id)

	err = tx.Save(oldInbound).Error
	if err != nil {
		return false, err
	}

	return needRestart, err
}

func (s *InboundService) updateClientTraffics(tx *gorm.DB, oldInbound *model.Inbound, newInbound *model.Inbound) error {
	oldClients, err := s.GetClients(oldInbound)
	if err != nil {
		return err
	}
	newClients, err := s.GetClients(newInbound)
	if err != nil {
		return err
	}

	oldEmailMap := make(map[string]model.Client)
	for _, c := range oldClients {
		oldEmailMap[c.Email] = c
	}

	for _, newClient := range newClients {
		if _, exists := oldEmailMap[newClient.Email]; !exists {
			// New client
			err = s.AddClientStat(tx, newInbound.Id, &newClient)
			if err != nil {
				return err
			}
		} else {
			// Update existing client
			err = s.UpdateClientStat(tx, newClient.Email, &newClient)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// =============================================================================
// 客户端设置修改
// =============================================================================

func (s *InboundService) SetClientTelegramUserID(trafficId int, tgId int64) (bool, error) {
	traffic, inbound, err := s.GetClientInboundByTrafficID(trafficId)
	if err != nil {
		return false, err
	}
	if inbound == nil {
		return false, common.NewError("Inbound Not Found For Traffic ID:", trafficId)
	}

	clientEmail := traffic.Email

	oldClients, err := s.GetClients(inbound)
	if err != nil {
		return false, err
	}

	clientId := ""

	for _, oldClient := range oldClients {
		if oldClient.Email == clientEmail {
			switch inbound.Protocol {
			case "trojan":
				clientId = oldClient.Password
			case "shadowsocks":
				clientId = oldClient.Email
			default:
				clientId = oldClient.ID
			}
			break
		}
	}

	if len(clientId) == 0 {
		return false, common.NewError("Client Not Found For Email:", clientEmail)
	}

	var settings map[string]any
	err = json.Unmarshal([]byte(inbound.Settings), &settings)
	if err != nil {
		return false, err
	}
	clients := settings["clients"].([]any)
	var newClients []any
	for client_index := range clients {
		c := clients[client_index].(map[string]any)
		if c["email"] == clientEmail {
			c["tgId"] = tgId
			c["updated_at"] = time.Now().Unix() * 1000
			newClients = append(newClients, any(c))
		}
	}
	settings["clients"] = newClients
	modifiedSettings, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return false, err
	}
	inbound.Settings = string(modifiedSettings)
	needRestart, err := s.UpdateInboundClient(inbound, clientId)
	if err == nil {
		s.invalidateSettingsCache(inbound.Id)
	}
	return needRestart, err
}

func (s *InboundService) ToggleClientEnableByEmail(clientEmail string) (bool, bool, error) {
	_, inbound, err := s.GetClientInboundByEmail(clientEmail)
	if err != nil {
		return false, false, err
	}
	if inbound == nil {
		return false, false, common.NewError("Inbound Not Found For Email:", clientEmail)
	}

	oldClients, err := s.GetClients(inbound)
	if err != nil {
		return false, false, err
	}

	clientId := ""
	clientOldEnabled := false

	for _, oldClient := range oldClients {
		if oldClient.Email == clientEmail {
			switch inbound.Protocol {
			case "trojan":
				clientId = oldClient.Password
			case "shadowsocks":
				clientId = oldClient.Email
			default:
				clientId = oldClient.ID
			}
			clientOldEnabled = oldClient.Enable
			break
		}
	}

	if len(clientId) == 0 {
		return false, false, common.NewError("Client Not Found For Email:", clientEmail)
	}

	var settings map[string]any
	err = json.Unmarshal([]byte(inbound.Settings), &settings)
	if err != nil {
		return false, false, err
	}
	clients := settings["clients"].([]any)
	var newClients []any
	for client_index := range clients {
		c := clients[client_index].(map[string]any)
		if c["email"] == clientEmail {
			c["enable"] = !clientOldEnabled
			c["updated_at"] = time.Now().Unix() * 1000
			newClients = append(newClients, any(c))
		}
	}
	settings["clients"] = newClients
	modifiedSettings, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return false, false, err
	}
	inbound.Settings = string(modifiedSettings)

	needRestart, err := s.UpdateInboundClient(inbound, clientId)
	if err != nil {
		return false, needRestart, err
	}
	s.invalidateSettingsCache(inbound.Id)

	return !clientOldEnabled, needRestart, nil
}

func (s *InboundService) ResetClientIpLimitByEmail(clientEmail string, count int) (bool, error) {
	_, inbound, err := s.GetClientInboundByEmail(clientEmail)
	if err != nil {
		return false, err
	}
	if inbound == nil {
		return false, common.NewError("Inbound Not Found For Email:", clientEmail)
	}

	oldClients, err := s.GetClients(inbound)
	if err != nil {
		return false, err
	}

	clientId := ""

	for _, oldClient := range oldClients {
		if oldClient.Email == clientEmail {
			switch inbound.Protocol {
			case "trojan":
				clientId = oldClient.Password
			case "shadowsocks":
				clientId = oldClient.Email
			default:
				clientId = oldClient.ID
			}
			break
		}
	}

	if len(clientId) == 0 {
		return false, common.NewError("Client Not Found For Email:", clientEmail)
	}

	var settings map[string]any
	err = json.Unmarshal([]byte(inbound.Settings), &settings)
	if err != nil {
		return false, err
	}
	clients := settings["clients"].([]any)
	var newClients []any
	for client_index := range clients {
		c := clients[client_index].(map[string]any)
		if c["email"] == clientEmail {
			c["limitIp"] = count
			c["updated_at"] = time.Now().Unix() * 1000
			newClients = append(newClients, any(c))
		}
	}
	settings["clients"] = newClients
	modifiedSettings, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return false, err
	}
	inbound.Settings = string(modifiedSettings)
	needRestart, err := s.UpdateInboundClient(inbound, clientId)
	if err == nil {
		s.invalidateSettingsCache(inbound.Id)
	}
	return needRestart, err
}

func (s *InboundService) ResetClientExpiryTimeByEmail(clientEmail string, expiry_time int64) (bool, error) {
	_, inbound, err := s.GetClientInboundByEmail(clientEmail)
	if err != nil {
		return false, err
	}
	if inbound == nil {
		return false, common.NewError("Inbound Not Found For Email:", clientEmail)
	}

	oldClients, err := s.GetClients(inbound)
	if err != nil {
		return false, err
	}

	clientId := ""

	for _, oldClient := range oldClients {
		if oldClient.Email == clientEmail {
			switch inbound.Protocol {
			case "trojan":
				clientId = oldClient.Password
			case "shadowsocks":
				clientId = oldClient.Email
			default:
				clientId = oldClient.ID
			}
			break
		}
	}

	if len(clientId) == 0 {
		return false, common.NewError("Client Not Found For Email:", clientEmail)
	}

	var settings map[string]any
	err = json.Unmarshal([]byte(inbound.Settings), &settings)
	if err != nil {
		return false, err
	}
	clients := settings["clients"].([]any)
	var newClients []any
	for client_index := range clients {
		c := clients[client_index].(map[string]any)
		if c["email"] == clientEmail {
			c["expiryTime"] = expiry_time
			c["updated_at"] = time.Now().Unix() * 1000
			newClients = append(newClients, any(c))
		}
	}
	settings["clients"] = newClients
	modifiedSettings, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return false, err
	}
	inbound.Settings = string(modifiedSettings)
	needRestart, err := s.UpdateInboundClient(inbound, clientId)
	if err == nil {
		s.invalidateSettingsCache(inbound.Id)
	}
	return needRestart, err
}

func (s *InboundService) ResetClientTrafficLimitByEmail(clientEmail string, totalGB int) (bool, error) {
	if totalGB < 0 {
		return false, common.NewError("totalGB must be >= 0")
	}
	_, inbound, err := s.GetClientInboundByEmail(clientEmail)
	if err != nil {
		return false, err
	}
	if inbound == nil {
		return false, common.NewError("Inbound Not Found For Email:", clientEmail)
	}

	oldClients, err := s.GetClients(inbound)
	if err != nil {
		return false, err
	}

	clientId := ""

	for _, oldClient := range oldClients {
		if oldClient.Email == clientEmail {
			switch inbound.Protocol {
			case "trojan":
				clientId = oldClient.Password
			case "shadowsocks":
				clientId = oldClient.Email
			default:
				clientId = oldClient.ID
			}
			break
		}
	}

	if len(clientId) == 0 {
		return false, common.NewError("Client Not Found For Email:", clientEmail)
	}

	var settings map[string]any
	err = json.Unmarshal([]byte(inbound.Settings), &settings)
	if err != nil {
		return false, err
	}
	clients := settings["clients"].([]any)
	var newClients []any
	for client_index := range clients {
		c := clients[client_index].(map[string]any)
		if c["email"] == clientEmail {
			c["totalGB"] = totalGB * 1024 * 1024 * 1024
			c["updated_at"] = time.Now().Unix() * 1000
			newClients = append(newClients, any(c))
		}
	}
	settings["clients"] = newClients
	modifiedSettings, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return false, err
	}
	inbound.Settings = string(modifiedSettings)
	needRestart, err := s.UpdateInboundClient(inbound, clientId)
	return needRestart, err
}

func (s *InboundService) DelDepletedClients(id int) (err error) {
	db := s.getInboundRepo().GetDB()
	tx := db.Begin()
	defer func() {
		if err == nil {
			tx.Commit()
		} else {
			tx.Rollback()
		}
	}()

	whereText := "reset = 0 and inbound_id "
	if id < 0 {
		whereText += "> ?"
	} else {
		whereText += "= ?"
	}

	depletedClients := []xray.ClientTraffic{}
	err = db.Model(xray.ClientTraffic{}).Where(whereText+" and enable = ?", id, false).Select("inbound_id, GROUP_CONCAT(email) as email").Group("inbound_id").Find(&depletedClients).Error
	if err != nil {
		return err
	}

	for _, depletedClient := range depletedClients {
		emails := strings.Split(depletedClient.Email, ",")
		oldInbound, err := s.GetInbound(depletedClient.InboundId)
		if err != nil {
			return err
		}
		var oldSettings map[string]any
		err = json.Unmarshal([]byte(oldInbound.Settings), &oldSettings)
		if err != nil {
			return err
		}

		oldClients := oldSettings["clients"].([]any)
		var newClients []any
		for _, client := range oldClients {
			deplete := false
			c := client.(map[string]any)
			for _, email := range emails {
				if email == c["email"].(string) {
					deplete = true
					break
				}
			}
			if !deplete {
				newClients = append(newClients, client)
			}
		}
		if len(newClients) > 0 {
			oldSettings["clients"] = newClients

			newSettings, err := json.MarshalIndent(oldSettings, "", "  ")
			if err != nil {
				return err
			}

			oldInbound.Settings = string(newSettings)
			err = tx.Save(oldInbound).Error
			if err != nil {
				return err
			}
		} else {
			// Delete inbound if no client remains
			_, _ = s.DelInbound(depletedClient.InboundId)
		}
	}

	err = tx.Where(whereText+" and enable = ?", id, false).Delete(xray.ClientTraffic{}).Error
	if err != nil {
		return err
	}

	return nil
}

// =============================================================================
// IP 管理
// =============================================================================

func (s *InboundService) GetInboundClientIps(clientEmail string) (string, error) {
	clientIps, err := s.getClientIPRepo().FindByEmail(clientEmail)
	if err != nil {
		return "", err
	}
	return clientIps.Ips, nil
}

func (s *InboundService) ClearClientIps(clientEmail string) error {
	return s.getClientIPRepo().ClearIPs(clientEmail)
}

func (s *InboundService) UpdateClientIPs(tx *gorm.DB, oldEmail string, newEmail string) error {
	return tx.Model(model.InboundClientIps{}).Where("client_email = ?", oldEmail).Update("client_email", newEmail).Error
}

func (s *InboundService) DelClientIPs(tx *gorm.DB, email string) error {
	return tx.Where("client_email = ?", email).Delete(model.InboundClientIps{}).Error
}

// =============================================================================
// 辅助方法
// =============================================================================

func (s *InboundService) IsXrayApiAvailable() bool {
	return s.xrayService != nil && s.xrayService.IsXrayRunning()
}
