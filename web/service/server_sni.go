package service

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"x-ui/config"
	"x-ui/logger"
)

// =============================================================================
// 地理位置检测
// =============================================================================

// 检测服务器IP地理位置
func (s *ServerService) GetServerLocation() (string, error) {
	// 检查缓存，如果1小时内已经检测过，直接返回缓存结果
	if s.cachedCountry != "" && time.Since(s.countryCheckTime) < time.Hour {
		return s.cachedCountry, nil
	}

	// 获取服务器公网IP，尝试多个API
	var serverIP string
	ipAPIs := []string{
		"https://api4.ipify.org",
		"https://ipv4.icanhazip.com",
		"https://v4.api.ipinfo.io/ip",
		"https://ipv4.myexternalip.com/raw",
	}

	// 首先尝试使用缓存的IP
	if s.cachedIPv4 != "" && s.cachedIPv4 != "N/A" {
		serverIP = s.cachedIPv4
	}

	// 如果缓存中没有IP或IP无效，尝试获取新的IP
	if serverIP == "" || serverIP == "N/A" {
		for _, apiURL := range ipAPIs {
			ip := getPublicIP(apiURL)
			if ip != "N/A" && ip != "" {
				serverIP = ip
				break
			}
		}
	}

	if serverIP == "" || serverIP == "N/A" {
		return "Unknown", fmt.Errorf("无法获取服务器公网IP，所有API都不可用")
	}

	// 使用多个地理位置检测API
	geoAPIs := []string{
		fmt.Sprintf("https://ipapi.co/%s/json/", serverIP),
		fmt.Sprintf("https://ip-api.com/json/%s?fields=status,country,message", serverIP),
	}

	var country string
	for _, apiURL := range geoAPIs {
		country = s.queryLocationAPI(apiURL, serverIP)
		if country != "" && country != "Unknown" {
			break
		}
	}

	// 更新缓存
	if country == "" {
		country = "Unknown"
	}

	// 标准化国家代码
	country = normalizeCountryCode(country)

	// 缓存结果
	if country != "Unknown" {
		s.cachedCountry = country
		s.countryCheckTime = time.Now()
	}

	return country, nil
}

// queryLocationAPI 查询地理位置API
func (s *ServerService) queryLocationAPI(apiURL, serverIP string) string {
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return ""
	}

	req.Header.Set("User-Agent", "Xray-UI-Panel/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return ""
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ""
	}

	// 解析ipapi.co响应
	if strings.Contains(apiURL, "ipapi.co") {
		var response map[string]interface{}
		if err := json.Unmarshal(body, &response); err == nil {
			if country, ok := response["country_code"].(string); ok && country != "" {
				return country
			}
			if countryName, ok := response["country"].(string); ok && countryName != "" {
				return countryName
			}
		}
	}

	// 解析ip-api.com响应
	if strings.Contains(apiURL, "ip-api.com") {
		var response struct {
			Status  string `json:"status"`
			Country string `json:"country"`
			Message string `json:"message"`
		}
		if err := json.Unmarshal(body, &response); err == nil {
			if response.Status == "success" && response.Country != "" {
				return response.Country
			}
		}
	}

	return ""
}

// normalizeCountryCode 标准化国家代码
func normalizeCountryCode(country string) string {
	country = strings.TrimSpace(country)

	// 将国家名称映射到ISO代码
	countryMap := map[string]string{
		"United States":            "US",
		"United States of America": "US",
		"USA":                      "US",

		"United Kingdom": "GB",
		"UK":             "GB",
		"Japan":          "JP",
		"Korea":          "KR",
		"South Korea":    "KR",
		"Germany":        "DE",
		"France":         "FR",
		"Canada":         "CA",
		"Australia":      "AU",
		"Singapore":      "SG",
		"Hong Kong":      "HK",
		"Taiwan":         "TW",
		"Netherlands":    "NL",
		"Sweden":         "SE",
		"Norway":         "NO",
		"Finland":        "FI",
		"Denmark":        "DK",
		"Switzerland":    "CH",
		"Belgium":        "BE",
		"Austria":        "AT",
		"Ireland":        "IE",
		"Portugal":       "PT",
		"Spain":          "ES",
		"Italy":          "IT",
		"Russia":         "RU",
		"India":          "IN",
		"Brazil":         "BR",
		"Mexico":         "MX",
	}

	// 检查精确匹配
	if normalized, exists := countryMap[country]; exists {
		return normalized
	}

	// 检查不区分大小写的匹配
	for key, value := range countryMap {
		if strings.EqualFold(strings.ToLower(country), strings.ToLower(key)) {
			return value
		}
	}

	// 如果已经是标准的国家代码，直接返回
	if len(country) == 2 {
		return strings.ToUpper(country)
	}

	return "Unknown"
}

