package service

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"x-ui/config"
	"x-ui/database/model"
	"x-ui/database/repository"
	"x-ui/logger"
	"x-ui/util/common"
	"x-ui/xray"
)

type InboundService struct {
	xrayApi           xray.XrayAPI
	xrayService       *XrayService
	tgService         TelegramService
	settingsCache     map[int]map[string]any
	cacheMutex        sync.RWMutex
	inboundRepo       repository.InboundRepository
	clientTrafficRepo repository.ClientTrafficRepository
	clientIPRepo      repository.ClientIPRepository
}

// NewInboundService 创建 InboundService 实例，通过构造函数注入 Repository
func NewInboundService(
	inboundRepo repository.InboundRepository,
	clientTrafficRepo repository.ClientTrafficRepository,
	clientIPRepo repository.ClientIPRepository,
) *InboundService {
	return &InboundService{
		inboundRepo:       inboundRepo,
		clientTrafficRepo: clientTrafficRepo,
		clientIPRepo:      clientIPRepo,
		settingsCache:     make(map[int]map[string]any),
	}
}

// =============================================================================
// Setter 方法
// =============================================================================

// SetXrayService 用于从外部注入 XrayService 实例
func (s *InboundService) SetXrayService(xrayService *XrayService) {
	s.xrayService = xrayService
}

// SetXrayAPI 用于从外部注入 XrayAPI 实例
func (s *InboundService) SetXrayAPI(api xray.XrayAPI) {
	s.xrayApi = api
}

// SetTelegramService 用于从外部注入 TelegramService 实例
func (s *InboundService) SetTelegramService(tgService TelegramService) {
	s.tgService = tgService
}

// =============================================================================
// Repository getter (延迟初始化，保持向后兼容)
// =============================================================================

// getInboundRepo 返回 InboundRepository，支持延迟初始化以保持向后兼容
func (s *InboundService) getInboundRepo() repository.InboundRepository {
	if s.inboundRepo == nil {
		s.inboundRepo = repository.NewInboundRepository()
	}
	return s.inboundRepo
}

// getClientTrafficRepo 返回 ClientTrafficRepository，支持延迟初始化以保持向后兼容
func (s *InboundService) getClientTrafficRepo() repository.ClientTrafficRepository {
	if s.clientTrafficRepo == nil {
		s.clientTrafficRepo = repository.NewClientTrafficRepository()
	}
	return s.clientTrafficRepo
}

// getClientIPRepo 返回 ClientIPRepository，支持延迟初始化以保持向后兼容
func (s *InboundService) getClientIPRepo() repository.ClientIPRepository {
	if s.clientIPRepo == nil {
		s.clientIPRepo = repository.NewClientIPRepository()
	}
	return s.clientIPRepo
}

// =============================================================================
// Inbound CRUD
// =============================================================================

func (s *InboundService) GetInbounds(userId int) ([]*model.Inbound, error) {
	return s.getInboundRepo().FindByUserID(userId)
}

func (s *InboundService) GetAllInbounds() ([]*model.Inbound, error) {
	return s.getInboundRepo().FindAll()
}

func (s *InboundService) GetInbound(id int) (*model.Inbound, error) {
	return s.getInboundRepo().FindByID(id)
}

func (s *InboundService) SearchInbounds(query string) ([]*model.Inbound, error) {
	return s.getInboundRepo().Search(query)
}

func (s *InboundService) GetInboundTags() (string, error) {
	inboundTags, err := s.getInboundRepo().GetAllTags()
	if err != nil {
		return "", err
	}
	tags, _ := json.Marshal(inboundTags)
	return string(tags), nil
}

