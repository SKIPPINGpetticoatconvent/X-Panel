package service

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// mockTransport 用于测试的自定义 HTTP 传输层
type mockTransport struct {
	server *httptest.Server
}

func (t *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// 直接将请求转发到模拟服务器
	targetURL := t.server.URL + req.URL.Path + "?" + req.URL.RawQuery

	newReq, err := http.NewRequestWithContext(req.Context(), req.Method, targetURL, req.Body)
	if err != nil {
		return nil, err
	}

	// 复制请求头
	for k, v := range req.Header {
		newReq.Header[k] = v
	}

	return http.DefaultTransport.RoundTrip(newReq)
}

// TestGeoIPServiceInitialization 测试 GeoIP 服务初始化
func TestGeoIPServiceInitialization(t *testing.T) {
	// 创建 GeoIP 服务实例
	geoIPService := NewGeoIPService()

	if geoIPService == nil {
		t.Error("NewGeoIPService() 返回了 nil")
	}

	if geoIPService.client == nil {
		t.Error("GeoIP 服务的 HTTP 客户端未初始化")
	}

	// 测试获取国家代码（实际 API 调用）
	t.Log("开始测试 GeoIP API 调用...")
	countryCode := geoIPService.GetCountryCode()

	if countryCode == "" {
		t.Error("GetCountryCode() 返回了空字符串")
	}

	t.Logf("检测到的国家代码: %s", countryCode)

	// 验证国家代码格式（应该是2-3个大写字母）
	if len(countryCode) < 2 || len(countryCode) > 3 {
		t.Errorf("国家代码格式不正确: %s", countryCode)
	}
}

// TestSNISelectorWithGeoIP 测试 SNI 选择器与 GeoIP 服务的集成
func TestSNISelectorWithGeoIP(t *testing.T) {
	// 创建 GeoIP 服务
	geoIPService := NewGeoIPService()

	if geoIPService == nil {
		t.Fatal("GeoIP 服务创建失败")
	}

	// 创建带有 GeoIP 服务的 SNI 选择器
	initialDomains := []string{"www.google.com:443", "www.amazon.com:443", "www.microsoft.com:443"}
	sniSelector := NewSNISelectorWithGeoIP(initialDomains, geoIPService)

	if sniSelector == nil {
		t.Error("NewSNISelectorWithGeoIP() 返回了 nil")
	}

	if sniSelector.geoIPService != geoIPService {
		t.Error("GeoIP 服务未正确注入到 SNI 选择器")
	}

	// 测试获取 GeoIP 信息
	geoIPInfo := sniSelector.GetGeoIPInfo()
	if geoIPInfo == "" {
		t.Error("GetGeoIPInfo() 返回了空字符串")
	}

	t.Logf("GeoIP 信息: %s", geoIPInfo)
}

// TestServerServiceWithGeoIP 测试 ServerService 与 GeoIP 的集成
func TestServerServiceWithGeoIP(t *testing.T) {
	// 创建 ServerService 实例
	serverService := &ServerService{}

	// 初始化 GeoIP 服务
	serverService.geoIPService = NewGeoIPService()

	if serverService.geoIPService == nil {
		t.Error("ServerService 的 GeoIP 服务未初始化")
	}

	// 测试获取地理位置
	countryCode := serverService.geoIPService.GetCountryCode()
	if countryCode == "" {
		t.Error("从 ServerService 获取国家代码失败")
	}

	t.Logf("ServerService 检测到的国家代码: %s", countryCode)

	// 测试获取详细的地理位置信息
	geoIPInfo := serverService.GetGeoIPInfo()
	if geoIPInfo == "" {
		t.Error("GetGeoIPInfo() 返回了空字符串")
	}

	t.Logf("详细 GeoIP 信息: %s", geoIPInfo)

	// 测试 SNI 选择器初始化
	// 注意：这里可能会因为文件路径问题而失败，但至少可以测试初始化逻辑
	t.Log("开始测试 SNI 选择器初始化...")
	serverService.initSNISelector()

	if serverService.sniSelector == nil {
		t.Error("SNI 选择器初始化失败")
	} else {
		t.Log("SNI 选择器初始化成功")

		// 测试获取 SNI 域名
		sni := serverService.GetNewSNI()
		if sni == "" {
			t.Error("GetNewSNI() 返回了空字符串")
		} else {
			t.Logf("获取到的 SNI 域名: %s", sni)
		}
	}
}

