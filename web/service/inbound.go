package service

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"x-ui/config"
	"x-ui/database"
	"x-ui/database/model"
	"x-ui/database/repository"
	"x-ui/logger"
	"x-ui/util/common"
	"x-ui/xray"

	"gorm.io/gorm"
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
		s.inboundRepo = repository.NewInboundRepository(database.GetDB())
	}
	return s.inboundRepo
}

// getClientTrafficRepo 返回 ClientTrafficRepository，支持延迟初始化以保持向后兼容
func (s *InboundService) getClientTrafficRepo() repository.ClientTrafficRepository {
	if s.clientTrafficRepo == nil {
		s.clientTrafficRepo = repository.NewClientTrafficRepository(database.GetDB())
	}
	return s.clientTrafficRepo
}

// getClientIPRepo 返回 ClientIPRepository，支持延迟初始化以保持向后兼容
func (s *InboundService) getClientIPRepo() repository.ClientIPRepository {
	if s.clientIPRepo == nil {
		s.clientIPRepo = repository.NewClientIPRepository(database.GetDB())
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

	type addResult struct {
		inbound     *model.Inbound
		needRestart bool
	}

	result, err := database.WithTxResult(func(tx *gorm.DB) (addResult, error) {
		// Generate unique ID for new inbound
		var maxId int
		if err := tx.Model(model.Inbound{}).Select("COALESCE(MAX(id), 0)").Row().Scan(&maxId); err != nil {
			return addResult{}, err
		}
		inbound.Id = maxId + 1

		// Generate tag if empty
		if inbound.Tag == "" {
			inbound.Tag = fmt.Sprintf("inbound-%d", inbound.Id)
		}

		if err := tx.Create(inbound).Error; err != nil {
			return addResult{}, err
		}

		// Add client stats
		for i := range clients {
			if err := s.AddClientStat(tx, inbound.Id, &clients[i]); err != nil {
				return addResult{}, err
			}
		}

		return addResult{inbound: inbound, needRestart: true}, nil
	})
	if err != nil {
		return inbound, false, err
	}

	// Send one-click config notification if TG service is available
	if s.tgService != nil && s.tgService.IsRunning() {
		go func() {
			time.Sleep(2 * time.Second)
			err := s.tgService.SendOneClickConfig(result.inbound, true, 0)
			if err != nil {
				logger.Debug("Error sending one-click config:", err)
			}
		}()
	}

	return result.inbound, result.needRestart, nil
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

	// Clear stream settings cache
	s.invalidateSettingsCache(inbound.Id)

	// Determine if restart is needed
	needRestart := oldInbound.Tag != inbound.Tag ||
		oldInbound.Port != inbound.Port ||
		oldInbound.Enable != inbound.Enable

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

	err = database.WithTx(func(tx *gorm.DB) error {
		// Update client traffics
		if err := s.updateClientTraffics(tx, oldInbound, inbound); err != nil {
			return err
		}

		// Save inbound
		return tx.Save(inbound).Error
	})
	if err != nil {
		return inbound, false, err
	}

	return inbound, needRestart, nil
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
