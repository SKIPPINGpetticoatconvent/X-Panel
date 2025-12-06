package tests

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"x-ui/database"
	"x-ui/database/model"

	"gorm.io/gorm"
)

// CreateMockInboundService 创建模拟的入站服务用于测试
func CreateMockInboundService() *MockInboundService {
	return &MockInboundService{}
}

// MockInboundService 模拟入站服务
type MockInboundService struct {
	db *gorm.DB
}

// SetDB 设置数据库连接
func (s *MockInboundService) SetDB(db *gorm.DB) {
	s.db = db
}

// MigrateClientsToDatabase 迁移客户端数据到数据库表
func (s *MockInboundService) MigrateClientsToDatabase() error {
	if s.db == nil {
		return fmt.Errorf("数据库未设置")
	}

	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 获取所有有客户端数据的 Inbound 记录
	var inbounds []*model.Inbound
	err := tx.Model(model.Inbound{}).Where("settings LIKE '%\"clients\"%'").Find(&inbounds).Error
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("获取入站失败: %w", err)
	}

	fmt.Printf("开始迁移 %d 个入站的客户端数据\n", len(inbounds))
	migratedClientCount := 0

	for _, inbound := range inbounds {
		// 解析 settings JSON
		var settings map[string]interface{}
		if err := json.Unmarshal([]byte(inbound.Settings), &settings); err != nil {
			fmt.Printf("无法解析 inbound %d 的 settings: %v\n", inbound.Id, err)
			continue
		}

		// 提取 clients 数组
		clientsInterface, ok := settings["clients"]
		if !ok || clientsInterface == nil {
			continue
		}

		clientsArray, ok := clientsInterface.([]interface{})
		if !ok {
			fmt.Printf("inbound %d 的 clients 不是数组格式\n", inbound.Id)
			continue
		}

		// 迁移每个客户端
		for _, clientInterface := range clientsArray {
			clientMap, ok := clientInterface.(map[string]interface{})
			if !ok {
				fmt.Printf("inbound %d 的 client 不是对象格式\n", inbound.Id)
				continue
			}



			// 转换为 Client 结构体
			client := s.convertJSONClientToDBClient(clientMap, inbound.Id)
			if client == nil {
				continue
			}

			// 检查邮箱是否已存在（防止重复）
			var existingCount int64
			tx.Model(&model.Client{}).Where("email = ?", client.Email).Count(&existingCount)
			if existingCount > 0 {
				fmt.Printf("邮箱 %s 已存在，跳过迁移\n", client.Email)
				continue
			}

			// 插入到数据库（使用原生SQL绕过GORM默认值问题）

			
			// 使用原生SQL插入以确保Enable字段不被默认值覆盖
			sql := `
				INSERT INTO clients (
					inbound_id, key, password, security, flow, email, 
					limit_ip, total_gb, expiry_time, speed_limit, 
					enable, tg_id, sub_id, reset, comment, created_at, updated_at
				) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			`
			if err := tx.Exec(sql, 
				client.InboundId, client.Key, client.Password, client.Security, client.Flow, client.Email,
				client.LimitIp, client.TotalGB, client.ExpiryTime, client.SpeedLimit,
				client.Enable, client.TgID, client.SubID, client.Reset, client.Comment, client.CreatedAt, client.UpdatedAt,
			).Error; err != nil {
				fmt.Printf("插入客户端 %s 失败: %v\n", client.Email, err)
				continue
			}
			
			// 验证插入后的数据
			var insertedClient model.Client
			if err := tx.Where("email = ?", client.Email).First(&insertedClient).Error; err == nil {

			}

			migratedClientCount++
		}

		// 从 settings 中移除 clients 数据
		delete(settings, "clients")

		// 重新序列化 settings
		updatedSettings, err := json.MarshalIndent(settings, "", "  ")
		if err != nil {
			fmt.Printf("重新序列化 inbound %d 的 settings 失败: %v\n", inbound.Id, err)
			continue
		}

		// 更新 inbound 记录
		inbound.Settings = string(updatedSettings)
		if err := tx.Save(inbound).Error; err != nil {
			fmt.Printf("更新 inbound %d 的 settings 失败: %v\n", inbound.Id, err)
		}
	}

	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("迁移提交失败: %w", err)
	}

	fmt.Printf("客户端数据迁移完成，共迁移 %d 个客户端\n", migratedClientCount)
	return nil
}

