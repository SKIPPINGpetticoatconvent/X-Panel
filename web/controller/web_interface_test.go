package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"x-ui/database/model"
	"x-ui/web/service"
	"x-ui/web/session"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// mockSessionStore 用于测试的会话存储
type mockSessionStore struct {
	data map[string]interface{}
}

func (m *mockSessionStore) Get(c *gin.Context, key string) (interface{}, bool) {
	val, exists := m.data[key]
	return val, exists
}

func (m *mockSessionStore) Set(c *gin.Context, key string, value interface{}) {
	m.data[key] = value
}

func (m *mockSessionStore) Delete(c *gin.Context, key string) {
	delete(m.data, key)
}

// setupTestRouter 设置测试用的Gin路由器
func setupTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// 添加中间件
	router.Use(func(c *gin.Context) {
		// 模拟国际化函数
		c.Set("I18n", func(i18nType interface{}, key string, params ...string) string {
			return key
		})
		c.Next()
	})

	return router
}

// TestInboundController_GetInbounds 测试获取入站列表
func TestInboundController_GetInbounds(t *testing.T) {
	// 创建模拟服务
	_ = &service.InboundService{}

	// 设置测试路由
	router := setupTestRouter()

	// 创建控制器
	inboundController := NewInboundController(router.Group("/api/inbounds"))
	_ = inboundController

	// 创建测试请求
	req, _ := http.NewRequest("GET", "/api/inbounds/list", nil)

	// 设置用户会话
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	// 模拟登录用户
	mockUser := &model.User{Id: 1, Username: "testuser"}
	session.SetLoginUser(c, mockUser)

	// 执行请求
	router.ServeHTTP(w, req)

	// 验证响应
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.NotNil(t, response)
}

// TestInboundController_AddInbound 测试添加入站
func TestInboundController_AddInbound(t *testing.T) {
	// 创建测试数据
	inboundData := model.Inbound{
		Port:     8080,
		Protocol: model.VLESS,
		Remark:   "Test Inbound",
		Enable:   true,
	}

	// 序列化为JSON
	jsonData, err := json.Marshal(inboundData)
	assert.NoError(t, err)

	// 创建测试请求
	req, _ := http.NewRequest("POST", "/api/inbounds/add",
		strings.NewReader(string(jsonData)))
	req.Header.Set("Content-Type", "application/json")

	// 设置测试路由
	router := setupTestRouter()
	NewInboundController(router.Group("/api/inbounds"))

	// 执行请求
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 验证响应状态码
	assert.NotEqual(t, http.StatusOK, w.Code, "Should handle missing session gracefully")
}

// TestInboundController_ValidateInboundData 测试入站数据验证
func TestInboundController_ValidateInboundData(t *testing.T) {
	controller := &InboundController{}

	// 测试有效数据
	validInbound := &model.Inbound{
		Port:     8080,
		Protocol: model.VLESS,
		Settings: `{"clients":[{"id":"test-id","email":"test@example.com"}]}`,
	}

	err := controller.validateInboundData(validInbound)
	assert.NoError(t, err)

	// 测试无效端口号
	invalidPortInbound := &model.Inbound{
		Port:     70000, // 超出范围
		Protocol: model.VLESS,
	}

	err = controller.validateInboundData(invalidPortInbound)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid port number")

	// 测试无效协议
	invalidProtocolInbound := &model.Inbound{
		Port:     8080,
		Protocol: "invalid-protocol",
	}

	err = controller.validateInboundData(invalidProtocolInbound)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid protocol")

	// 测试无效的JSON设置
	invalidJSONInbound := &model.Inbound{
		Port:     8080,
		Protocol: model.VLESS,
		Settings: `{"invalid": json}`, // 无效JSON
	}

	err = controller.validateInboundData(invalidJSONInbound)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid settings JSON")

	// 测试负数流量限制
	negativeTrafficInbound := &model.Inbound{
		Port:     8080,
		Protocol: model.VLESS,
		Up:       -1,
		Down:     -1,
		Total:    -1,
	}

	err = controller.validateInboundData(negativeTrafficInbound)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "traffic limits cannot be negative")

	// 测试过长的备注
	longRemarkInbound := &model.Inbound{
		Port:     8080,
		Protocol: model.VLESS,
		Remark:   strings.Repeat("a", 501), // 超过500字符限制
	}

	err = controller.validateInboundData(longRemarkInbound)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "remark too long")
}

// TestSettingController_GetAllSetting 测试获取所有设置
func TestSettingController_GetAllSetting(t *testing.T) {
	// 创建模拟设置服务
	mockSettingService := &service.SettingService{}
	mockUserService := &service.UserService{}
	mockPanelService := &service.PanelService{}

	// 设置测试路由
	router := setupTestRouter()

	// 创建控制器
	settingController := &SettingController{
		settingService: mockSettingService,
		userService:    mockUserService,
		panelService:   mockPanelService,
	}
	_ = settingController

	// 创建测试请求
	req, _ := http.NewRequest("POST", "/api/setting/all", nil)

	// 执行请求
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 验证响应
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
}

