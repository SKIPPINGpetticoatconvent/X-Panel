package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"x-ui/logger"
)

// SNI域名管理模块
// 负责SNI域名选择、加载、标准化等核心功能

// readSNIDomainsFromFile 通用函数：从指定国家的SNI文件读取域名列表
func (s *ServerService) readSNIDomainsFromFile(countryCode string) ([]string, error) {
	// 修复文件路径问题：使用绝对路径适配工作目录
	// 获取程序执行目录或使用绝对路径
	execPath, err := os.Executable()
	if err != nil {
		logger.Warningf("无法获取程序执行路径: %v，将使用相对路径", err)
	}
	
	if execPath != "" {
		// 使用程序所在目录作为基准路径
		execDir := filepath.Dir(execPath)
		filePath := filepath.Join(execDir, "sni", countryCode, "sni_domains.txt")
		logger.Infof("使用绝对路径读取 SNI 文件: %s", filePath)
		
		// 读取SNI域名文件
		data, err := os.ReadFile(filePath)
		if err == nil {
			logger.Infof("成功从绝对路径读取 SNI 文件: %s", filePath)
			// 处理文件内容
			lines := strings.Split(string(data), "\n")
			var domains []string

			for _, line := range lines {
				line = strings.TrimSpace(line)
				// 跳过空行和注释行
				if line == "" || strings.HasPrefix(line, "//") || strings.HasPrefix(line, "#") {
					continue
				}

				// 【修复】清理JSON数组格式的引号和逗号（增强版）
				// 先清理首尾的引号（多次循环确保清理干净）
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
				// 【新增】清理可能的转义引号和其他特殊字符
				line = strings.ReplaceAll(line, `\"`, `"`)  // 清理转义引号
				line = strings.ReplaceAll(line, `""`, `"`) // 清理双引号
				line = strings.TrimSpace(line)
				// 【修复】最终验证：确保没有多余引号
				if strings.HasPrefix(line, `"`) || strings.HasSuffix(line, `"`) {
					logger.Warningf("域名清理后仍包含引号，将跳过此行: %s", line)
					continue
				}

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

			logger.Infof("从文件 %s 成功读取SNI域名，共 %d 个", filePath, len(domains))
			return domains, nil
		}
		
		logger.Warningf("从绝对路径读取 SNI 文件失败: %v，尝试相对路径", err)
	}
	
	// 回退到相对路径（用于测试环境或特殊部署）
	filePath := fmt.Sprintf("sni/%s/sni_domains.txt", countryCode)
	logger.Infof("尝试使用相对路径读取 SNI 文件: %s", filePath)
	
	// 读取SNI域名文件
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("读取SNI文件 %s 失败: %w", filePath, err)
	}
	
	logger.Infof("成功从相对路径读取 SNI 文件: %s", filePath)

	lines := strings.Split(string(data), "\n")
	var domains []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		// 跳过空行和注释行
		if line == "" || strings.HasPrefix(line, "//") || strings.HasPrefix(line, "#") {
			continue
		}

		// 【修复】清理JSON数组格式的引号和逗号（增强版）
		// 先清理首尾的引号（多次循环确保清理干净）
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
		// 【新增】清理可能的转义引号和其他特殊字符
		line = strings.ReplaceAll(line, `\"`, `"`)  // 清理转义引号
		line = strings.ReplaceAll(line, `""`, `"`) // 清理双引号
		line = strings.TrimSpace(line)
		// 【修复】最终验证：确保没有多余引号
		if strings.HasPrefix(line, `"`) || strings.HasSuffix(line, `"`) {
			logger.Warningf("域名清理后仍包含引号，将跳过此行: %s", line)
			continue
		}

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

	logger.Infof("从文件 %s 成功读取SNI域名，共 %d 个", filePath, len(domains))
	return domains, nil
}

// 【新增辅助函数】获取工作目录用于调试
func (s *ServerService) getWorkingDirectoryInfo() {
	// 获取当前工作目录
	workDir, err := os.Getwd()
	if err != nil {
		logger.Warningf("无法获取当前工作目录: %v", err)
		return
	}
	logger.Infof("当前工作目录: %s", workDir)
	
	// 获取程序执行目录
	execPath, err := os.Executable()
	if err != nil {
		logger.Warningf("无法获取程序执行路径: %v", err)
		return
	}
	
	execDir := filepath.Dir(execPath)
	logger.Infof("程序执行目录: %s", execDir)
	logger.Infof("程序路径: %s", execPath)
}