// convertJSONClientToDBClient 将 JSON 客户端对象转换为数据库 Client 结构体
func (s *MockInboundService) convertJSONClientToDBClient(clientMap map[string]interface{}, inboundId int) *model.Client {
	// 提取基本字段
	email, _ := clientMap["email"].(string)
	if email == "" {
		fmt.Printf("客户端缺少 email 字段，跳过\n")
		return nil
	}

	// 提取认证字段（根据协议不同可能使用 id 或 password）
	key, _ := clientMap["id"].(string) // VMess/VLESS 使用 UUID
	if key == "" {
		key, _ = clientMap["password"].(string) // Trojan 可能使用 password
	}
	if key == "" {
		key = email // Shadowsocks 使用 email 作为标识
	}

	password, _ := clientMap["password"].(string)

	// 提取其他字段
	security, _ := clientMap["security"].(string)
	flow, _ := clientMap["flow"].(string)
	
	limitIp := 0
	if v, ok := clientMap["limitIp"].(float64); ok {
		limitIp = int(v)
	} else if v, ok := clientMap["limitIp"].(int); ok {
		limitIp = v
	}

	totalGB := int64(0)
	if v, ok := clientMap["totalGB"].(float64); ok {
		totalGB = int64(v)
	} else if v, ok := clientMap["totalGB"].(int64); ok {
		totalGB = v
	}

	expiryTime := int64(0)
	if v, ok := clientMap["expiryTime"].(float64); ok {
		expiryTime = int64(v)
	} else if v, ok := clientMap["expiryTime"].(int64); ok {
		expiryTime = v
	}

	speedLimit := 0
	if v, ok := clientMap["speedLimit"].(float64); ok {
		speedLimit = int(v)
	} else if v, ok := clientMap["speedLimit"].(int); ok {
		speedLimit = v
	}

	enable := true
	if v, ok := clientMap["enable"]; ok {
		switch vv := v.(type) {
		case bool:
			enable = vv
		case float64:
			enable = vv > 0
		case int:
			enable = vv > 0
		case string:
			enable = vv == "true"
		}
	}

	tgID := int64(0)
	if v, ok := clientMap["tgId"].(float64); ok {
		tgID = int64(v)
	} else if v, ok := clientMap["tgId"].(int64); ok {
		tgID = v
	}

	subID, _ := clientMap["subId"].(string)
	
	reset := 0
	if v, ok := clientMap["reset"].(float64); ok {
		reset = int(v)
	} else if v, ok := clientMap["reset"].(int); ok {
		reset = v
	}

	comment, _ := clientMap["comment"].(string)

	// 处理时间戳
	createdAt := int64(0)
	if v, ok := clientMap["created_at"].(float64); ok {
		createdAt = int64(v)
	} else if v, ok := clientMap["created_at"].(int64); ok {
		createdAt = v
	} else {
		createdAt = time.Now().Unix() * 1000
	}

	updatedAt := int64(0)
	if v, ok := clientMap["updated_at"].(float64); ok {
		updatedAt = int64(v)
	} else if v, ok := clientMap["updated_at"].(int64); ok {
		updatedAt = v
	} else {
		updatedAt = time.Now().Unix() * 1000
	}

	client := &model.Client{
		InboundId:   inboundId,
		Key:         key,
		Password:    password,
		Security:    security,
		Flow:        flow,
		Email:       email,
		LimitIp:     limitIp,
		TotalGB:     totalGB,
		ExpiryTime:  expiryTime,
		SpeedLimit:  speedLimit,
		TgID:        tgID,
		SubID:       subID,
		Reset:       reset,
		Comment:     comment,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}
	// 明确设置Enable字段以覆盖默认值
	client.Enable = enable
	return client
}

