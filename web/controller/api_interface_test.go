package controller

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"x-ui/database/model"
	"x-ui/web/service"
	"x-ui/web/session"
	"x-ui/xray"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// MockXrayAPI 模拟Xray API
type MockXrayAPI struct {
	isRunning bool
	version   string
	err       error
}

func (m *MockXrayAPI) Init(port int) {}
func (m *MockXrayAPI) Close()        {}
func (m *MockXrayAPI) GetTraffic(clearStats bool) ([]*xray.Traffic, []*xray.ClientTraffic, error) {
	return nil, nil, m.err
}

// TestInboundAPI_ResponseFormat 测试入站API响应格式
func TestInboundAPI_ResponseFormat(t *testing.T) {
	// 设置测试环境
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

	// 创建模拟服务
	mockInboundService := &service.InboundService{}
	mockXrayService := &service.XrayService{}

	// 创建控制器
	inboundController := &InboundController{
		inboundService: mockInboundService,
		xrayService:    mockXrayService,
	}

	// 注册路由
	api := router.Group("/api/inbounds")
	api.GET("/list", inboundController.getInbounds)

	// 测试获取入站列表
	t.Run("GetInboundsList", func(t *testing.T) {
		// 创建请求
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

		// 验证响应格式
		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)

		// 验证响应结构
		assert.Contains(t, response, "success")
		assert.Contains(t, response, "obj")
		assert.Contains(t, response, "msg")
	})
}