// 【重构方法】: 获取指定国家的SNI域名列表（优先从文件读取）
func (s *ServerService) GetCountrySNIDomains(countryCode string) []string {
	// 将国家代码转换为大写
	countryCode = strings.ToUpper(countryCode)
	
	// 【新增】输出调试信息
	logger.Infof("=== SNI域名读取调试开始 ===")
	logger.Infof("请求获取 %s 的SNI域名列表", countryCode)
	
	// 获取工作目录信息用于调试
	s.getWorkingDirectoryInfo()

	// 首先尝试从文件读取SNI域名列表
	domains, err := s.readSNIDomainsFromFile(countryCode)
	if err == nil {
		logger.Infof("✅ 成功从文件读取 %s SNI域名列表，共 %d 个域名", countryCode, len(domains))
		// 显示前5个域名示例
		maxShow := 5
		if len(domains) < maxShow {
			maxShow = len(domains)
		}
		logger.Infof("前%d个域名示例: %v", maxShow, domains[:maxShow])
		logger.Infof("=== SNI域名读取调试结束 ===")
		return s.removeDuplicatesFromSlice(domains)
	}

	// 文件读取失败，记录警告并使用默认列表
	logger.Warningf("❌ 从文件读取 %s SNI域名失败: %v，使用默认域名列表", countryCode, err)
	logger.Infof("=== SNI域名读取调试结束 ===")

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

	// 获取对应国家的SNI域名列表
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
	
	selectedSNI := s.sniSelector.Next()
	logger.Infof("🎯 选中SNI域名: %s", selectedSNI)
	return selectedSNI
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

// 【修复】GenerateEnhancedServerNames 根据传入的域名生成增强的 ServerNames 列表
// 这个方法从 Tgbot 迁移过来，确保后端和 TG Bot 使用相同的逻辑（修复基础域名缺失问题）
func (s *ServerService) GenerateEnhancedServerNames(domain string) []string {
	// 为指定的域名生成多个常见的子域名变体
	var serverNames []string

	// 添加主域名
	serverNames = append(serverNames, domain)

	// 【修复】提取基础域名，避免在www.前缀前添加子域名
	baseDomain := domain
	if strings.HasPrefix(domain, "www.") {
		baseDomain = strings.TrimPrefix(domain, "www.")
		// 【关键修复】添加基础域名（之前缺失这一步）
		serverNames = append(serverNames, baseDomain)
	}

	// 添加常见的 www 子域名
	if !strings.HasPrefix(domain, "www.") {
		serverNames = append(serverNames, "www."+domain)
	}

	// 根据域名类型添加特定的子域名
	switch {
	case strings.Contains(domain, "apple.com") || strings.Contains(domain, "icloud.com"):
		serverNames = append(serverNames, "developer.apple.com", "store.apple.com", "www.icloud.com")
	case strings.Contains(domain, "google.com"):
		serverNames = append(serverNames, "www.google.com", "accounts.google.com", "play.google.com")
	case strings.Contains(domain, "microsoft.com"):
		serverNames = append(serverNames, "www.microsoft.com", "account.microsoft.com", "dev.microsoft.com")
	case strings.Contains(domain, "amazon.com"):
		serverNames = append(serverNames, "www.amazon.com", "smile.amazon.com", "sellercentral.amazon.com")
	case strings.Contains(domain, "github.com"):
		serverNames = append(serverNames, "www.github.com", "api.github.com", "docs.github.com")
	case strings.Contains(domain, "meta.com"):
		serverNames = append(serverNames, "www.meta.com", "developers.meta.com", "about.fb.com")
	case strings.Contains(domain, "tesla.com"):
		serverNames = append(serverNames, "www.tesla.com", "shop.tesla.com", "service.tesla.com")
	case strings.Contains(domain, "sega.com"):
		serverNames = append(serverNames, "www.sega.com", "games.sega.com", "support.sega.com")
	default:
		// 【修复】在基础域名上添加通用子域名
		serverNames = append(serverNames, "api."+baseDomain, "cdn."+baseDomain, "support."+baseDomain)
	}

	// 去重并限制数量（避免过长）
	result := s.removeDuplicatesFromSlice(serverNames)
	if len(result) > 8 {
		return result[:8]
	}
	return result
}
