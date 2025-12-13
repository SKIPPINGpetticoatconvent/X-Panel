package service

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// GeoIP地理位置服务模块
// 负责IP地址获取、地理位置检测、缓存管理等功能

// getPublicIP 从指定URL获取公网IP
func getPublicIP(url string) string {
	client := &http.Client{
		Timeout: 3 * time.Second,
	}

	resp, err := client.Get(url)
	if err != nil {
		return "N/A"
	}
	defer resp.Body.Close()

	// Don't retry if access is blocked or region-restricted
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnavailableForLegalReasons {
		return "N/A"
	}
	if resp.StatusCode != http.StatusOK {
		return "N/A"
	}

	ip, err := io.ReadAll(resp.Body)
	if err != nil {
		return "N/A"
	}

	ipString := strings.TrimSpace(string(ip))
	if ipString == "" {
		return "N/A"
	}

	return ipString
}

// 【新增方法】: 检测服务器IP地理位置
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
	defer resp.Body.Close()

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

		"United Kingdom":           "GB",
		"UK":                       "GB",
		"Japan":                    "JP",
		"Korea":                    "KR",
		"South Korea":              "KR",
		"Germany":                  "DE",
		"France":                   "FR",
		"Canada":                   "CA",
		"Australia":                "AU",
		"Singapore":                "SG",
		"Hong Kong":                "HK",
		"Taiwan":                   "TW",
		"Netherlands":              "NL",
		"Sweden":                   "SE",
		"Norway":                   "NO",
		"Finland":                  "FI",
		"Denmark":                  "DK",
		"Switzerland":              "CH",
		"Belgium":                  "BE",
		"Austria":                  "AT",
		"Ireland":                  "IE",
		"Portugal":                 "PT",
		"Spain":                    "ES",
		"Italy":                    "IT",
		"Russia":                   "RU",
		"India":                    "IN",
		"Brazil":                   "BR",
		"Mexico":                   "MX",
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