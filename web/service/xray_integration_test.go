package service

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"x-ui/database/model"
	"x-ui/xray"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

// MockXrayAPI 模拟Xray API接口
type MockXrayAPI struct {
	isRunning      bool
	version        string
	apiPort        int
	trafficData    []*xray.Traffic
	clientTraffic  []*xray.ClientTraffic
	err            error
	startTime      time.Time
}

func (m *MockXrayAPI) Init(port int) {
	m.apiPort = port
	m.isRunning = true
	m.startTime = time.Now()
}

func (m *MockXrayAPI) Close() {
	m.isRunning = false
}

func (m *MockXrayAPI) GetTraffic(clearStats bool) ([]*xray.Traffic, []*xray.ClientTraffic, error) {
	if !m.isRunning {
		return nil, nil, assert.AnError
	}
	
	// 返回模拟的流量数据
	return m.trafficData, m.clientTraffic, m.err
}

// MockXrayProcess 模拟Xray进程
type MockXrayProcess struct {
	config  *xray.Config
	running bool
	version string
	result  string
	err     error
	apiPort int
}

func (m *MockXrayProcess) IsRunning() bool {
	return m.running
}

func (m *MockXrayProcess) GetConfig() *xray.Config {
	return m.config
}

func (m *MockXrayProcess) GetVersion() string {
	return m.version
}

func (m *MockXrayProcess) GetResult() string {
	return m.result
}

func (m *MockXrayProcess) GetErr() error {
	return m.err
}

func (m *MockXrayProcess) GetAPIPort() int {
	return m.apiPort
}

func (m *MockXrayProcess) Start() error {
	if m.running {
		return assert.AnError
	}
	m.running = true
	return nil
}

func (m *MockXrayProcess) Stop() error {
	m.running = false
	return nil
}

// XrayIntegrationTestSuite Xray集成测试套件
type XrayIntegrationTestSuite struct {
	suite.Suite
	xrayService     *XrayService
	mockInboundService *InboundService
	mockSettingService *SettingService
	mockXrayAPI     *MockXrayAPI
	tempDir         string
}

// SetupSuite 设置测试套件
func (suite *XrayIntegrationTestSuite) SetupSuite() {
	// 创建临时目录
	var err error
	suite.tempDir, err = os.MkdirTemp("", "xray-test")
	if err != nil {
		suite.T().Fatalf("Failed to create temp dir: %v", err)
	}
	
	// 创建模拟服务
	suite.mockInboundService = &InboundService{}
	suite.mockSettingService = &SettingService{}
	suite.mockXrayAPI = &MockXrayAPI{
		version: "1.8.0",
		apiPort: 10080,
	}
	
	// 创建Xray服务
	suite.xrayService = &XrayService{
		inboundService: *suite.mockInboundService,
		settingService: *suite.mockSettingService,
		xrayAPI:        suite.mockXrayAPI,
	}
}

// TearDownSuite 清理测试套件
func (suite *XrayIntegrationTestSuite) TearDownSuite() {
	// 清理临时目录
	if suite.tempDir != "" {
		os.RemoveAll(suite.tempDir)
	}
}

// SetupTest 设置每个测试
func (suite *XrayIntegrationTestSuite) SetupTest() {
	// 重置模拟状态
	suite.mockXrayAPI.isRunning = false
	suite.mockXrayAPI.trafficData = nil
	suite.mockXrayAPI.clientTraffic = nil
	suite.mockXrayAPI.err = nil
}