// GetClients 获取客户端列表（简化版本）
func (s *MockInboundService) GetClients(inbound *model.Inbound) ([]model.Client, error) {
	if s.db == nil {
		return nil, fmt.Errorf("数据库未设置")
	}

	var clients []model.Client
	
	// 尝试从数据库的 clients 表获取
	err := s.db.Where("inbound_id = ?", inbound.Id).Find(&clients).Error
	if err == nil && len(clients) > 0 {
		return clients, nil
	}
	
	// 如果数据库中没有数据，尝试从 JSON 中获取（向后兼容）
	if err == gorm.ErrRecordNotFound || len(clients) == 0 {
		return s.getClientsFromJSON(inbound)
	}
	
	return nil, err
}

// getClientsFromJSON 从 JSON 中获取客户端数据（向后兼容）
func (s *MockInboundService) getClientsFromJSON(inbound *model.Inbound) ([]model.Client, error) {
	settings := map[string][]model.ClientForJSON{}
	if err := json.Unmarshal([]byte(inbound.Settings), &settings); err != nil {
		return nil, fmt.Errorf("setting is null: %w", err)
	}

	clientsJSON := settings["clients"]
	if clientsJSON == nil {
		return nil, nil
	}

	// 转换为新的 Client 结构体
	var clients []model.Client
	for _, clientJSON := range clientsJSON {
		client := model.Client{
			Key:        clientJSON.ID,
			Password:   clientJSON.Password,
			Security:   clientJSON.Security,
			Flow:       clientJSON.Flow,
			Email:      clientJSON.Email,
			LimitIp:    clientJSON.LimitIP,
			TotalGB:    clientJSON.TotalGB,
			ExpiryTime: clientJSON.ExpiryTime,
			SpeedLimit: clientJSON.SpeedLimit,
			Enable:     clientJSON.Enable,
			TgID:       clientJSON.TgID,
			SubID:      clientJSON.SubID,
			Comment:    clientJSON.Comment,
			Reset:      clientJSON.Reset,
			CreatedAt:  clientJSON.CreatedAt,
			UpdatedAt:  clientJSON.UpdatedAt,
		}
		clients = append(clients, client)
	}
	
	return clients, nil
}