// =============================================================================
// SNI 域名管理
// =============================================================================

// readSNIDomainsFromFile 通用函数：从指定国家的SNI文件读取域名列表
func (s *ServerService) readSNIDomainsFromFile(countryCode string) ([]string, error) {
	filePath := filepath.Join(config.GetSNIFolderPath(), countryCode, "sni_domains.txt")

	// 读取SNI域名文件
	//nolint:gosec
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("读取 SSL 证书文件失败: %v", err)
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

	logger.Infof("从 %s 文件成功读取到 %d 个SNI域名", filePath, len(domains))
	return domains, nil
}

// 获取指定国家的SNI域名列表（优先从文件读取）
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
	// 最小化硬编码，只保留最基本的回退域名
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

// GetRandomRealitySNI 获取一个随机的 Reality SNI 信息，返回 target 和 domain
func (s *ServerService) GetRandomRealitySNI() (string, string) {
	if s.sniSelector == nil {
		logger.Warning("SNI selector not initialized, initializing now")
		s.initSNISelector()
	}

	// 获取下一个 SNI 域名
	sni := s.sniSelector.Next()

	// 解析 SNI 域名，提取 domain 部分
	domain := sni
	if strings.Contains(sni, ":") {
		domain = strings.Split(sni, ":")[0]
	}

	// 返回 target (完整 SNI) 和 domain (域名部分)
	return sni, domain
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

// GetGeoIPInfo 获取当前 GeoIP 信息
func (s *ServerService) GetGeoIPInfo() string {
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

// =============================================================================
// 系统操作
// =============================================================================

// 与 TG 端 openPortWithFirewalld 采用完全相同的 Shell 脚本执行逻辑。
// OpenPort 供前端调用，自动检查/安装 firewalld 并放行指定的端口。
// 改为同步执行，使用完整的 Shell 脚本（与 TG 端一致），确保端口放行操作的可靠性。
func (s *ServerService) OpenPort(port string) error {
	// 1. 验证端口号：必须是数字，且在有效范围内 (1-65535)
	portInt, err := strconv.Atoi(port)
	if err != nil || portInt < 1 || portInt > 65535 {
		return fmt.Errorf("端口号无效，必须是 1-65535 之间的数字: %s", port)
	}

	// 将所有 Shell 逻辑整合为一个命令，与 TG 端 openPortWithFirewalld 完全一致。
	// 新增了对默认端口列表 (22, 80, 443, 13688, 8443) 的放行逻辑。
	shellCommand := fmt.Sprintf(`
	# 定义需要放行的指定端口和一系列默认端口
	PORT_TO_OPEN=%d
	DEFAULT_PORTS="22 80 443 13688 8443"

	echo "脚本开始：准备配置 firewalld 防火墙..."

	# 1. 检查/安装 firewalld
	if ! command -v firewall-cmd &> /dev/null; then
		echo "firewalld 防火墙未安装，正在自动安装..."
		# 使用新的防火墙安装命令
		sudo apt update
		sudo apt install -y firewalld
		sudo systemctl enable firewalld --now
	fi

	# 2. 【新增】循环放行所有默认端口
	echo "正在检查并放行基础服务端口: $DEFAULT_PORTS"
	for p in $DEFAULT_PORTS; do
		# 使用静默模式检查规则是否存在，如果不存在则添加
		if ! firewall-cmd --list-ports | grep -qw "$p/tcp"; then
			echo "端口 $p/tcp 未放行，正在执行 firewall-cmd --zone=public --add-port=$p/tcp --permanent..."
			firewall-cmd --zone=public --add-port=$p/tcp --permanent >/dev/null
			if [ $? -ne 0 ]; then echo "❌ firewalld 端口 $p 放行失败。"; exit 1; fi
		else
			echo "端口 $p/tcp 规则已存在，跳过。"
		fi
	done
	echo "✅ 基础服务端口检查/放行完毕。"

	# 3. 放行指定的端口
	echo "正在为当前【入站配置】放行指定端口 $PORT_TO_OPEN..."
	if ! firewall-cmd --list-ports | grep -qw "$PORT_TO_OPEN/tcp"; then
		firewall-cmd --zone=public --add-port=$PORT_TO_OPEN/tcp --permanent >/dev/null
		if [ $? -ne 0 ]; then echo "❌ firewalld 端口 $PORT_TO_OPEN 放行失败。"; exit 1; fi
		echo "✅ 端口 $PORT_TO_OPEN 已成功放行。"
	else
		echo "端口 $PORT_TO_OPEN 规则已存在，跳过。"
	fi
	

	# 4. 检查/激活防火墙
	if ! systemctl is-active --quiet firewalld; then
		echo "firewalld 状态：未激活。正在启动..."
		systemctl start firewalld
		systemctl enable firewalld
		if [ $? -ne 0 ]; then echo "❌ firewalld 激活失败。"; exit 1; fi
		echo "✅ firewalld 已成功激活。"
	else
		echo "firewalld 状态已经是激活状态。"
	fi

	# 重新加载规则
	firewall-cmd --reload
	if [ $? -ne 0 ]; then echo "❌ firewalld 重新加载失败。"; exit 1; fi
	echo "✅ firewalld 规则已重新加载。"

	echo "🎉 所有防火墙配置已完成。"

	`, portInt) // 将函数传入的 port 参数填充到 Shell 脚本中

	// 使用 exec.CommandContext 运行完整的 shell 脚本
	//nolint:gosec
	cmd := exec.Command("/bin/bash", "-c", shellCommand)

	// 捕获命令的标准输出和标准错误
	output, err := cmd.CombinedOutput()

	// 无论成功与否，都记录完整的 Shell 执行日志，便于调试
	logOutput := string(output)
	logger.Infof("执行 firewalld 端口放行脚本（目标端口 %d）的完整输出：\n%s", portInt, logOutput)

	if err != nil {
		// 如果脚本执行出错 (例如 exit 1)，则返回包含详细输出的错误信息
		return fmt.Errorf("执行 firewalld 端口放行脚本时发生错误: %v, Shell 输出: %s", err, logOutput)
	}

	return nil
}

// 重启面板服务
// 这个函数会执行 /usr/bin/x-ui restart 命令来重启整个面板服务。
func (s *ServerService) RestartPanel() error {
	// 定义脚本的绝对路径，确保执行的命令是正确的。
	scriptPath := "/usr/bin/x-ui"

	// 检查脚本文件是否存在，增加健壮性。
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		errMsg := fmt.Sprintf("关键脚本文件 `%s` 未找到，无法执行重启。", scriptPath)
		logger.Error(errMsg)
		return fmt.Errorf("%s", errMsg)
	}

	// 定义要执行的命令和参数。
	cmd := exec.Command(scriptPath, "restart")

	// 执行命令并捕获组合输出（标准输出和标准错误）。
	output, err := cmd.CombinedOutput()
	if err != nil {
		// 如果命令执行失败，记录详细日志并返回错误。
		logger.Errorf("执行 '%s restart' 失败: %v, 输出: %s", scriptPath, err, string(output))
		return fmt.Errorf("命令执行失败: %v", err)
	}

	// 如果命令成功执行，记录成功的日志。
	logger.Infof("'%s restart' 命令已成功执行。", scriptPath)
	return nil
}