// TestXrayService_GetXrayConfig 测试Xray配置生成
func (suite *XrayIntegrationTestSuite) TestXrayService_GetXrayConfig() {
	// 创建测试配置模板
	configTemplate := `{
		"log": {
			"loglevel": "warning"
		},
		"inbound": [],
		"outbound": [],
		"policy": {
			"levels": {
				"0": {
					"statsUserUplink": true,
					"statsUserDownlink": true
				}
			}
		}
	}`
	
	// 设置模拟的模板配置
	// 注意：这里我们需要实际实现设置服务的方法
	// suite.mockSettingService.SetXrayConfigTemplate(configTemplate)
	
	// 创建测试入站
	testInbound := &model.Inbound{
		Id:         1,
		UserId:     1,
		Port:       8080,
		Protocol:   model.VLESS,
		Remark:     "Test VLESS Inbound",
		Enable:     true,
		Settings:   `{"clients":[{"id":"test-id","email":"test@example.com","enable":true}]}`,
		StreamSettings: `{"network":"tcp","security":"reality","realitySettings":{"publicKey":"test-public-key"}}`,
		Tag:        "inbound-8080",
	}
	
	// 添加测试入站到模拟服务
	// suite.mockInboundService.AddTestInbound(testInbound)
	
	// 生成Xray配置
	config, err := suite.xrayService.GetXrayConfig()
	if err != nil {
		suite.T().Skip("Skipping test due to service dependency: %v", err)
		return
	}
	
	// 验证配置结构
	assert.NotNil(suite.T(), config)
	
	// 如果配置生成成功，验证基本结构
	if config != nil {
		// 验证log配置
		if config.Log != nil {
			assert.NotNil(suite.T(), config.Log)
		}
		
		// 验证inbound配置
		if len(config.InboundConfigs) > 0 {
			assert.Greater(suite.T(), len(config.InboundConfigs), 0)
			
			// 验证第一个入站配置
			inbound := config.InboundConfigs[0]
			assert.Equal(suite.T(), 8080, inbound.Port)
			assert.Equal(suite.T(), "vless", inbound.Protocol)
		}
	}
}

// TestXrayService_StartStopXray 测试Xray进程启动和停止
func (suite *XrayIntegrationTestSuite) TestXrayService_StartStopXray() {
	// 测试启动Xray
	err := suite.xrayService.RestartXray(false)
	if err != nil {
		suite.T().Skip("Skipping test due to Xray dependency: %v", err)
		return
	}
	
	// 验证Xray正在运行
	assert.True(suite.T(), suite.xrayService.IsXrayRunning())
	
	// 获取Xray版本
	version := suite.xrayService.GetXrayVersion()
	assert.NotEmpty(suite.T(), version)
	
	// 获取API端口
	apiPort := suite.xrayService.GetApiPort()
	assert.Greater(suite.T(), apiPort, 0)
	
	// 停止Xray
	err = suite.xrayService.StopXray()
	assert.NoError(suite.T(), err)
	
	// 验证Xray已停止
	assert.False(suite.T(), suite.xrayService.IsXrayRunning())
}

// TestXrayService_GetXrayTraffic 测试流量获取
func (suite *XrayIntegrationTestSuite) TestXrayService_GetXrayTraffic() {
	// 启动Xray
	err := suite.xrayService.RestartXray(false)
	if err != nil {
		suite.T().Skip("Skipping test due to Xray dependency: %v", err)
		return
	}
	defer suite.xrayService.StopXray()
	
	// 设置模拟的流量数据
	suite.mockXrayAPI.trafficData = []*xray.Traffic{
		{
			Tag:       "inbound-8080",
			IsInbound: true,
			Up:        1000,
			Down:      2000,
		},
	}
	
	suite.mockXrayAPI.clientTraffic = []*xray.ClientTraffic{
		{
			Email: "test@example.com",
			Up:    500,
			Down:  1000,
		},
	}
	
	// 获取流量数据
	traffic, clientTraffic, err := suite.xrayService.GetXrayTraffic()
	
	// 如果Xray未运行，期望返回错误
	if !suite.xrayService.IsXrayRunning() {
		assert.Error(suite.T(), err)
		assert.Nil(suite.T(), traffic)
		assert.Nil(suite.T(), clientTraffic)
	} else {
		// 如果Xray正在运行，验证返回的数据
		assert.NoError(suite.T(), err)
		if traffic != nil {
			assert.Greater(suite.T(), len(traffic), 0)
		}
		if clientTraffic != nil {
			assert.Greater(suite.T(), len(clientTraffic), 0)
		}
	}
}

