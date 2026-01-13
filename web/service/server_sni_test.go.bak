package service

import (
	"strings"
	"testing"
)

// TestServerService_GetNewSNI 测试 ServerService 的 GetNewSNI 方法
func TestServerService_GetNewSNI(t *testing.T) {
	// 创建 ServerService 实例
	serverService := &ServerService{}

	// 初始化 SNI 选择器（使用默认域名）
	serverService.initSNISelector()

	// 测试获取 SNI
	sni := serverService.GetNewSNI()

	// 验证返回的 SNI 不为空
	if sni == "" {
		t.Error("GetNewSNI() 返回空字符串")
	}

	// 验证返回的 SNI 包含端口号
	if !strings.Contains(sni, ":") {
		t.Errorf("SNI 应该包含端口号，格式为 domain:port，但得到: %s", sni)
	}

	// 验证多次调用不会返回空值
	for i := 0; i < 10; i++ {
		sni := serverService.GetNewSNI()
		if sni == "" {
			t.Errorf("第 %d 次调用 GetNewSNI() 返回空字符串", i+1)
		}
	}
}

// TestServerService_GetCountrySNIDomains 测试按国家获取 SNI 域名列表
func TestServerService_GetCountrySNIDomains(t *testing.T) {
	serverService := &ServerService{}

	// 测试获取美国域名列表
	usDomains := serverService.GetCountrySNIDomains("US")
	if len(usDomains) == 0 {
		t.Error("获取美国 SNI 域名列表失败")
	}

	// 验证域名格式
	for _, domain := range usDomains {
		if !strings.Contains(domain, ":") {
			t.Errorf("域名应该包含端口号，格式为 domain:port，但得到: %s", domain)
		}
	}

	// 测试获取中国域名列表
	cnDomains := serverService.GetCountrySNIDomains("CN")
	if len(cnDomains) == 0 {
		t.Error("获取中国 SNI 域名列表失败")
	}

	// 测试获取日本域名列表
	jpDomains := serverService.GetCountrySNIDomains("JP")
	if len(jpDomains) == 0 {
		t.Error("获取日本 SNI 域名列表失败")
	}

	// 测试未知国家代码（应该返回默认域名）
	unknownDomains := serverService.GetCountrySNIDomains("UNKNOWN")
	if len(unknownDomains) == 0 {
		t.Error("未知国家应该返回默认域名列表")
	}
}

// TestServerService_initSNISelector 测试 SNI 选择器初始化
func TestServerService_initSNISelector(t *testing.T) {
	serverService := &ServerService{}

	// 初始化 SNI 选择器
	serverService.initSNISelector()

	// 验证 sniSelector 不为空
	if serverService.sniSelector == nil {
		t.Error("SNI 选择器初始化失败")
	}

	// 验证可以获取域名
	domains := serverService.sniSelector.GetDomains()
	if len(domains) == 0 {
		t.Error("SNI 选择器没有域名")
	}
}

// TestServerService_readSNIDomainsFromFile 测试从文件读取SNI域名
func TestServerService_readSNIDomainsFromFile(t *testing.T) {
	serverService := &ServerService{}

	// 测试从US文件读取域名（在测试环境中可能不存在文件）
	domains, err := serverService.readSNIDomainsFromFile("US")

	// 检查是否有错误（文件可能不存在）
	if err != nil {
		// 在测试环境中文件可能不存在，这是预期的
		if strings.Contains(err.Error(), "no such file or directory") || strings.Contains(err.Error(), "cannot find the file") {
			t.Logf("预期错误：SNI文件不存在: %v", err)
		} else {
			t.Errorf("读取US SNI文件时出现意外错误: %v", err)
		}
	} else {
		// 如果文件存在，验证域名格式
		if len(domains) == 0 {
			t.Error("从US文件读取到空域名列表")
		}

		// 验证域名格式
		for _, domain := range domains {
			if !strings.Contains(domain, ":") {
				t.Errorf("域名应该包含端口号，格式为 domain:port，但得到: %s", domain)
			}
			// 验证域名不为空
			parts := strings.Split(domain, ":")
			if len(parts) != 2 || parts[0] == "" {
				t.Errorf("域名格式无效: %s", domain)
			}
		}
	}

	// 测试读取不存在的文件（应该返回错误）
	_, err = serverService.readSNIDomainsFromFile("NONEXISTENT")
	if err == nil {
		t.Error("读取不存在的文件应该返回错误")
	}

	// 验证错误消息包含文件路径
	expectedPath := "sni/NONEXISTENT/sni_domains.txt"
	if !strings.Contains(err.Error(), expectedPath) {
		t.Errorf("错误消息应该包含文件路径 %s，但实际错误: %v", expectedPath, err)
	}
}

// TestServerService_SNI_Integration 测试 SNI 集成功能
func TestServerService_SNI_Integration(t *testing.T) {
	serverService := &ServerService{}

	// 初始化
	serverService.initSNISelector()

	// 测试连续调用 SNI 选择器
	var snis []string
	for i := 0; i < 20; i++ {
		sni := serverService.GetNewSNI()
		if sni == "" {
			t.Fatalf("第 %d 次调用 GetNewSNI() 返回空字符串", i+1)
		}
		snis = append(snis, sni)
	}

	// 验证没有返回空值
	for i, sni := range snis {
		if sni == "" {
			t.Errorf("第 %d 个 SNI 为空", i+1)
		}
	}

	// 验证域名多样性（至少有一些不同的域名）
	uniqueDomains := make(map[string]bool)
	for _, sni := range snis {
		domain := strings.Split(sni, ":")[0]
		uniqueDomains[domain] = true
	}

	if len(uniqueDomains) < 2 {
		t.Errorf("SNI 选择器应该提供域名多样性，但只得到 %d 个唯一域名", len(uniqueDomains))
	}
}