// TestGeoIPServiceRetry 测试 GeoIP 服务的重试机制
func TestGeoIPServiceRetry(t *testing.T) {
	geoIPService := NewGeoIPService()

	if geoIPService == nil {
		t.Fatal("GeoIP 服务创建失败")
	}

	// 测试带重试的获取
	location, err := geoIPService.FetchLocationWithRetry(3)

	if err != nil {
		t.Errorf("FetchLocationWithRetry(3) 失败: %v", err)
	}

	if location == nil {
		t.Error("FetchLocationWithRetry() 返回了 nil location")
	}

	if location.GetCountryCode() == "" {
		t.Error("位置的 CountryCode 为空")
	}

	t.Logf("重试获取成功 - 国家: %s, 代码: %s, IP: %s",
		location.GetCountry(), location.GetCountryCode(), location.IP)
}

// TestGeoIPLocationStruct 测试 GeoIPLocation 结构体
func TestGeoIPLocationStruct(t *testing.T) {
	// 创建一个模拟的 GeoIP 响应数据来测试结构体
	mockLocation := GeoIPLocation{
		IP: "8.8.8.8",
		Location: struct {
			City        string `json:"city"`
			CountryCode string `json:"country_code"`
			CountryName string `json:"country_name"`
			Latitude    string `json:"latitude"`
			Longitude   string `json:"longitude"`
			Province    string `json:"province"`
		}{
			City:        "Mountain View",
			CountryCode: "US",
			CountryName: "United States",
			Latitude:    "37.4056",
			Longitude:   "-122.0775",
			Province:    "California",
		},
	}

	// 验证结构体字段
	if mockLocation.IP != "8.8.8.8" {
		t.Error("IP 字段设置不正确")
	}

	if mockLocation.GetCountryCode() != "US" {
		t.Error("CountryCode 字段设置不正确")
	}

	if mockLocation.GetCountry() != "United States" {
		t.Error("Country 字段设置不正确")
	}

	t.Log("GeoIPLocation 结构体测试通过")
}

// TestSNISelectorRefreshFromGeoIP 测试从 GeoIP 刷新域名列表
func TestSNISelectorRefreshFromGeoIP(t *testing.T) {
	// 创建测试用的 ServerService
	serverService := &ServerService{}
	serverService.geoIPService = NewGeoIPService()

	if serverService.geoIPService == nil {
		t.Fatal("GeoIP 服务创建失败")
	}

	// 创建 SNI 选择器
	initialDomains := []string{"www.google.com:443"}
	sniSelector := NewSNISelectorWithGeoIP(initialDomains, serverService.geoIPService)

	if sniSelector == nil {
		t.Fatal("SNI 选择器创建失败")
	}

	// 测试刷新功能（这可能会因为文件不存在而失败，但至少测试逻辑）
	t.Log("开始测试域名刷新功能...")
	sniSelector.RefreshDomainsFromGeoIP(serverService)

	// 验证 SNI 选择器仍然可以正常工作
	sni := sniSelector.Next()
	if sni == "" {
		t.Error("刷新后获取 SNI 失败")
	} else {
		t.Logf("刷新后成功获取 SNI: %s", sni)
	}
}

// TestGeoIPServiceMainAPIFailover 测试主 API 失败时切换到备用 API
func TestGeoIPServiceMainAPIFailover(t *testing.T) {
	// 创建模拟服务器
	mainAPICallCount := 0

	// 创建统一的模拟服务器，根据 URL 路径返回不同的响应
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mainAPICallCount++
		w.Header().Set("Content-Type", "application/json")

		if r.URL.Path == "/en?json" {
			// 模拟主 API 第一次失败，第二次成功（但我们在测试中只调用一次）
			if mainAPICallCount == 1 {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("Server Error"))
				return
			}
		}

		if r.URL.Path == "/geoip" {
			// 备用 API 返回成功
			ipsbLocation := IPSBLocation{
				IP:           "5.6.7.8",
				CountryCode:  "BS",
				CountryName:  "Backup Country",
				Region:       "BR",
				RegionName:   "Backup Region",
				City:         "Backup City",
				Latitude:     98.76,
				Longitude:    54.32,
				Timezone:     "UTC",
				ISP:          "Backup ISP",
				Organization: "Backup Org",
			}
			json.NewEncoder(w).Encode(ipsbLocation)
			return
		}

		// 默认情况下主 API 失败
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Server Error"))
	}))
	defer mockServer.Close()

	// 创建自定义 HTTP 客户端，将请求路由到模拟服务器
	client := &http.Client{
		Transport: &mockTransport{
			server: mockServer,
		},
	}

	geoIPService := NewGeoIPServiceWithClient(client)

	// 测试主 API 失败，备用 API 成功的场景
	location, err := geoIPService.FetchLocation()
	if err != nil {
		t.Errorf("备用 API 应该成功，但返回错误: %v", err)
	}

	if location == nil {
		t.Error("位置信息不应为空")
	}

	if location.GetCountryCode() != "BS" {
		t.Errorf("国家代码应该是 BS，但得到: %s", location.GetCountryCode())
	}

	t.Logf("主备 API 切换测试成功 - 国家: %s, 代码: %s, IP: %s",
		location.GetCountry(), location.GetCountryCode(), location.IP)
}