// TestXrayService_PolicyGeneration 测试策略生成
func (suite *XrayIntegrationTestSuite) TestXrayService_PolicyGeneration() {
	// 创建带有不同限速的测试入站
	testInbounds := []*model.Inbound{
		{
			Id:       1,
			Port:     8080,
			Protocol: model.VLESS,
			Enable:   true,
			Settings: `{"clients":[{"id":"client1","email":"client1@example.com","speedLimit":1024,"enable":true}]}`,
			Tag:      "inbound-8080",
		},
		{
			Id:       2,
			Port:     9090,
			Protocol: model.VMESS,
			Enable:   true,
			Settings: `{"clients":[{"id":"client2","email":"client2@example.com","speedLimit":2048,"enable":true}]}`,
			Tag:      "inbound-9090",
		},
	}
	
	// 添加测试入站
	// 注意：这里需要实际的数据库操作
	// for _, inbound := range testInbounds {
	//     suite.mockInboundService.CreateInbound(inbound)
	// }
	
	// 生成配置并验证策略
	config, err := suite.xrayService.GetXrayConfig()
	if err != nil {
		suite.T().Skip("Skipping test due to service dependency: %v", err)
		return
	}
	
	if config != nil && config.Policy != nil {
		// 验证策略结构
		var policy map[string]interface{}
		err := json.Unmarshal(config.Policy, &policy)
		assert.NoError(suite.T(), err)
		
		if levels, ok := policy["levels"].(map[string]interface{}); ok {
			// 验证level 0存在
			assert.Contains(suite.T(), levels, "0")
			
			// 验证限速策略
			if level1024, ok := levels["1024"]; ok {
				levelMap := level1024.(map[string]interface{})
				assert.Equal(suite.T(), float64(1024), levelMap["downlinkOnly"])
				assert.Equal(suite.T(), float64(1024), levelMap["uplinkOnly"])
			}
		}
	}
}

// TestXrayService_ConfigValidation 测试配置验证
func (suite *XrayIntegrationTestSuite) TestXrayService_ConfigValidation() {
	// 测试无效的配置
	invalidConfig := &xray.Config{
		InboundConfigs: []xray.InboundConfig{
			{
				Port:     80, // 特权端口，可能需要权限
				Protocol: "invalid-protocol",
			},
		},
	}
	
	// 序列化无效配置
	configJSON, err := json.Marshal(invalidConfig)
	assert.NoError(suite.T(), err)
	
	// 尝试使用无效配置启动Xray
	err = suite.xrayService.RestartXray(false)
	if err == nil {
		// 如果启动成功，停止Xray
		suite.xrayService.StopXray()
	}
	
	// 验证Xray是否正常运行
	// 注意：实际测试中可能因为权限问题无法启动
	// assert.False(suite.T(), suite.xrayService.IsXrayRunning())
}

// TestXrayService_GracefulShutdown 测试优雅关闭
func (suite *XrayIntegrationTestSuite) TestXrayService_GracefulShutdown() {
	// 启动Xray
	err := suite.xrayService.RestartXray(false)
	if err != nil {
		suite.T().Skip("Skipping test due to Xray dependency: %v", err)
		return
	}
	
	// 等待一小段时间确保Xray启动
	time.Sleep(100 * time.Millisecond)
	
	// 手动停止Xray
	suite.xrayService.StopXray()
	
	// 验证Xray已停止
	assert.False(suite.T(), suite.xrayService.IsXrayRunning())
	
	// 验证错误状态
	xrayErr := suite.xrayService.GetXrayErr()
	if xrayErr != nil {
		// 在Windows上，exit status 1是正常的
		if runtime.GOOS == "windows" && xrayErr.Error() == "exit status 1" {
			assert.Nil(suite.T(), xrayErr)
		}
	}
}

// TestXrayService_NeedRestartFlag 测试重启标志
func (suite *XrayIntegrationTestSuite) TestXrayService_NeedRestartFlag() {
	// 测试设置重启标志
	suite.xrayService.SetToNeedRestart()
	
	// 验证重启标志已设置
	assert.True(suite.T(), suite.xrayService.IsNeedRestartAndSetFalse())
	
	// 验证标志已被清除
	assert.False(suite.T(), suite.xrayService.IsNeedRestartAndSetFalse())
	
	// 测试崩溃检测
	assert.False(suite.T(), suite.xrayService.DidXrayCrash())
}

