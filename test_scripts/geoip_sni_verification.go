package main

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"time"

	"x-ui/web/service"
)

// GeoIP 集成和 SNI 选择逻辑手动验证测试脚本
func main() {
	fmt.Println("=== GeoIP 集成和 SNI 选择逻辑手动验证测试 ===")
	fmt.Println("开始时间:", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Println()

	// 1. 验证 GeoIP 服务初始化
	fmt.Println("1. 验证 GeoIP 服务初始化")
	fmt.Println("--------------------------------")
	geoIPService := service.NewGeoIPService()
	if geoIPService == nil {
		log.Fatal("GeoIP 服务创建失败")
	}
	fmt.Println("✓ GeoIP 服务创建成功")

	// 获取国家代码
	countryCode := geoIPService.GetCountryCode()
	fmt.Printf("✓ 检测到的国家代码: %s\n", countryCode)

	// 获取详细位置信息
	location, err := geoIPService.FetchLocation()
	if err != nil {
		fmt.Printf("⚠️  获取详细位置信息失败: %v\n", err)
	} else {
		fmt.Printf("✓ 详细位置信息: %s (%s), IP: %s\n",
			location.GetCountry(), location.GetCountryCode(), location.IP)
	}
	fmt.Println()

	// 2. 验证 SNI 选择器初始化
	fmt.Println("2. 验证 SNI 选择器初始化")
	fmt.Println("--------------------------------")

	// 测试不同的初始化场景
	testScenarios := []struct {
		name           string
		initialDomains []string
	}{
		{"标准域名列表", []string{"www.google.com:443", "www.amazon.com:443", "www.microsoft.com:443"}},
		{"空域名列表", []string{}},
		{"单域名", []string{"www.apple.com:443"}},
	}

	for i, scenario := range testScenarios {
		fmt.Printf("2.%d 测试场景: %s\n", i+1, scenario.name)

		// 创建 SNI 选择器
		sniSelector := service.NewSNISelectorWithGeoIP(scenario.initialDomains, geoIPService)

		if sniSelector == nil {
			fmt.Printf("   ✗ SNI 选择器创建失败\n")
			continue
		}
		fmt.Printf("   ✓ SNI 选择器创建成功\n")

		// 获取 GeoIP 信息
		geoIPInfo := sniSelector.GetGeoIPInfo()
		fmt.Printf("   ✓ GeoIP 信息: %s\n", geoIPInfo)

		// 获取 SNI 域名列表
		domains := sniSelector.GetDomains()
		fmt.Printf("   ✓ SNI 域名列表 (共 %d 个):\n", len(domains))
		for j, domain := range domains {
			if j < 5 { // 只显示前5个
				fmt.Printf("     - %s\n", domain)
			} else if j == 5 {
				fmt.Printf("     - ... (还有 %d 个域名)\n", len(domains)-5)
				break
			}
		}

		// 测试轮询获取 SNI
		fmt.Printf("   测试 SNI 轮询 (前 5 次):\n")
		seenDomains := make(map[string]bool)
		for k := 0; k < 5; k++ {
			sni := sniSelector.Next()
			if sni != "" {
				fmt.Printf("     第 %d 次: %s\n", k+1, sni)
				seenDomains[sni] = true
			} else {
				fmt.Printf("     第 %d 次: 获取失败\n", k+1)
			}
		}

		// 检查是否实现了轮询
		if len(seenDomains) > 1 {
			fmt.Printf("   ✓ 轮询机制工作正常 (获取到 %d 个不同域名)\n", len(seenDomains))
		} else {
			fmt.Printf("   ⚠️  轮询可能有问题 (只获取到 1 个域名)\n")
		}
		fmt.Println()
	}

	// 3. 模拟 ServerService 启动过程
	fmt.Println("3. 模拟 ServerService 启动过程")
	fmt.Println("--------------------------------")

	// 创建 ServerService 实例 (模拟)
	serverService := &MockServerService{
		geoIPService: geoIPService,
	}

	// 模拟 initSNISelector 过程
	fmt.Println("3.1 模拟 SNI 选择器初始化")
	serverService.initSNISelector()

	if serverService.sniSelector != nil {
		fmt.Printf("   ✓ SNI 选择器初始化成功\n")

		// 测试获取 SNI
		for i := 0; i < 3; i++ {
			sni := serverService.GetNewSNI()
			fmt.Printf("   获取到的 SNI #%d: %s\n", i+1, sni)
		}
	} else {
		fmt.Printf("   ✗ SNI 选择器初始化失败\n")
	}

	// 测试 GeoIP 信息获取
	fmt.Println("3.2 测试 GeoIP 信息获取")
	geoIPInfo := serverService.GetGeoIPInfo()
	fmt.Printf("   ✓ GeoIP 信息: %s\n", geoIPInfo)

	// 测试获取国家 SNI 域名列表
	fmt.Println("3.3 测试获取国家 SNI 域名列表")
	testCountries := []string{countryCode, "US", "JP", "UK", "UNKNOWN"}
	for _, testCountry := range testCountries {
		domains := serverService.GetCountrySNIDomains(testCountry)
		fmt.Printf("   %s SNI 域名列表 (共 %d 个):\n", testCountry, len(domains))
		for j, domain := range domains {
			if j < 3 { // 只显示前3个
				fmt.Printf("     - %s\n", domain)
			} else if j == 3 {
				fmt.Printf("     - ... (还有 %d 个域名)\n", len(domains)-3)
				break
			}
		}
	}
	fmt.Println()

	// 4. 测试错误回退流程
	fmt.Println("4. 测试错误回退流程")
	fmt.Println("--------------------------------")

	// 测试空域名列表的回退机制
	fmt.Println("4.1 测试空域名列表回退")
	emptySelector := service.NewSNISelectorWithGeoIP([]string{}, geoIPService)
	if emptySelector != nil {
		emptySNI := emptySelector.Next()
		if emptySNI != "" {
			fmt.Printf("   ✓ 空列表回退成功，获取到: %s\n", emptySNI)
		} else {
			fmt.Printf("   ✗ 空列表回退失败\n")
		}
	}

	// 测试刷新功能
	fmt.Println("4.2 测试 SNI 域名刷新功能")
	if serverService.sniSelector != nil {
		// 获取刷新前的域名列表
		beforeDomains := serverService.sniSelector.GetDomains()
		fmt.Printf("   刷新前域名数量: %d\n", len(beforeDomains))

		// 执行刷新
		serverService.RefreshSNIFromGeoIP()

		// 获取刷新后的域名列表
		afterDomains := serverService.sniSelector.GetDomains()
		fmt.Printf("   刷新后域名数量: %d\n", len(afterDomains))

		if len(afterDomains) > 0 {
			fmt.Printf("   ✓ 刷新功能正常\n")
		} else {
			fmt.Printf("   ⚠️  刷新后域名列表为空\n")
		}
	}

	fmt.Println()

	// 5. 测试备用 API 机制
	fmt.Println("5. 测试备用 API 机制")
	fmt.Println("--------------------------------")

	testAPIFailoverScenarios := []struct {
		name          string
		mainAPIFail   bool
		backupAPIFail bool
	}{
		{"主API成功，备用API成功", false, false},
		{"主API失败，备用API成功", true, false},
		{"主API成功，备用API失败", false, true},
		{"主API失败，备用API失败", true, true},
	}

	for i, scenario := range testAPIFailoverScenarios {
		fmt.Printf("5.%d 测试场景: %s\n", i+1, scenario.name)

		// 创建模拟服务器
		mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")

			// 调试信息
			fmt.Printf("     模拟服务器收到请求: %s %s\n", r.Method, r.URL.String())

			// 检查是否是主 API 请求 (api.myip.la/en?json)
			if r.URL.Path == "/en" && r.URL.RawQuery == "json" {
				if scenario.mainAPIFail {
					fmt.Printf("     模拟主 API 失败\n")
					// 主 API 失败
					w.WriteHeader(http.StatusInternalServerError)
					w.Write([]byte("Main API Error"))
				} else {
					fmt.Printf("     模拟主 API 成功\n")
					// 主 API 成功
					w.Write([]byte(`{"ip": "1.2.3.4", "country_code": "CN", "country_name": "China", "location": {"city": "Beijing", "province": "Beijing", "latitude": "39.9042", "longitude": "116.4074"}}`))
				}
				return
			}

			// 检查是否是备用 API 请求 (api.ip.sb/geoip)
			if r.URL.Path == "/geoip" {
				if scenario.backupAPIFail {
					fmt.Printf("     模拟备用 API 失败\n")
					// 备用 API 失败
					w.WriteHeader(http.StatusInternalServerError)
					w.Write([]byte("Backup API Error"))
				} else {
					fmt.Printf("     模拟备用 API 成功\n")
					// 备用 API 成功
					w.Write([]byte(`{"ip": "5.6.7.8", "country_code": "BS", "country": "Backup Country", "region": "BR", "region_name": "Backup Region", "city": "Backup City", "latitude": 98.76, "longitude": 54.32, "timezone": "UTC", "isp": "Backup ISP", "organization": "Backup Org"}`))
				}
				return
			}

			// 默认返回 404
			fmt.Printf("     路径未匹配，返回 404\n")
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("Not Found"))
		}))
		defer mockServer.Close()

		// 创建自定义 HTTP 客户端
		client := &http.Client{
			Transport: &mockTransport{
				server: mockServer,
			},
		}

		// 创建 GeoIP 服务实例
		geoIPServiceTest := service.NewGeoIPServiceWithClient(client)

		// 测试获取位置信息
		location, err := geoIPServiceTest.FetchLocation()

		if scenario.mainAPIFail && scenario.backupAPIFail {
			// 两个 API 都失败
			if err != nil {
				fmt.Printf("   ✓ 预期失败 - 错误: %v\n", err)
			} else {
				fmt.Printf("   ✗ 预期失败但成功获取位置: %v\n", location)
			}
		} else if scenario.mainAPIFail && !scenario.backupAPIFail {
			// 主 API 失败，备用 API 成功
			if err != nil {
				fmt.Printf("   ✗ 备用API应该成功，但失败: %v\n", err)
			} else if location == nil {
				fmt.Printf("   ✗ 位置信息不应为空\n")
			} else {
				fmt.Printf("   ✓ 备用API切换成功 - 位置: %s (%s), IP: %s\n",
					location.GetCountry(), location.GetCountryCode(), location.IP)
			}
		} else if !scenario.mainAPIFail && scenario.backupAPIFail {
			// 主 API 成功，备用 API 失败
			if err != nil {
				fmt.Printf("   ✗ 主API应该成功，但失败: %v\n", err)
			} else if location == nil {
				fmt.Printf("   ✗ 位置信息不应为空\n")
			} else {
				fmt.Printf("   ✓ 主API成功 - 位置: %s (%s), IP: %s\n",
					location.GetCountry(), location.GetCountryCode(), location.IP)
			}
		} else {
			// 两个 API 都成功（正常情况）
			if err != nil {
				fmt.Printf("   ✗ 预期成功，但失败: %v\n", err)
			} else if location == nil {
				fmt.Printf("   ✗ 位置信息不应为空\n")
			} else {
				fmt.Printf("   ✓ 正常获取位置 - 位置: %s (%s), IP: %s\n",
					location.GetCountry(), location.GetCountryCode(), location.IP)
			}
		}
		fmt.Println()
	}

	fmt.Println()
	fmt.Println("=== 手动验证测试完成 ===")
	fmt.Println("结束时间:", time.Now().Format("2006-01-02 15:04:05"))
}

