package database

import (
	"encoding/json"
	"strings"
	"x-ui/database/model"
	"x-ui/logger"
)

// migrateRealityTarget 修复旧版本 Reality 入站的 target 字段，添加端口号
func migrateRealityTarget() error {
	var inbounds []model.Inbound

	// 查找所有使用 Reality 协议的入站
	if err := db.Where("stream_settings LIKE '%reality%'").Find(&inbounds).Error; err != nil {
		return err
	}

	fixedCount := 0
	for _, inbound := range inbounds {
		var streamSettings map[string]interface{}
		if err := json.Unmarshal([]byte(inbound.StreamSettings), &streamSettings); err != nil {
			logger.Warningf("无法解析入站 %d 的 stream_settings: %v", inbound.Id, err)
			continue
		}

		// 检查是否有 realitySettings
		realitySettings, exists := streamSettings["realitySettings"]
		if !exists {
			continue
		}

		realityMap, ok := realitySettings.(map[string]interface{})
		if !ok {
			continue
		}

		// 检查 target 字段
		target, exists := realityMap["target"]
		if !exists {
			continue
		}

		targetStr, ok := target.(string)
		if !ok {
			continue
		}

		// 如果 target 不包含端口，添加 :443
		if !strings.Contains(targetStr, ":") {
			realityMap["target"] = targetStr + ":443"

			// 重新序列化
			updatedSettings, err := json.Marshal(streamSettings)
			if err != nil {
				logger.Warningf("无法序列化入站 %d 的更新后配置: %v", inbound.Id, err)
				continue
			}

			// 更新数据库
			if err := db.Model(&model.Inbound{}).Where("id = ?", inbound.Id).
				Update("stream_settings", string(updatedSettings)).Error; err != nil {
				logger.Warningf("无法更新入站 %d: %v", inbound.Id, err)
				continue
			}

			logger.Infof("修复入站 %d (%s) 的 Reality target: %s -> %s",
				inbound.Id, inbound.Tag, targetStr, targetStr+":443")
			fixedCount++
		}
	}

	if fixedCount > 0 {
		logger.Infof("Reality target 迁移完成，修复了 %d 个入站", fixedCount)
	} else {
		logger.Info("Reality target 迁移检查完成，无需修复")
	}

	return nil
}