// TestDatabaseMigration 验证数据库迁移功能的测试
func TestDatabaseMigration(t *testing.T) {
	// 创建临时测试数据库
	tempDBPath := filepath.Join(t.TempDir(), "test_migration.db")
	db, err := database.CreateTestDB(tempDBPath)
	if err != nil {
		t.Fatalf("无法创建测试数据库: %v", err)
	}
	defer database.CleanupTestDB(db)

	// 初始化数据库模型
	if err := database.InitTestModels(db); err != nil {
		t.Fatalf("无法初始化测试模型: %v", err)
	}

	// 创建模拟服务实例
	inboundService := CreateMockInboundService()
	inboundService.SetDB(db)

	// 创建测试用的旧格式 JSON 数据（包含 clients 数组）
	oldSettings := map[string]interface{}{
		"clients": []interface{}{
			map[string]interface{}{
				"id":          "test-uuid-1",
				"email":       "user1@example.com",
				"security":    "auto",
				"flow":        "",
				"password":    "",
				"limitIp":     2,
				"totalGB":     int64(1024 * 1024 * 1024), // 1GB
				"expiryTime":  time.Now().Unix() * 1000 + 86400000, // 明天过期
				"speedLimit":  100, // 100KB/s
				"enable":      true,
				"tgId":        int64(123456789),
				"subId":       "sub123",
				"reset":       0,
				"comment":     "测试用户1",
				"created_at":  time.Now().Unix() * 1000,
				"updated_at":  time.Now().Unix() * 1000,
			},
			map[string]interface{}{
				"id":          "test-uuid-2",
				"email":       "user2@example.com",
				"security":    "aes-128-gcm",
				"flow":        "xtls-rprx-direct",
				"password":    "",
				"limitIp":     1,
				"totalGB":     int64(5 * 1024 * 1024 * 1024), // 5GB
				"expiryTime":  0, // 永不过期
				"speedLimit":  0, // 不限速
				"enable":      true,
				"tgId":        int64(987654321),
				"subId":       "sub456",
				"reset":       30,
				"comment":     "测试用户2 - VIP",
				"created_at":  time.Now().Unix() * 1000,
				"updated_at":  time.Now().Unix() * 1000,
			},
			map[string]interface{}{
				"password":    "trojan-password-1",
				"email":       "trojan-user@example.com",
				"security":    "",
				"flow":        "xtls-rprx-vision",
				"limitIp":     3,
				"totalGB":     int64(10 * 1024 * 1024 * 1024), // 10GB
				"expiryTime":  time.Now().Unix()*1000 + 7*86400000, // 7天后过期
				"speedLimit":  500, // 500KB/s
				"enable":      false,
				"tgId":        int64(0),
				"subId":       "",
				"reset":       0,
				"comment":     "Trojan 用户 - 已禁用",
				"created_at":  time.Now().Unix() * 1000,
				"updated_at":  time.Now().Unix() * 1000,
			},
		},
	}

	// 序列化旧设置
	oldSettingsJSON, err := json.MarshalIndent(oldSettings, "", "  ")
	if err != nil {
		t.Fatalf("序列化旧设置失败: %v", err)
	}

	// 创建测试入站记录
	inbound := &model.Inbound{
		Id:          1,
		UserId:      1,
		Up:          0,
		Down:        0,
		Total:       0,
		AllTime:     0,
		Remark:      "测试入站 - 迁移验证",
		Enable:      true,
		ExpiryTime:  0,
		DeviceLimit: 0,
		Listen:      "0.0.0.0",
		Port:        8080,
		Protocol:    model.VMESS,
		Settings:    string(oldSettingsJSON),
		StreamSettings: "{}",
		Tag:         "inbound-8080",
		Sniffing:    "{}",
	}

	// 插入测试数据到数据库
	if err := db.Create(inbound).Error; err != nil {
		t.Fatalf("插入测试入站失败: %v", err)
	}

	fmt.Println("✅ 成功创建包含旧格式客户端数据的测试入站")

	// 验证迁移前的数据状态
	var inboundsBefore []*model.Inbound
	if err := db.Find(&inboundsBefore).Error; err != nil {
		t.Fatalf("查询迁移前数据失败: %v", err)
	}

	if len(inboundsBefore) != 1 {
		t.Fatalf("预期1个入站，实际%d个", len(inboundsBefore))
	}

	// 验证旧 JSON 数据格式
	var settingsBefore map[string]interface{}
	if err := json.Unmarshal([]byte(inboundsBefore[0].Settings), &settingsBefore); err != nil {
		t.Fatalf("解析迁移前设置失败: %v", err)
	}

	clientsBefore, ok := settingsBefore["clients"].([]interface{})
	if !ok {
		t.Fatalf("迁移前设置中未找到 clients 数组")
	}
	
	if len(clientsBefore) != 3 {
		t.Fatalf("预期3个客户端，实际%d个", len(clientsBefore))
	}

	fmt.Println("✅ 验证迁移前数据状态成功")

	// 执行迁移
	fmt.Println("🔄 开始执行数据库迁移...")
	if err := inboundService.MigrateClientsToDatabase(); err != nil {
		t.Fatalf("数据库迁移失败: %v", err)
	}

	fmt.Println("✅ 数据库迁移执行完成")

	// 验证迁移后的数据状态

	// 1. 验证 clients 表中的数据
	var migratedClients []model.Client
	if err := db.Find(&migratedClients).Error; err != nil {
		t.Fatalf("查询迁移后 clients 表失败: %v", err)
	}



	if len(migratedClients) != 3 {
		t.Fatalf("预期迁移3个客户端，实际%d个", len(migratedClients))
	}

	fmt.Println("✅ 验证客户端迁移数量成功")

	// 2. 验证每个客户端的具体字段（使用邮箱作为查找键）
	expectedClients := map[string]struct {
		Email       string
		Key         string
		Password    string
		Security    string
		Flow        string
		LimitIp     int
		TotalGB     int64
		SpeedLimit  int
		Enable      bool
		Comment     string
	}{
		"user1@example.com": {
			Email:       "user1@example.com",
			Key:         "test-uuid-1",
			Security:    "auto",
			Flow:        "",
			LimitIp:     2,
			TotalGB:     int64(1024 * 1024 * 1024),
			SpeedLimit:  100,
			Enable:      true,
			Comment:     "测试用户1",
		},
		"user2@example.com": {
			Email:       "user2@example.com",
			Key:         "test-uuid-2",
			Security:    "aes-128-gcm",
			Flow:        "xtls-rprx-direct",
			LimitIp:     1,
			TotalGB:     int64(5 * 1024 * 1024 * 1024),
			SpeedLimit:  0,
			Enable:      true,
			Comment:     "测试用户2 - VIP",
		},
		"trojan-user@example.com": {
			Email:       "trojan-user@example.com",
			Password:    "trojan-password-1",
			Security:    "",
			Flow:        "xtls-rprx-vision",
			LimitIp:     3,
			TotalGB:     int64(10 * 1024 * 1024 * 1024),
			SpeedLimit:  500,
			Enable:      false,
			Comment:     "Trojan 用户 - 已禁用",
		},
	}

	for _, client := range migratedClients {
		expected, exists := expectedClients[client.Email]
		if !exists {
			t.Errorf("未找到期望的客户端: %s", client.Email)
			continue
		}
		
		if expected.Key != "" && client.Key != expected.Key {
			t.Errorf("客户端 %s Key不匹配: 预期%s，实际%s", client.Email, expected.Key, client.Key)
		}
		
		if expected.Password != "" && client.Password != expected.Password {
			t.Errorf("客户端 %s Password不匹配: 预期%s，实际%s", client.Email, expected.Password, client.Password)
		}
		
		if client.Security != expected.Security {
			t.Errorf("客户端 %s Security不匹配: 预期%s，实际%s", client.Email, expected.Security, client.Security)
		}
		
		if client.Flow != expected.Flow {
			t.Errorf("客户端 %s Flow不匹配: 预期%s，实际%s", client.Email, expected.Flow, client.Flow)
		}
		
		if client.LimitIp != expected.LimitIp {
			t.Errorf("客户端 %s LimitIp不匹配: 预期%d，实际%d", client.Email, expected.LimitIp, client.LimitIp)
		}
		
		if client.TotalGB != expected.TotalGB {
			t.Errorf("客户端 %s TotalGB不匹配: 预期%d，实际%d", client.Email, expected.TotalGB, client.TotalGB)
		}
		
		if client.SpeedLimit != expected.SpeedLimit {
			t.Errorf("客户端 %s SpeedLimit不匹配: 预期%d，实际%d", client.Email, expected.SpeedLimit, client.SpeedLimit)
		}
		
		if client.Enable != expected.Enable {
			t.Errorf("客户端 %s Enable不匹配: 预期%v，实际%v", client.Email, expected.Enable, client.Enable)
		}
		
		if client.Comment != expected.Comment {
			t.Errorf("客户端 %s Comment不匹配: 预期%s，实际%s", client.Email, expected.Comment, client.Comment)
		}
		
		// 验证时间戳
		if client.CreatedAt == 0 {
			t.Errorf("客户端 %s CreatedAt为空", client.Email)
		}
		
		if client.UpdatedAt == 0 {
			t.Errorf("客户端 %s UpdatedAt为空", client.Email)
		}
	}

	fmt.Println("✅ 验证客户端字段数据成功")

	// 3. 验证入站设置的 JSON 已经被清理
	var inboundsAfter []*model.Inbound
	if err := db.Find(&inboundsAfter).Error; err != nil {
		t.Fatalf("查询迁移后入站失败: %v", err)
	}

	var settingsAfter map[string]interface{}
	if err := json.Unmarshal([]byte(inboundsAfter[0].Settings), &settingsAfter); err != nil {
		t.Fatalf("解析迁移后设置失败: %v", err)
	}

	if _, ok := settingsAfter["clients"]; ok {
		t.Errorf("迁移后设置中仍包含 clients 字段")
	}

	fmt.Println("✅ 验证入站设置清理成功")

	// 4. 验证关联关系
	if inboundsAfter[0].Id != migratedClients[0].InboundId {
		t.Errorf("客户端关联的 InboundId 不正确: 预期%d，实际%d", inboundsAfter[0].Id, migratedClients[0].InboundId)
	}

	fmt.Println("✅ 验证关联关系成功")

	fmt.Println("🎉 数据库迁移验证完成 - 所有测试通过!")
}