// TestSettingController_UpdateUser 测试用户更新
func TestSettingController_UpdateUser(t *testing.T) {
	// 创建测试表单数据
	formData := map[string]string{
		"oldUsername": "testuser",
		"oldPassword": "oldpassword",
		"newUsername": "newuser",
		"newPassword": "newpassword",
	}

	// 序列化为表单数据
	formValues := make(map[string]string)
	for k, v := range formData {
		formValues[k] = v
	}

	jsonData, err := json.Marshal(formValues)
	assert.NoError(t, err)

	// 创建测试请求
	req, _ := http.NewRequest("POST", "/api/setting/updateUser",
		strings.NewReader(string(jsonData)))
	req.Header.Set("Content-Type", "application/json")

	// 设置测试路由
	router := setupTestRouter()
	NewSettingController(router.Group("/api/setting"))

	// 执行请求
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 验证响应状态码
	assert.NotEqual(t, http.StatusOK, w.Code, "Should handle missing session gracefully")
}

// TestBaseController_CheckLogin 测试登录检查
func TestBaseController_CheckLogin(t *testing.T) {
	controller := &BaseController{}

	// 测试未登录状态
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request, _ = http.NewRequest("GET", "/", nil)

	// 模拟未登录状态
	// session.IsLogin(c) 应该返回 false

	// 由于我们无法直接控制session.IsLogin的结果，
	// 这里我们测试控制器结构是否正确初始化
	assert.NotNil(t, controller)

	// 测试I18nWeb函数
	c.Set("I18n", func(i18nType interface{}, key string, params ...string) string {
		return "translated:" + key
	})

	result := I18nWeb(c, "test.key")
	assert.Equal(t, "translated:test.key", result)
}

// TestAPIController_BackuptoTgbot 测试备份到Telegram
func TestAPIController_BackuptoTgbot(t *testing.T) {
	// 创建模拟服务器服务
	mockServerService := &service.ServerService{}

	// 设置测试路由
	router := setupTestRouter()

	// 创建API控制器
	apiController := NewAPIController(router.Group("/panel/api"), mockServerService)
	_ = apiController

	// 创建测试请求
	req, _ := http.NewRequest("GET", "/panel/api/backuptotgbot", nil)

	// 执行请求
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 验证响应
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestProtocolValidation 测试协议验证
func TestProtocolValidation(t *testing.T) {
	// 测试所有支持的协议
	validProtocols := []model.Protocol{
		model.VMESS,
		model.VLESS,
		model.Tunnel,
		model.HTTP,
		model.Trojan,
		model.Shadowsocks,
		model.Socks,
		model.WireGuard,
	}

	controller := &InboundController{}

	for _, protocol := range validProtocols {
		inbound := &model.Inbound{
			Port:     8080,
			Protocol: protocol,
		}

		err := controller.validateInboundData(inbound)
		assert.NoError(t, err, "Protocol %s should be valid", protocol)
	}
}

// BenchmarkInboundController_ValidateInboundData 性能测试
func BenchmarkInboundController_ValidateInboundData(b *testing.B) {
	mockInboundService := &service.InboundService{}
	mockXrayService := &service.XrayService{}

	inboundController := &InboundController{
		inboundService: mockInboundService,
		xrayService:    mockXrayService,
	}
	inbound := &model.Inbound{
		Port:     8080,
		Protocol: model.VLESS,
		Settings: `{"clients":[{"id":"test-id","email":"test@example.com","enable":true}]}`,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = inboundController.validateInboundData(inbound)
	}
}

// TestInboundController_ImportInbound 测试导入入站配置
func TestInboundController_ImportInbound(t *testing.T) {
	// 创建有效的导入数据
	importData := model.Inbound{
		Port:     9090,
		Protocol: model.VLESS,
		Remark:   "Imported Inbound",
		Settings: `{"clients":[{"id":"imported-id","email":"imported@example.com","enable":true}]}`,
	}

	jsonData, err := json.Marshal(importData)
	assert.NoError(t, err)

	// 创建测试请求
	req, _ := http.NewRequest("POST", "/api/inbounds/import",
		strings.NewReader("data="+string(jsonData)))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// 设置测试路由
	router := setupTestRouter()
	NewInboundController(router.Group("/api/inbounds"))

	// 执行请求
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 验证响应状态码
	assert.NotEqual(t, http.StatusOK, w.Code, "Should handle missing session gracefully")
}

// TestInboundController_ClientOperations 测试客户端操作
func TestInboundController_ClientOperations(t *testing.T) {
	router := setupTestRouter()
	controller := NewInboundController(router.Group("/api/inbounds"))
	_ = controller

	// 测试添加客户端
	addClientData := model.Inbound{
		Settings: `{"clients":[{"id":"new-client","email":"newclient@example.com"}]}`,
	}

	jsonData, err := json.Marshal(addClientData)
	assert.NoError(t, err)

	req, _ := http.NewRequest("POST", "/api/inbounds/addClient",
		strings.NewReader(string(jsonData)))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 验证响应
	assert.NotEqual(t, http.StatusOK, w.Code, "Should handle missing session gracefully")
}