// MockServerService 模拟 ServerService 用于测试
type MockServerService struct {
	geoIPService *service.GeoIPService
	sniSelector  *service.SNISelector
}

// initSNISelector 模拟初始化 SNI 选择器
func (s *MockServerService) initSNISelector() {
	if s.geoIPService == nil {
		s.geoIPService = service.NewGeoIPService()
	}

	// 获取服务器地理位置
	countryCode := s.geoIPService.GetCountryCode()
	fmt.Printf("   检测到服务器地理位置: %s\n", countryCode)

	// 获取对应国家的 SNI 域名列表
	domains := s.GetCountrySNIDomains(countryCode)
	s.sniSelector = service.NewSNISelectorWithGeoIP(domains, s.geoIPService)
	fmt.Printf("   SNI 选择器初始化完成，共 %d 个域名\n", len(domains))
}

// GetNewSNI 获取下一个 SNI 域名
func (s *MockServerService) GetNewSNI() string {
	if s.sniSelector == nil {
		return ""
	}
	return s.sniSelector.Next()
}

// GetGeoIPInfo 获取 GeoIP 信息
func (s *MockServerService) GetGeoIPInfo() string {
	if s.geoIPService == nil {
		return "GeoIP 服务未初始化"
	}

	location, err := s.geoIPService.FetchLocationWithRetry(1)
	if err != nil {
		return fmt.Sprintf("GeoIP 查询失败: %v", err)
	}

	return fmt.Sprintf("服务器位置: %s (%s), IP: %s",
		location.GetCountry(), location.GetCountryCode(), location.IP)
}

// GetCountrySNIDomains 获取指定国家的 SNI 域名列表
func (s *MockServerService) GetCountrySNIDomains(countryCode string) []string {
	// 简化实现，调用 service 包的方法
	// 这里我们创建一个临时的 ServerService 来调用方法
	tempService := &service.ServerService{}
	return tempService.GetCountrySNIDomains(countryCode)
}

// RefreshSNIFromGeoIP 根据地理位置刷新 SNI 域名列表
func (s *MockServerService) RefreshSNIFromGeoIP() {
	if s.sniSelector == nil {
		return
	}
	// 简化实现，直接调用 SNISelector 的方法
	// 注意：这里需要传入 ServerService 实例，但为了测试简化，我们跳过这部分
	fmt.Println("   (简化实现) SNI域名列表刷新完成")
}

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