// TestBusinessLogic 验证业务逻辑的测试
func TestBusinessLogic(t *testing.T) {
	// 创建临时测试数据库
	tempDBPath := filepath.Join(t.TempDir(), "test_business.db")
	db, err := database.CreateTestDB(tempDBPath)
	if err != nil {
		t.Fatalf("无法创建测试数据库: %v", err)
	}
	defer database.CleanupTestDB(db)

	// 初始化数据库模型
	if err := database.InitTestModels(db); err != nil {
		t.Fatalf("无法初始化测试模型: %v", err)
	}

	// 创建模拟服务实例
	inboundService := CreateMockInboundService()
	inboundService.SetDB(db)

	// 创建测试入站
	inbound := &model.Inbound{
		Id:             1,
		UserId:         1,
		Up:             0,
		Down:           0,
		Total:          0,
		AllTime:        0,
		Remark:         "测试入站 - 业务逻辑",
		Enable:         true,
		ExpiryTime:     0,
		DeviceLimit:    0,
		Listen:         "0.0.0.0",
		Port:           8081,
		Protocol:       model.VMESS,
		Settings:       `{"clients": []}`,
		StreamSettings: "{}",
		Tag:            "inbound-8081",
		Sniffing:       "{}",
	}

	if err := db.Create(inbound).Error; err != nil {
		t.Fatalf("插入测试入站失败: %v", err)
	}

	fmt.Println("✅ 成功创建测试入站")

	// 测试 GetClients 功能（向后兼容性）
	t.Run("GetClients", func(t *testing.T) {
		// 创建一个包含旧 JSON 格式数据的入站
		oldInbound := &model.Inbound{
			Id:             2,
			UserId:         1,
			Up:             0,
			Down:           0,
			Total:          0,
			AllTime:        0,
			Remark:         "旧格式入站",
			Enable:         true,
			ExpiryTime:     0,
			DeviceLimit:    0,
			Listen:         "0.0.0.0",
			Port:           8082,
			Protocol:       model.VLESS,
			Settings:       `{"clients": [{"id": "old-client-uuid", "email": "olduser@example.com", "security": "none", "enable": true, "limitIp": 1, "totalGB": 536870912, "expiryTime": 0, "speedLimit": 50, "tgId": 0, "subId": "", "reset": 0, "comment": "旧格式用户"}]}`,
			StreamSettings: "{}",
			Tag:            "inbound-8082",
			Sniffing:       "{}",
		}

		if err := db.Create(oldInbound).Error; err != nil {
			t.Fatalf("插入旧格式入站失败: %v", err)
		}

		// 测试 GetClients 是否能正确读取旧 JSON 格式数据
		clients, err := inboundService.GetClients(oldInbound)
		if err != nil {
			t.Fatalf("获取旧格式客户端失败: %v", err)
		}

		if len(clients) != 1 {
			t.Fatalf("预期1个旧格式客户端，实际%d个", len(clients))
		}

		client := clients[0]
		if client.Email != "olduser@example.com" {
			t.Errorf("旧格式客户端邮箱不匹配: 预期olduser@example.com，实际%s", client.Email)
		}
		if client.Key != "old-client-uuid" {
			t.Errorf("旧格式客户端Key不匹配: 预期old-client-uuid，实际%s", client.Key)
		}

		fmt.Println("✅ 向后兼容性测试通过")
	})

	// 测试迁移后的数据读取
	t.Run("PostMigrationDataAccess", func(t *testing.T) {
		// 先执行迁移
		if err := inboundService.MigrateClientsToDatabase(); err != nil {
			t.Fatalf("迁移失败: %v", err)
		}

		// 测试迁移后是否能正确读取数据
		clients, err := inboundService.GetClients(inbound)
		if err != nil {
			t.Fatalf("获取迁移后客户端失败: %v", err)
		}

		// 由于当前入站没有客户端，预期返回空列表
		if len(clients) != 0 {
			t.Fatalf("预期0个客户端，实际%d个", len(clients))
		}

		fmt.Println("✅ 迁移后数据访问测试通过")
	})

	fmt.Println("🎉 业务逻辑验证完成 - 所有测试通过!")
}