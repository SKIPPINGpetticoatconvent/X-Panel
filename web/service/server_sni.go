package service

import (
	"fmt"
	"os"
	"strings"

	"x-ui/logger"
)

// SNI域名管理模块
// 负责SNI域名选择、加载、标准化等核心功能

// readSNIDomainsFromFile 通用函数：从指定国家的SNI文件读取域名列表
func (s *ServerService) readSNIDomainsFromFile(countryCode string) ([]string, error) {
	filePath := fmt.Sprintf("sni/%s/sni_domains.txt", countryCode)
	
	// 读取SNI域名文件
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("读取SNI文件 %s 失败: %w", filePath, err)
	}

	lines := strings.Split(string(data), "\n")
	var domains []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		// 跳过空行和注释行
		if line == "" || strings.HasPrefix(line, "//") || strings.HasPrefix(line, "#") {
			continue
		}

		// 清理JSON数组格式的引号和逗号
		// 先清理首尾的引号
		for strings.HasPrefix(line, `"`) {
			line = strings.TrimPrefix(line, `"`)
		}
		for strings.HasSuffix(line, `"`) {
			line = strings.TrimSuffix(line, `"`)
		}
		// 再清理首尾的逗号
		for strings.HasPrefix(line, `,`) {
			line = strings.TrimPrefix(line, `,`)
		}
		for strings.HasSuffix(line, `,`) {
			line = strings.TrimSuffix(line, `,`)
		}
		line = strings.TrimSpace(line)

		if line != "" {
			// 确保格式正确
			if !strings.Contains(line, ":") {
				line += ":443"
			}
			domains = append(domains, line)
		}
	}

	if len(domains) == 0 {
		return nil, fmt.Errorf("SNI文件 %s 中没有有效域名", filePath)
	}

	logger.Infof("从 %到 %d 个s 文件成功读取SNI域名", filePath, len(domains))
	return domains, nil
}

// 【重构方法】: 获取指定国家的SNI域名列表（优先从文件读取）
func (s *ServerService) GetCountrySNIDomains(countryCode string) []string {
	// 将国家代码转换为大写
	countryCode = strings.ToUpper(countryCode)

	// 首先尝试从文件读取SNI域名列表
	domains, err := s.readSNIDomainsFromFile(countryCode)
	if err == nil {
		logger.Infof("成功从文件读取 %s SNI域名列表，共 %d 个域名", countryCode, len(domains))
		return s.removeDuplicatesFromSlice(domains)
	}

	// 文件读取失败，记录警告并使用默认列表
	logger.Warningf("从文件读取 %s SNI域名失败: %v，使用默认域名列表", countryCode, err)

	// 获取默认域名列表（简化版本）
	defaultDomains := s.getDefaultSNIDomains(countryCode)
	if len(defaultDomains) > 0 {
		logger.Infof("使用 %s 的默认SNI域名列表，共 %d 个域名", countryCode, len(defaultDomains))
		return defaultDomains
	}

	// 如果默认列表也为空，使用国际通用域名
	logger.Warningf("%s 没有默认域名列表，使用国际通用域名", countryCode)
	return s.getDefaultSNIDomains("DEFAULT")
}

// normalizeDomain 标准化域名格式（转小写、去空格）
func (s *ServerService) normalizeDomain(domain string) string {
	// 去除首尾空格
	domain = strings.TrimSpace(domain)
	// 转换为小写以确保大小写不敏感的域名比较
	return strings.ToLower(domain)
}

// removeDuplicatesFromSlice 从字符串切片中移除重复元素（增强版）
func (s *ServerService) removeDuplicatesFromSlice(slice []string) []string {
	seen := make(map[string]bool)
	var result []string

	for _, item := range slice {
		// 标准化域名格式
		normalizedItem := s.normalizeDomain(item)

		if !seen[normalizedItem] {
			seen[normalizedItem] = true
			result = append(result, item) // 保留原始格式
		}
	}

	return result
}

// getDefaultSNIDomains 获取默认的SNI域名列表（最小化硬编码）
func (s *ServerService) getDefaultSNIDomains(countryCode string) []string {
	// 【重构】: 最小化硬编码，只保留最基本的回退域名
	// 推荐使用 sni/{CountryCode}/sni_domains.txt 文件来配置域名
	switch countryCode {
	case "US":
		// 美国 - 最小化默认列表
		return []string{
			"www.microsoft.com:443",
			"www.amazon.com:443",
			"www.google.com:443",
		}

	case "JP":
		// 日本 - 最小化默认列表
		return []string{
			"www.amazon.co.jp:443",
			"www.rakuten.co.jp:443",
			"www.yahoo.co.jp:443",
		}
	case "UK", "GB":
		// 英国 - 最小化默认列表
		return []string{
			"www.bbc.com:443",
			"www.theguardian.com:443",
			"www.gov.uk:443",
		}
	case "KR":
		// 韩国 - 最小化默认列表
		return []string{
			"www.naver.com:443",
			"www.daum.net:443",
			"www.amazon.co.kr:443",
		}
	case "DE":
		// 德国 - 最小化默认列表
		return []string{
			"www.amazon.de:443",
			"www.google.de:443",
			"www.bundesregierung.de:443",
		}
	default:
		// 默认返回国际通用域名（最小化）
		return []string{
			"www.google.com:443",
			"www.amazon.com:443",
			"www.apple.com:443",
		}
	}
}

// 初始化 SNI 选择器
func (s *ServerService) initSNISelector() {
	// 初始化 GeoIP 服务
	if s.geoIPService == nil {
		s.geoIPService = NewGeoIPService()
		logger.Info("GeoIP service initialized in ServerService")
	}

	// 获取服务器地理位置
	countryCode := s.geoIPService.GetCountryCode()
	logger.Infof("检测到服务器地理位置: %s", countryCode)

	// 获取对应国家的 SNI 域名列表
	domains := s.GetCountrySNIDomains(countryCode)
	s.sniSelector = NewSNISelectorWithGeoIP(domains, s.geoIPService)
	logger.Infof("SNI selector initialized with %s domains (%d domains)", countryCode, len(domains))
}

// GetNewSNI 获取下一个不重复的 SNI 域名
func (s *ServerService) GetNewSNI() string {
	if s.sniSelector == nil {
		logger.Warning("SNI selector not initialized, initializing now")
		s.initSNISelector()
	}
	return s.sniSelector.Next()
}

// RefreshSNIFromGeoIP 根据地理位置刷新 SNI 域名列表
func (s *ServerService) RefreshSNIFromGeoIP() {
	if s.sniSelector == nil {
		logger.Warning("SNI selector not initialized, cannot refresh")
		return
	}

	// 使用 SNISelector 的刷新方法
	s.sniSelector.RefreshDomainsFromGeoIP(s)
	logger.Info("SNI域名列表已根据地理位置刷新")
}