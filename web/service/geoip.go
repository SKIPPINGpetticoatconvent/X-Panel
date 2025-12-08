package service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"x-ui/logger"
)

// GeoIPLocation 表示从 API 返回的地理定位信息
type GeoIPLocation struct {
	IP          string  `json:"ip"`
	Location    struct {
		City         string  `json:"city"`
		CountryCode  string  `json:"country_code"`
		CountryName  string  `json:"country_name"`
		Latitude     string  `json:"latitude"`
		Longitude    string  `json:"longitude"`
		Province     string  `json:"province"`
	} `json:"location"`
}

// IPSBLocation 表示从 api.ip.sb 返回的地理定位信息（字段结构不同）
type IPSBLocation struct {
	IP            string  `json:"ip"`
	CountryCode   string  `json:"country_code"`
	CountryName   string  `json:"country"`
	Region        string  `json:"region"`
	RegionName    string  `json:"region_name"`
	City          string  `json:"city"`
	Latitude      float64 `json:"latitude"`
	Longitude     float64 `json:"longitude"`
	Timezone      string  `json:"timezone"`
	ISP           string  `json:"isp"`
	Organization  string  `json:"organization"`
}

// GetCountryCode 获取国家代码的便捷方法
func (g *GeoIPLocation) GetCountryCode() string {
	return g.Location.CountryCode
}

// GetCountry 获取国家名称的便捷方法
func (g *GeoIPLocation) GetCountry() string {
	return g.Location.CountryName
}

// GetCity 获取城市名称的便捷方法
func (g *GeoIPLocation) GetCity() string {
	return g.Location.City
}

// convertIPSBToGeoIP 将 IPSBLocation 转换为 GeoIPLocation
func convertIPSBToGeoIP(ipsb *IPSBLocation) *GeoIPLocation {
	geoIP := &GeoIPLocation{
		IP: ipsb.IP,
		Location: struct {
			City         string  `json:"city"`
			CountryCode  string  `json:"country_code"`
			CountryName  string  `json:"country_name"`
			Latitude     string  `json:"latitude"`
			Longitude    string  `json:"longitude"`
			Province     string  `json:"province"`
		}{
			City:         ipsb.City,
			CountryCode:  ipsb.CountryCode,
			CountryName:  ipsb.CountryName,
			Latitude:     fmt.Sprintf("%.4f", ipsb.Latitude),
			Longitude:    fmt.Sprintf("%.4f", ipsb.Longitude),
			Province:     ipsb.RegionName,
		},
	}
	return geoIP
}

// GeoIPService 提供地理位置查询服务
type GeoIPService struct {
	client *http.Client
}

// NewGeoIPService 创建新的 GeoIP 服务实例
func NewGeoIPService() *GeoIPService {
	return NewGeoIPServiceWithClient(nil)
}

// NewGeoIPServiceWithClient 创建带有自定义 HTTP 客户端的 GeoIP 服务实例
func NewGeoIPServiceWithClient(client *http.Client) *GeoIPService {
	if client == nil {
		client = &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:          10,
				IdleConnTimeout:       90 * time.Second,
				DisableCompression:    false,
				ResponseHeaderTimeout: 5 * time.Second,
			},
		}
	}
	return &GeoIPService{
		client: client,
	}
}

// FetchLocation 获取当前服务器的地理位置信息（支持主备 API）
func (g *GeoIPService) FetchLocation() (*GeoIPLocation, error) {
	// 首先尝试主 API
	if location, err := g.fetchFromMainAPI(); err == nil {
		return location, nil
	}
	
	// 主 API 失败，尝试备用 API
	logger.Warning("主 GeoIP API 失败，尝试备用 API")
	if location, err := g.fetchFromBackupAPI(); err == nil {
		return location, nil
	}
	
	// 两个 API 都失败
	return nil, fmt.Errorf("所有 GeoIP API 都失败")
}

// fetchFromMainAPI 从主 API 获取位置信息
func (g *GeoIPService) fetchFromMainAPI() (*GeoIPLocation, error) {
	logger.Info("尝试使用主 GeoIP API: https://api.myip.la")
	
	resp, err := g.client.Get("https://api.myip.la/en?json")
	if err != nil {
		logger.Errorf("主 GeoIP API 请求失败: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.Errorf("主 GeoIP API 返回非 200 状态码: %d", resp.StatusCode)
		return nil, fmt.Errorf("主 API 返回错误状态码: %d", resp.StatusCode)
	}

	var location GeoIPLocation
	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(&location); err != nil {
		logger.Errorf("主 GeoIP API JSON 解析失败: %v", err)
		return nil, err
	}

	logger.Infof("主 GeoIP API 获取成功: %s (%s)", location.GetCountry(), location.GetCountryCode())
	return &location, nil
}

// fetchFromBackupAPI 从备用 API 获取位置信息
func (g *GeoIPService) fetchFromBackupAPI() (*GeoIPLocation, error) {
	logger.Info("尝试使用备用 GeoIP API: https://api.ip.sb/geoip")
	
	resp, err := g.client.Get("https://api.ip.sb/geoip")
	if err != nil {
		logger.Errorf("备用 GeoIP API 请求失败: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.Errorf("备用 GeoIP API 返回非 200 状态码: %d", resp.StatusCode)
		return nil, fmt.Errorf("备用 API 返回错误状态码: %d", resp.StatusCode)
	}

	var ipsbLocation IPSBLocation
	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(&ipsbLocation); err != nil {
		logger.Errorf("备用 GeoIP API JSON 解析失败: %v", err)
		return nil, err
	}

	// 转换为统一格式
	geoIPLocation := convertIPSBToGeoIP(&ipsbLocation)
	logger.Infof("备用 GeoIP API 获取成功: %s (%s)", geoIPLocation.GetCountry(), geoIPLocation.GetCountryCode())
	return geoIPLocation, nil
}

// FetchLocationWithRetry 带重试机制的地理位置获取
func (g *GeoIPService) FetchLocationWithRetry(maxRetries int) (*GeoIPLocation, error) {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		location, err := g.FetchLocation()
		if err == nil {
			return location, nil
		}
		lastErr = err
		logger.Warningf("GeoIP 获取尝试 %d/%d 失败: %v", i+1, maxRetries, err)
		
		// 指数退避
		if i < maxRetries-1 {
			waitTime := time.Duration(1<<uint(i)) * time.Second
			time.Sleep(waitTime)
		}
	}
	return nil, fmt.Errorf("GeoIP 获取失败，已重试 %d 次: %w", maxRetries, lastErr)
}

// GetCountryCode 获取国家代码，如果失败则返回默认值
func (g *GeoIPService) GetCountryCode() string {
	location, err := g.FetchLocationWithRetry(3)
	if err != nil {
		logger.Warningf("GeoIP 获取失败，使用默认国家代码: %v", err)
		return "USA" // 默认使用美国
	}
	
	if location.GetCountryCode() == "" {
		logger.Warningf("GeoIP 返回空国家代码，使用默认值")
		return "USA"
	}
	
	logger.Infof("从 GeoIP 获取到国家代码: %s", location.GetCountryCode())
	return location.GetCountryCode()
}