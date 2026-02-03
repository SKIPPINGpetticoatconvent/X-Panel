package service

import (
	_ "embed"
	"encoding/json"

	"x-ui/util/common"
	"x-ui/xray"
)

type XraySettingService struct {
	SettingService
}

// NewXraySettingService 创建 XraySettingService 实例
func NewXraySettingService(settingService *SettingService) *XraySettingService {
	return &XraySettingService{SettingService: *settingService}
}

func (s *XraySettingService) SaveXraySetting(newXraySettings string) error {
	if err := s.CheckXrayConfig(newXraySettings); err != nil {
		return err
	}
	return s.saveSetting("xrayTemplateConfig", newXraySettings)
}

func (s *XraySettingService) CheckXrayConfig(XrayTemplateConfig string) error {
	xrayConfig := &xray.Config{}
	err := json.Unmarshal([]byte(XrayTemplateConfig), xrayConfig)
	if err != nil {
		return common.NewError("xray template config invalid:", err)
	}
	return nil
}