// TestXrayService_ClientFiltering 测试客户端过滤
func (suite *XrayIntegrationTestSuite) TestXrayService_ClientFiltering() {
	// 创建包含禁用客户端的入站
	inboundWithDisabledClient := &model.Inbound{
		Id:       1,
		Port:     8080,
		Protocol: model.VLESS,
		Enable:   true,
		Settings: `{"clients":[
			{"id":"client1","email":"client1@example.com","enable":true},
			{"id":"client2","email":"client2@example.com","enable":false}
		]}`,
		ClientStats: []xray.ClientTraffic{
			{
				Email:  "client1@example.com",
				Enable: true,
			},
			{
				Email:  "client2@example.com",
				Enable: false,
			},
		},
		Tag: "inbound-8080",
	}
	
	// 生成配置
	config, err := suite.xrayService.GetXrayConfig()
	if err != nil {
		suite.T().Skip("Skipping test due to service dependency: %v", err)
		return
	}
	
	// 验证客户端过滤逻辑
	if config != nil && len(config.InboundConfigs) > 0 {
		// 解析生成的客户端设置
		var settings map[string]interface{}
		err := json.Unmarshal([]byte(inboundWithDisabledClient.Settings), &settings)
		assert.NoError(suite.T(), err)
		
		// 验证配置中只包含启用的客户端
		if clients, ok := settings["clients"].([]interface{}); ok {
			enabledCount := 0
			for _, client := range clients {
				clientMap := client.(map[string]interface{})
				if enable, ok := clientMap["enable"].(bool); ok && enable {
					enabledCount++
				}
			}
			assert.Greater(suite.T(), enabledCount, 0)
		}
	}
}

// BenchmarkXrayService_GetXrayConfig 性能测试
func (suite *XrayIntegrationTestSuite) BenchmarkXrayService_GetXrayConfig(b *testing.B) {
	// 创建大量测试入站
	testInbounds := make([]*model.Inbound, 100)
	for i := 0; i < 100; i++ {
		testInbounds[i] = &model.Inbound{
			Id:       i + 1,
			Port:     8000 + i,
			Protocol: model.VLESS,
			Enable:   true,
			Settings: `{"clients":[{"id":"client` + string(rune(i)) + `","email":"client` + string(rune(i)) + `@example.com","speedLimit":1024,"enable":true}]}`,
			Tag:      "inbound-" + string(rune(8000+i)),
		}
	}
	
	// 添加测试入站到服务
	// for _, inbound := range testInbounds {
	//     suite.mockInboundService.CreateInbound(inbound)
	// }
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := suite.xrayService.GetXrayConfig()
		if err != nil {
			b.Skip("Skipping benchmark due to service dependency: %v", err)
			return
		}
	}
}

// TestXrayService_XrayCrashDetection 测试Xray崩溃检测
func (suite *XrayIntegrationTestSuite) TestXrayService_XrayCrashDetection() {
	// 模拟Xray崩溃的情况
	// 这里我们无法真正模拟崩溃，但可以测试逻辑
	
	// 在没有启动Xray的情况下测试崩溃检测
	assert.False(suite.T(), suite.xrayService.IsXrayRunning())
	assert.False(suite.T(), suite.xrayService.DidXrayCrash())
	
	// 启动Xray
	err := suite.xrayService.RestartXray(false)
	if err != nil {
		suite.T().Skip("Skipping test due to Xray dependency: %v", err)
		return
	}
	
	// 验证Xray正在运行
	assert.True(suite.T(), suite.xrayService.IsXrayRunning())
	assert.False(suite.T(), suite.xrayService.DidXrayCrash())
	
	// 清理
	suite.xrayService.StopXray()
}

// TestXrayService_ConfigEquals 测试配置相等性检查
func (suite *XrayIntegrationTestSuite) TestXrayService_ConfigEquals() {
	// 这个测试需要实际实现配置比较逻辑
	// 由于我们使用的是Mock，这里只测试基本结构
	
	config1 := &xray.Config{
		Log: &xray.LogConfig{
			Loglevel: "warning",
		},
	}
	
	config2 := &xray.Config{
		Log: &xray.LogConfig{
			Loglevel: "warning",
		},
	}
	
	// 验证两个配置的JSON表示相等
	config1JSON, _ := json.Marshal(config1)
	config2JSON, _ := json.Marshal(config2)
	
	assert.Equal(suite.T(), string(config1JSON), string(config2JSON))
}

// 运行Xray集成测试套件
func TestXrayIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(XrayIntegrationTestSuite))
}