// AddInbound adds a new inbound to db
func (s *InboundService) AddInbound(inbound *model.Inbound) (*model.Inbound, bool, error) {
	// 检查端口是否已存在
	exist, err := s.checkPortExist(inbound.Listen, inbound.Port, 0)
	if err != nil {
		return inbound, false, err
	}
	if exist {
		return inbound, false, common.NewError("port already in use: ", inbound.Port)
	}

	existEmail, err := s.checkEmailExistForInbound(inbound)
	if err != nil {
		return inbound, false, err
	}
	if len(existEmail) > 0 {
		return inbound, false, common.NewError("Duplicate email: ", existEmail)
	}

	clients, err := s.GetClients(inbound)
	if err != nil {
		return inbound, false, err
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

	// Generate unique ID for new inbound
	var maxId int
	err = tx.Model(model.Inbound{}).Select("COALESCE(MAX(id), 0)").Row().Scan(&maxId)
	if err != nil {
		return inbound, false, err
	}
	inbound.Id = maxId + 1

	// Generate tag if empty
	if inbound.Tag == "" {
		inbound.Tag = fmt.Sprintf("inbound-%d", inbound.Id)
	}

	err = tx.Create(inbound).Error
	if err != nil {
		return inbound, false, err
	}

	// Add client stats
	for i := range clients {
		err = s.AddClientStat(tx, inbound.Id, &clients[i])
		if err != nil {
			return inbound, false, err
		}
	}

	needRestart := true

	// Send one-click config notification if TG service is available
	if s.tgService != nil && s.tgService.IsRunning() {
		go func() {
			time.Sleep(2 * time.Second)
			err := s.tgService.SendOneClickConfig(inbound, true, 0)
			if err != nil {
				logger.Debug("Error sending one-click config:", err)
			}
		}()
	}

	return inbound, needRestart, err
}

func (s *InboundService) UpdateInbound(inbound *model.Inbound) (*model.Inbound, bool, error) {
	exist, err := s.checkPortExist(inbound.Listen, inbound.Port, inbound.Id)
	if err != nil {
		return inbound, false, err
	}
	if exist {
		return inbound, false, common.NewError("port already in use: ", inbound.Port)
	}

	oldInbound, err := s.GetInbound(inbound.Id)
	if err != nil {
		return inbound, false, err
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

	// Update client traffics
	err = s.updateClientTraffics(tx, oldInbound, inbound)
	if err != nil {
		return inbound, false, err
	}

	// Clear stream settings cache
	s.invalidateSettingsCache(inbound.Id)

	needRestart := false

	// Check if restart is needed
	oldTag := oldInbound.Tag
	newTag := inbound.Tag
	if oldTag != newTag {
		needRestart = true
	}

	oldPort := oldInbound.Port
	newPort := inbound.Port
	if oldPort != newPort {
		needRestart = true
	}

	oldEnable := oldInbound.Enable
	newEnable := inbound.Enable
	if oldEnable != newEnable {
		needRestart = true
	}

	// Compare settings
	var oldSettings, newSettings map[string]any
	_ = json.Unmarshal([]byte(oldInbound.Settings), &oldSettings)
	_ = json.Unmarshal([]byte(inbound.Settings), &newSettings)

	oldSettingsBytes, _ := json.Marshal(oldSettings)
	newSettingsBytes, _ := json.Marshal(newSettings)
	if string(oldSettingsBytes) != string(newSettingsBytes) {
		needRestart = true
	}

	// Compare stream settings
	if oldInbound.StreamSettings != inbound.StreamSettings {
		needRestart = true
	}

	// Compare sniffing
	if oldInbound.Sniffing != inbound.Sniffing {
		needRestart = true
	}

	// Save inbound
	err = tx.Save(inbound).Error
	if err != nil {
		return inbound, false, err
	}

	return inbound, needRestart, err
}

func (s *InboundService) DelInbound(id int) (bool, error) {
	db := s.getInboundRepo().GetDB()

	var tag string
	err := db.Model(model.Inbound{}).Select("tag").Where("id = ?", id).Row().Scan(&tag)
	if err != nil {
		return false, err
	}

	// Delete client traffics
	err = db.Where("inbound_id = ?", id).Delete(xray.ClientTraffic{}).Error
	if err != nil {
		return false, err
	}

	// Delete client IPs by finding emails first
	var emails []string
	err = db.Raw(`
		SELECT JSON_EXTRACT(client.value, '$.email')
		FROM inbounds,
			JSON_EACH(JSON_EXTRACT(inbounds.settings, '$.clients')) AS client
		WHERE inbounds.id = ?
	`, id).Scan(&emails).Error
	if err == nil && len(emails) > 0 {
		for _, email := range emails {
			db.Where("client_email = ?", email).Delete(model.InboundClientIps{})
		}
	}

	s.invalidateSettingsCache(id)

	needRestart := true

	return needRestart, db.Delete(model.Inbound{}, id).Error
}

// =============================================================================
// 配置相关
// =============================================================================

func (s *InboundService) GetXrayConfig() string {
	return config.GetBinFolderPath() + "/config.json"
}