// TestGeoIPServiceBothAPIFail 测试两个 API 都失败的情况
func TestGeoIPServiceBothAPIFail(t *testing.T) {
	// 创建模拟服务器，两个 API 都失败
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 所有请求都返回错误
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Server Error"))
	}))
	defer mockServer.Close()

	// 创建自定义 HTTP 客户端，将请求路由到模拟服务器
	client := &http.Client{
		Transport: &mockTransport{
			server: mockServer,
		},
	}

	geoIPService := NewGeoIPServiceWithClient(client)

	// 测试两个 API 都失败的情况
	location, err := geoIPService.FetchLocation()
	if err == nil {
		t.Error("两个 API 都失败时应该返回错误")
	}

	if location != nil {
		t.Error("两个 API 都失败时位置信息应该为空")
	}

	t.Logf("两个 API 失败测试成功 - 错误: %v", err)
}

// TestIPSBLocationStruct 测试 IPSBLocation 结构体
func TestIPSBLocationStruct(t *testing.T) {
	// 创建模拟的 IPSBLocation 响应数据
	mockIPSBLocation := IPSBLocation{
		IP:           "192.168.1.1",
		CountryCode:  "CN",
		CountryName:  "China",
		Region:       "BJ",
		RegionName:   "Beijing",
		City:         "Beijing",
		Latitude:     39.9042,
		Longitude:    116.4074,
		Timezone:     "Asia/Shanghai",
		ISP:          "China Telecom",
		Organization: "China Telecom",
	}

	// 验证结构体字段
	if mockIPSBLocation.IP != "192.168.1.1" {
		t.Error("IP 字段设置不正确")
	}

	if mockIPSBLocation.CountryCode != "CN" {
		t.Error("CountryCode 字段设置不正确")
	}

	if mockIPSBLocation.CountryName != "China" {
		t.Error("CountryName 字段设置不正确")
	}

	if mockIPSBLocation.Latitude != 39.9042 {
		t.Error("Latitude 字段设置不正确")
	}

	t.Log("IPSBLocation 结构体测试通过")
}

// TestConvertIPSBToGeoIP 测试 IPSB 到 GeoIP 的转换
func TestConvertIPSBToGeoIP(t *testing.T) {
	// 创建测试用的 IPSBLocation
	ipsbLocation := IPSBLocation{
		IP:           "1.2.3.4",
		CountryCode:  "US",
		CountryName:  "United States",
		Region:       "CA",
		RegionName:   "California",
		City:         "San Francisco",
		Latitude:     37.7749,
		Longitude:    -122.4194,
		Timezone:     "America/Los_Angeles",
		ISP:          "Test ISP",
		Organization: "Test Org",
	}

	// 执行转换
	geoIPLocation := convertIPSBToGeoIP(&ipsbLocation)

	// 验证转换结果
	if geoIPLocation == nil {
		t.Error("转换结果不应为空")
	}

	if geoIPLocation.IP != "1.2.3.4" {
		t.Error("IP 字段转换不正确")
	}

	if geoIPLocation.Location.CountryCode != "US" {
		t.Error("CountryCode 字段转换不正确")
	}

	if geoIPLocation.Location.CountryName != "United States" {
		t.Error("CountryName 字段转换不正确")
	}

	if geoIPLocation.Location.City != "San Francisco" {
		t.Error("City 字段转换不正确")
	}

	if geoIPLocation.Location.Province != "California" {
		t.Error("Province 字段转换不正确")
	}

	// 验证经纬度转换（float64 转 string）
	expectedLat := "37.7749"
	expectedLon := "-122.4194"
	if geoIPLocation.Location.Latitude != expectedLat {
		t.Errorf("Latitude 转换不正确，期望: %s, 实际: %s", expectedLat, geoIPLocation.Location.Latitude)
	}

	if geoIPLocation.Location.Longitude != expectedLon {
		t.Errorf("Longitude 转换不正确，期望: %s, 实际: %s", expectedLon, geoIPLocation.Location.Longitude)
	}

	t.Log("IPSB 到 GeoIP 转换测试通过")
}

// BenchmarkGeoIPServiceFetch 基准测试 GeoIP 服务性能
func BenchmarkGeoIPServiceFetch(b *testing.B) {
	geoIPService := NewGeoIPService()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := geoIPService.FetchLocation()
		if err != nil {
			b.Fatalf("GeoIP 获取失败: %v", err)
		}
	}
}
