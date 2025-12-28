package service

import (
	"database/sql"
	"encoding/json"
	"os"
	"testing"
	"time"

	"x-ui/database/model"
	"x-ui/xray"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// DatabaseTestSuite 数据库测试套件
type DatabaseTestSuite struct {
	suite.Suite
	db     *sql.DB
	gormDB *gorm.DB
}

// SetupSuite 设置测试套件
func (suite *DatabaseTestSuite) SetupSuite() {
	// 创建临时数据库文件
	dbFile := "./test.db"
	
	// 打开SQLite数据库
	var err error
	suite.db, err = sql.Open("sqlite", dbFile)
	if err != nil {
		suite.T().Fatalf("Failed to open database: %v", err)
	}
	
	// 创建GORM数据库连接
	suite.gormDB, err = gorm.Open(sqlite.Open(dbFile), &gorm.Config{})
	if err != nil {
		suite.T().Fatalf("Failed to open GORM database: %v", err)
	}
	
	// 自动迁移数据库模式
	err = suite.gormDB.AutoMigrate(
		&model.User{},
		&model.Inbound{},
		&model.OutboundTraffics{},
		&model.InboundClientIps{},
		&model.Setting{},
	)
	if err != nil {
		suite.T().Fatalf("Failed to migrate database: %v", err)
	}
}

// TearDownSuite 清理测试套件
func (suite *DatabaseTestSuite) TearDownSuite() {
	// 关闭数据库连接
	if suite.db != nil {
		suite.db.Close()
	}
	
	// 删除临时数据库文件
	os.Remove("./test.db")
}

// SetupTest 设置每个测试
func (suite *DatabaseTestSuite) SetupTest() {
	// 清空所有表
	suite.gormDB.Exec("DELETE FROM users")
	suite.gormDB.Exec("DELETE FROM inbounds")
	suite.gormDB.Exec("DELETE FROM outbound_traffics")
	suite.gormDB.Exec("DELETE FROM inbound_client_ips")
	suite.gormDB.Exec("DELETE FROM settings")
}

// TestUserService_CreateUser 测试创建用户
func (suite *DatabaseTestSuite) TestUserService_CreateUser() {
	userService := NewUserService()
	userService.db = suite.gormDB
	
	// 创建测试用户
	user := &model.User{
		Username: "testuser",
		Password: "hashedpassword",
	}
	
	err := userService.CreateUser(user)
	suite.NoError(err)
	
	// 验证用户已创建
	var savedUser model.User
	err = suite.gormDB.Where("username = ?", "testuser").First(&savedUser).Error()
	suite.NoError(err)
	suite.Equal("testuser", savedUser.Username)
}

// TestUserService_GetUserByUsername 测试根据用户名获取用户
func (suite *DatabaseTestSuite) TestUserService_GetUserByUsername() {
	userService := NewUserService()
	userService.db = suite.gormDB
	
	// 创建测试用户
	hashedPassword, _ := HashPasswordAsBcrypt("testpassword")
	user := &model.User{
		Username: "testuser",
		Password: hashedPassword,
	}
	err := userService.CreateUser(user)
	suite.NoError(err)
	
	// 测试获取用户
	retrievedUser, err := userService.GetUserByUsername("testuser")
	suite.NoError(err)
	suite.NotNil(retrievedUser)
	suite.Equal("testuser", retrievedUser.Username)
	
	// 测试获取不存在的用户
	_, err = userService.GetUserByUsername("nonexistent")
	suite.Error(err)
}

// TestUserService_UpdateUser 测试更新用户
func (suite *DatabaseTestSuite) TestUserService_UpdateUser() {
	userService := NewUserService()
	userService.db = suite.gormDB
	
	// 创建测试用户
	hashedPassword, _ := HashPasswordAsBcrypt("oldpassword")
	user := &model.User{
		Username: "testuser",
		Password: hashedPassword,
	}
	err := userService.CreateUser(user)
	suite.NoError(err)
	
	// 更新用户信息
	newHashedPassword, _ := HashPasswordAsBcrypt("newpassword")
	err = userService.UpdateUser(1, "newuser", "newpassword")
	suite.NoError(err)
	
	// 验证更新后的用户信息
	var updatedUser model.User
	err = suite.gormDB.Where("id = ?", 1).First(&updatedUser).Error()
	suite.NoError(err)
	suite.Equal("newuser", updatedUser.Username)
	
	// 验证密码已更新
	isValid := CheckPasswordHash("newpassword", updatedUser.Password)
	suite.True(isValid)
}

// TestInboundService_CreateInbound 测试创建入站
func (suite *DatabaseTestSuite) TestInboundService_CreateInbound() {
	inboundService := &InboundService{}
	inboundService.db = suite.gormDB
	
	// 创建测试入站
	inbound := &model.Inbound{
		UserId:   1,
		Port:     8080,
		Protocol: model.VLESS,
		Remark:   "Test Inbound",
		Enable:   true,
		Settings: `{"clients":[{"id":"test-id","email":"test@example.com"}]}`,
		Tag:      "inbound-8080",
	}
	
	err := inboundService.CreateInbound(inbound)
	suite.NoError(err)
	
	// 验证入站已创建
	var savedInbound model.Inbound
	err = suite.gormDB.Where("port = ?", 8080).First(&savedInbound).Error()
	suite.NoError(err)
	suite.Equal(8080, savedInbound.Port)
	suite.Equal(model.VLESS, savedInbound.Protocol)
}

// TestInboundService_GetInbounds 测试获取入站列表
func (suite *DatabaseTestSuite) TestInboundService_GetInbounds() {
	inboundService := &InboundService{}
	inboundService.db = suite.gormDB
	
	// 创建测试用户
	hashedPassword, _ := HashPasswordAsBcrypt("testpassword")
	user := &model.User{
		Username: "testuser",
		Password: hashedPassword,
	}
	err := suite.gormDB.Create(user).Error()
	suite.NoError(err)
	
	// 创建测试入站
	inbounds := []*model.Inbound{
		{
			UserId:   user.Id,
			Port:     8080,
			Protocol: model.VLESS,
			Remark:   "Inbound 1",
			Enable:   true,
			Tag:      "inbound-8080",
		},
		{
			UserId:   user.Id,
			Port:     9090,
			Protocol: model.VMESS,
			Remark:   "Inbound 2",
			Enable:   true,
			Tag:      "inbound-9090",
		},
	}
	
	for _, inbound := range inbounds {
		err := inboundService.CreateInbound(inbound)
		suite.NoError(err)
	}
	
	// 测试获取用户的入站列表
	retrievedInbounds, err := inboundService.GetInbounds(user.Id)
	suite.NoError(err)
	suite.Len(retrievedInbounds, 2)
	
	// 验证端口号
	ports := make(map[int]bool)
	for _, inbound := range retrievedInbounds {
		ports[inbound.Port] = true
	}
	suite.True(ports[8080])
	suite.True(ports[9090])
}

// TestInboundService_UpdateInbound 测试更新入站
func (suite *DatabaseTestSuite) TestInboundService_UpdateInbound() {
	inboundService := &InboundService{}
	inboundService.db = suite.gormDB
	
	// 创建测试入站
	inbound := &model.Inbound{
		UserId:   1,
		Port:     8080,
		Protocol: model.VLESS,
		Remark:   "Original Inbound",
		Enable:   true,
		Tag:      "inbound-8080",
	}
	err := inboundService.CreateInbound(inbound)
	suite.NoError(err)
	
	// 更新入站信息
	updatedInbound := &model.Inbound{
		Id:       1,
		Port:     8080,
		Protocol: model.VLESS,
		Remark:   "Updated Inbound",
		Enable:   false,
		Tag:      "inbound-8080",
	}
	
	_, _, err = inboundService.UpdateInbound(updatedInbound)
	suite.NoError(err)
	
	// 验证更新后的信息
	var savedInbound model.Inbound
	err = suite.gormDB.Where("id = ?", 1).First(&savedInbound).Error()
	suite.NoError(err)
	suite.Equal("Updated Inbound", savedInbound.Remark)
	suite.False(savedInbound.Enable)
}

// TestInboundService_DeleteInbound 测试删除入站
func (suite *DatabaseTestSuite) TestInboundService_DeleteInbound() {
	inboundService := &InboundService{}
	inboundService.db = suite.gormDB
	
	// 创建测试入站
	inbound := &model.Inbound{
		UserId:   1,
		Port:     8080,
		Protocol: model.VLESS,
		Remark:   "Test Inbound",
		Enable:   true,
		Tag:      "inbound-8080",
	}
	err := inboundService.CreateInbound(inbound)
	suite.NoError(err)
	
	// 删除入站
	_, err = inboundService.DelInbound(1)
	suite.NoError(err)
	
	// 验证入站已删除
	var deletedInbound model.Inbound
	err = suite.gormDB.Where("id = ?", 1).First(&deletedInbound).Error()
	suite.Error(err)
}

// TestSettingService_CreateSetting 测试创建设置
func (suite *DatabaseTestSuite) TestSettingService_CreateSetting() {
	settingService := &SettingService{}
	settingService.db = suite.gormDB
	
	// 创建设置
	setting := &model.Setting{
		Key:   "test_key",
		Value: "test_value",
	}
	
	err := settingService.CreateSetting(setting)
	suite.NoError(err)
	
	// 验证设置已创建
	var savedSetting model.Setting
	err = suite.gormDB.Where("key = ?", "test_key").First(&savedSetting).Error()
	suite.NoError(err)
	suite.Equal("test_value", savedSetting.Value)
}

// TestSettingService_GetSetting 测试获取设置
func (suite *DatabaseTestSuite) TestSettingService_GetSetting() {
	settingService := &SettingService{}
	settingService.db = suite.gormDB
	
	// 创建设置
	setting := &model.Setting{
		Key:   "database_key",
		Value: "database_value",
	}
	err := suite.gormDB.Create(setting).Error()
	suite.NoError(err)
	
	// 测试获取设置
	value, err := settingService.GetSetting("database_key")
	suite.NoError(err)
	suite.Equal("database_value", value)
	
	// 测试获取不存在的设置
	_, err = settingService.GetSetting("nonexistent_key")
	suite.Error(err)
}

// TestInboundService_ClientOperations 测试客户端操作
func (suite *DatabaseTestSuite) TestInboundService_ClientOperations() {
	inboundService := &InboundService{}
	inboundService.db = suite.gormDB
	
	// 创建测试入站
	inbound := &model.Inbound{
		UserId:   1,
		Port:     8080,
		Protocol: model.VLESS,
		Remark:   "Test Inbound",
		Enable:   true,
		Settings: `{"clients":[{"id":"client1","email":"client1@example.com","enable":true}]}`,
		Tag:      "inbound-8080",
	}
	err := inboundService.CreateInbound(inbound)
	suite.NoError(err)
	
	// 测试添加客户端
	clientData := &model.Inbound{
		Id:       1,
		Settings: `{"clients":[{"id":"client2","email":"client2@example.com","enable":true}]}`,
	}
	
	_, err = inboundService.AddInboundClient(clientData)
	suite.NoError(err)
	
	// 验证客户端已添加
	clients, err := inboundService.GetClients(inbound)
	suite.NoError(err)
	suite.Len(clients, 1) // 实际逻辑中可能只返回启用的客户端
	
	// 测试更新客户端
	updateData := &model.Inbound{
		Id:       1,
		Settings: `{"clients":[{"id":"client2","email":"client2@example.com","enable":false}]}`,
	}
	
	_, err = inboundService.UpdateInboundClient(updateData, "client2")
	suite.NoError(err)
	
	// 测试删除客户端
	_, err = inboundService.DelInboundClient(1, "client2")
	suite.NoError(err)
}

// TestInboundService_TrafficOperations 测试流量操作
func (suite *DatabaseTestSuite) TestInboundService_TrafficOperations() {
	inboundService := &InboundService{}
	inboundService.db = suite.gormDB
	
	// 创建测试入站
	inbound := &model.Inbound{
		UserId:   1,
		Port:     8080,
		Protocol: model.VLESS,
		Remark:   "Test Inbound",
		Enable:   true,
		Settings: `{"clients":[{"id":"client1","email":"client1@example.com","enable":true}]}`,
		Tag:      "inbound-8080",
		Up:       1000,
		Down:     2000,
		Total:    3000,
	}
	err := inboundService.CreateInbound(inbound)
	suite.NoError(err)
	
	// 测试重置流量
	err = inboundService.ResetClientTraffic(1, "client1@example.com")
	suite.NoError(err)
	
	// 测试重置所有流量
	err = inboundService.ResetAllTraffics()
	suite.NoError(err)
	
	// 验证流量已重置
	var updatedInbound model.Inbound
	err = suite.gormDB.Where("id = ?", 1).First(&updatedInbound).Error()
	suite.NoError(err)
	suite.Equal(int64(0), updatedInbound.Up)
	suite.Equal(int64(0), updatedInbound.Down)
	suite.Equal(int64(0), updatedInbound.Total)
}

// TestDatabase_Transaction 测试数据库事务
func (suite *DatabaseTestSuite) TestDatabase_Transaction() {
	inboundService := &InboundService{}
	inboundService.db = suite.gormDB
	
	// 开始事务
	tx := suite.gormDB.Begin()
	
	// 创建入站
	inbound := &model.Inbound{
		UserId:   1,
		Port:     8080,
		Protocol: model.VLESS,
		Remark:   "Test Inbound",
		Enable:   true,
		Tag:      "inbound-8080",
	}
	
	err := tx.Create(inbound).Error()
	suite.NoError(err)
	
	// 提交事务
	tx.Commit()
	
	// 验证数据已保存
	var savedInbound model.Inbound
	err = suite.gormDB.Where("port = ?", 8080).First(&savedInbound).Error()
	suite.NoError(err)
}

// TestDatabase_Concurrency 测试并发数据库操作
func (suite *DatabaseTestSuite) TestDatabase_Concurrency() {
	inboundService := &InboundService{}
	inboundService.db = suite.gormDB
	
	// 创建多个入站的并发测试
	done := make(chan bool, 10)
	
	for i := 0; i < 10; i++ {
		go func(index int) {
			defer func() { done <- true }()
			
			inbound := &model.Inbound{
				UserId:   1,
				Port:     8000 + index,
				Protocol: model.VLESS,
				Remark:   "Concurrent Test Inbound",
				Enable:   true,
				Tag:      "inbound-" + string(rune(8000+index)),
			}
			
			err := inboundService.CreateInbound(inbound)
			if err != nil {
				suite.T().Errorf("Failed to create inbound %d: %v", index, err)
			}
		}(i)
	}
	
	// 等待所有goroutine完成
	for i := 0; i < 10; i++ {
		<-done
	}
	
	// 验证所有入站都已创建
	var count int64
	err := suite.gormDB.Model(&model.Inbound{}).Where("remark = ?", "Concurrent Test Inbound").Count(&count).Error()
	suite.NoError(err)
	suite.Equal(int64(10), count)
}

// BenchmarkDatabase_CreateInbound 性能测试
func (suite *DatabaseTestSuite) BenchmarkDatabase_CreateInbound(b *testing.B) {
	inboundService := &InboundService{}
	inboundService.db = suite.gormDB
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		inbound := &model.Inbound{
			UserId:   1,
			Port:     10000 + i,
			Protocol: model.VLESS,
			Remark:   "Benchmark Inbound",
			Enable:   true,
			Tag:      "inbound-" + string(rune(10000+i)),
		}
		
		_ = inboundService.CreateInbound(inbound)
	}
}

// TestDatabase_ConnectionPool 测试连接池
func (suite *DatabaseTestSuite) TestDatabase_ConnectionPool() {
	// 测试数据库连接池配置
	db := suite.db
	
	// 设置连接池参数
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)
	
	// 验证连接池配置
	assert.Equal(suite.T(), 10, db.Stats().MaxOpenConnections)
	assert.Equal(suite.T(), 5, db.Stats().MaxIdleConnections)
}

// 运行数据库测试套件
func TestDatabaseTestSuite(t *testing.T) {
	suite.Run(t, new(DatabaseTestSuite))
}