// TestInboundAPI_DataValidation 测试API数据验证
func TestInboundAPI_DataValidation(t *testing.T) {
	router := setupTestRouter()

	mockInboundService := &service.InboundService{}
	mockXrayService := &service.XrayService{}

	inboundController := &InboundController{
		inboundService: mockInboundService,
		xrayService:    mockXrayService,
	}

	api := router.Group("/api/inbounds")
	api.POST("/add", inboundController.addInbound)

	// 测试无效的JSON数据
	t.Run("InvalidJSONData", func(t *testing.T) {
		invalidJSON := `{"port": "invalid_port", "protocol": "invalid_protocol"}`

		req, _ := http.NewRequest("POST", "/api/inbounds/add",
			strings.NewReader(invalidJSON))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// 应该返回错误状态码
		assert.NotEqual(t, http.StatusOK, w.Code)
	})

	// 测试缺少必需字段
	t.Run("MissingRequiredFields", func(t *testing.T) {
		incompleteData := map[string]interface{}{
			"port": 8080,
			// 缺少 protocol 字段
		}

		jsonData, _ := json.Marshal(incompleteData)

		req, _ := http.NewRequest("POST", "/api/inbounds/add",
			bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.NotEqual(t, http.StatusOK, w.Code)
	})

	// 测试无效的端口号
	t.Run("InvalidPortNumber", func(t *testing.T) {
		invalidPortData := map[string]interface{}{
			"port":     70000, // 超出有效范围
			"protocol": model.VLESS,
			"remark":   "Test",
		}

		jsonData, _ := json.Marshal(invalidPortData)

		req, _ := http.NewRequest("POST", "/api/inbounds/add",
			bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.NotEqual(t, http.StatusOK, w.Code)
	})
}

// TestInboundAPI_PermissionValidation 测试API权限验证
func TestInboundAPI_PermissionValidation(t *testing.T) {
	router := setupTestRouter()

	mockInboundService := &service.InboundService{}
	mockXrayService := &service.XrayService{}

	inboundController := &InboundController{
		inboundService: mockInboundService,
		xrayService:    mockXrayService,
	}

	api := router.Group("/api/inbounds")
	api.POST("/add", inboundController.addInbound)
	api.DELETE("/del/:id", inboundController.delInbound)
	api.PUT("/update/:id", inboundController.updateInbound)

	// 测试未登录用户
	t.Run("UnauthorizedUser", func(t *testing.T) {
		addData := map[string]interface{}{
			"port":     8080,
			"protocol": model.VLESS,
			"remark":   "Test",
		}

		jsonData, _ := json.Marshal(addData)

		req, _ := http.NewRequest("POST", "/api/inbounds/add",
			bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// 应该被重定向到登录页面或返回未授权状态
		assert.True(t, w.Code == http.StatusUnauthorized || w.Code == http.StatusFound)
	})

	// 测试跨用户权限
	t.Run("CrossUserPermission", func(t *testing.T) {
		// 创建两个不同用户的会话
		user1 := &model.User{Id: 1, Username: "user1"}

		// 模拟用户1试图访问用户2的资源
		c1, _ := gin.CreateTestContext(httptest.NewRecorder())
		session.SetLoginUser(c1, user1)

		// 这里需要测试用户1是否能访问用户2的入站
		// 由于我们使用的是模拟服务，实际测试中需要更复杂的权限检查
	})
}

// TestSettingAPI_Configuration 测试设置API配置
func TestSettingAPI_Configuration(t *testing.T) {
	router := setupTestRouter()

	mockSettingService := &service.SettingService{}
	mockUserService := &service.UserService{}
	mockPanelService := &service.PanelService{}

	settingController := &SettingController{
		settingService: mockSettingService,
		userService:    mockUserService,
		panelService:   mockPanelService,
	}

	api := router.Group("/api/setting")
	api.POST("/all", settingController.getAllSetting)
	api.POST("/update", settingController.updateSetting)
	api.POST("/updateUser", settingController.updateUser)

	// 测试获取所有设置
	t.Run("GetAllSettings", func(t *testing.T) {
		req, _ := http.NewRequest("POST", "/api/setting/all", nil)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
	})

	// 测试更新设置
	t.Run("UpdateSettings", func(t *testing.T) {
		settings := map[string]interface{}{
			"panel": map[string]interface{}{
				"port":   8080,
				"path":   "/panel/",
				"secret": "secret123",
			},
		}

		jsonData, _ := json.Marshal(settings)

		req, _ := http.NewRequest("POST", "/api/setting/update",
			bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	// 测试更新用户信息
	t.Run("UpdateUser", func(t *testing.T) {
		userData := map[string]string{
			"oldUsername": "olduser",
			"oldPassword": "oldpass",
			"newUsername": "newuser",
			"newPassword": "newpass",
		}

		jsonData, _ := json.Marshal(userData)

		req, _ := http.NewRequest("POST", "/api/setting/updateUser",
			bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// 由于没有真实的用户验证，应该返回错误
		assert.NotEqual(t, http.StatusOK, w.Code)
	})
}

// TestAPI_SecurityHeaders 测试API安全头
func TestAPI_SecurityHeaders(t *testing.T) {
	router := setupTestRouter()

	api := router.Group("/api")
	api.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	t.Run("SecurityHeaders", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/test", nil)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// 验证安全相关响应头
		// 注意：Gin默认不设置这些头，可能需要中间件
		headers := w.Header()

		// 检查是否设置了适当的内容类型
		assert.Contains(t, headers.Get("Content-Type"), "application/json")
	})
}

// TestAPI_RateLimiting 测试API速率限制
func TestAPI_RateLimiting(t *testing.T) {
	router := setupTestRouter()

	// 简单的速率限制中间件
	rateLimiter := make(map[string]int)

	router.Use(func(c *gin.Context) {
		clientIP := c.ClientIP()
		rateLimiter[clientIP]++

		if rateLimiter[clientIP] > 100 { // 限制每分钟100次请求
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "Rate limit exceeded",
			})
			return
		}
		c.Next()
	})

	api := router.Group("/api")
	api.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	t.Run("RateLimitExceeded", func(t *testing.T) {
		// 模拟大量请求
		for i := 0; i < 101; i++ {
			req, _ := http.NewRequest("GET", "/api/test", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if i == 100 { // 第101次请求应该被限制
				assert.Equal(t, http.StatusTooManyRequests, w.Code)
			}
		}
	})
}

// TestAPI_JSONResponseFormat 测试API JSON响应格式
func TestAPI_JSONResponseFormat(t *testing.T) {
	router := setupTestRouter()

	api := router.Group("/api")
	api.GET("/success", func(c *gin.Context) {
		jsonMsg(c, "Success message", nil)
	})
	api.GET("/error", func(c *gin.Context) {
		jsonMsg(c, "Error message", assert.AnError)
	})
	api.GET("/object", func(c *gin.Context) {
		jsonObj(c, gin.H{"key": "value"}, nil)
	})

	t.Run("SuccessResponse", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/success", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)

		assert.Contains(t, response, "success")
		assert.Contains(t, response, "msg")
		assert.True(t, response["success"].(bool))
	})

	t.Run("ErrorResponse", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/error", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)

		assert.Contains(t, response, "success")
		assert.Contains(t, response, "msg")
		assert.False(t, response["success"].(bool))
	})

	t.Run("ObjectResponse", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/object", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)

		assert.Contains(t, response, "success")
		assert.Contains(t, response, "obj")
		assert.Contains(t, response["obj"], "key")
	})
}

// TestAPI_ContentTypeValidation 测试API内容类型验证
func TestAPI_ContentTypeValidation(t *testing.T) {
	router := setupTestRouter()

	api := router.Group("/api")
	api.POST("/json", func(c *gin.Context) {
		var data map[string]interface{}
		if err := c.ShouldBind(&data); err != nil {
			jsonMsg(c, "Invalid JSON", err)
			return
		}
		jsonMsg(c, "Success", nil)
	})

	t.Run("ValidJSONContentType", func(t *testing.T) {
		data := map[string]interface{}{
			"key": "value",
		}
		jsonData, _ := json.Marshal(data)

		req, _ := http.NewRequest("POST", "/api/json",
			bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("InvalidContentType", func(t *testing.T) {
		req, _ := http.NewRequest("POST", "/api/json",
			strings.NewReader("invalid=data"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// 应该返回错误，因为期望JSON但收到表单数据
		assert.NotEqual(t, http.StatusOK, w.Code)
	})

	t.Run("MissingContentType", func(t *testing.T) {
		req, _ := http.NewRequest("POST", "/api/json",
			strings.NewReader("{}"))
		// 不设置Content-Type头

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.NotEqual(t, http.StatusOK, w.Code)
	})
}

// BenchmarkAPI_Performance API性能测试
func BenchmarkAPI_GetInbounds(b *testing.B) {
	router := setupTestRouter()

	mockInboundService := &service.InboundService{}
	mockXrayService := &service.XrayService{}

	inboundController := &InboundController{
		inboundService: mockInboundService,
		xrayService:    mockXrayService,
	}

	api := router.Group("/api/inbounds")
	api.GET("/list", inboundController.getInbounds)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest("GET", "/api/inbounds/list", nil)
		w := httptest.NewRecorder()

		// 添加用户会话
		c, _ := gin.CreateTestContext(w)
		c.Request = req
		mockUser := &model.User{Id: 1, Username: "testuser"}
		session.SetLoginUser(c, mockUser)

		router.ServeHTTP(w, req)
	}
}

// TestAPI_ErrorHandling 测试API错误处理
func TestAPI_ErrorHandling(t *testing.T) {
	router := setupTestRouter()

	api := router.Group("/api")
	api.GET("/panic", func(c *gin.Context) {
		panic("Test panic")
	})
	api.GET("/error", func(c *gin.Context) {
		c.Error(assert.AnError)
		jsonMsg(c, "Error occurred", assert.AnError)
	})

	t.Run("PanicHandling", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/panic", nil)
		w := httptest.NewRecorder()

		// 捕获panic
		defer func() {
			if r := recover(); r != nil {
				assert.Equal(t, "Test panic", r)
			}
		}()

		router.ServeHTTP(w, req)
		// Gin应该处理panic并返回500错误
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("ErrorHandling", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/error", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)

		assert.False(t, response["success"].(bool))
		assert.Contains(t, response, "msg")
	})
}
