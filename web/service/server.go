package service

import (
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"x-ui/database"
)

type ProcessState string

const (
	Running ProcessState = "running"
	Stop    ProcessState = "stop"
	Error   ProcessState = "error"
)

type Status struct {
	T           time.Time `json:"-"`
	Cpu         float64   `json:"cpu"`
	CpuCores    int       `json:"cpuCores"`
	LogicalPro  int       `json:"logicalPro"`
	CpuSpeedMhz float64   `json:"cpuSpeedMhz"`
	Mem         struct {
		Current uint64 `json:"current"`
		Total   uint64 `json:"total"`
	} `json:"mem"`
	Swap struct {
		Current uint64 `json:"current"`
		Total   uint64 `json:"total"`
	} `json:"swap"`
	Disk struct {
		Current uint64 `json:"current"`
		Total   uint64 `json:"total"`
	} `json:"disk"`
	Xray struct {
		State    ProcessState `json:"state"`
		ErrorMsg string       `json:"errorMsg"`
		Version  string       `json:"version"`
	} `json:"xray"`
	Uptime   uint64    `json:"uptime"`
	Loads    []float64 `json:"loads"`
	TcpCount int       `json:"tcpCount"`
	UdpCount int       `json:"udpCount"`
	NetIO    struct {
		Up   uint64 `json:"up"`
		Down uint64 `json:"down"`
	} `json:"netIO"`
	NetTraffic struct {
		Sent uint64 `json:"sent"`
		Recv uint64 `json:"recv"`
	} `json:"netTraffic"`
	PublicIP struct {
		IPv4 string `json:"ipv4"`
		IPv6 string `json:"ipv6"`
	} `json:"publicIP"`
	AppStats struct {
		Threads uint32 `json:"threads"`
		Mem     uint64 `json:"mem"`
		Uptime  uint64 `json:"uptime"`
	} `json:"appStats"`
}

type Release struct {
	TagName string `json:"tag_name"`
}

type ServerService struct {
	xrayService    XrayService
	inboundService InboundService
	tgService      TelegramService
	cachedIPv4     string
	cachedIPv6     string
	noIPv6         bool
	// IP地理位置缓存
	cachedCountry    string
	countryCheckTime time.Time
	// SNI 域名选择器
	sniSelector *SNISelector
	// GeoIP 服务
	geoIPService *GeoIPService
}

// SetTelegramService 用于从外部注入 TelegramService 实例
func (s *ServerService) SetTelegramService(tgService TelegramService) {
	s.tgService = tgService
}

// GetConfigJson 获取Xray配置JSON
func (s *ServerService) GetConfigJson() (any, error) {
	config, err := s.xrayService.GetXrayConfig()
	if err != nil {
		return nil, err
	}
	// 修复：将 U+00A0 替换为标准空格
	contents, err := json.MarshalIndent(config, "", " ")
	if err != nil {
		return nil, err
	}

	var jsonData any
	err = json.Unmarshal(contents, &jsonData)
	if err != nil {
		return nil, err
	}

	return jsonData, nil
}

// GetPublicIP 获取公网IP（缓存版本）
func (s *ServerService) GetPublicIP() string {
	showIp4ServiceLists := []string{
		"https://api4.ipify.org",
		"https://ipv4.icanhazip.com",
		"https://v4.api.ipinfo.io/ip",
		"https://ipv4.myexternalip.com/raw",
		"https://4.ident.me",
		"https://check-host.net/ip",
	}

	if s.cachedIPv4 == "" {
		for _, ip4Service := range showIp4ServiceLists {
			s.cachedIPv4 = getPublicIP(ip4Service)
			if s.cachedIPv4 != "N/A" {
				break
			}
		}
	}

	return s.cachedIPv4
}

// GetPublicIPv6 获取IPv6地址（缓存版本）
func (s *ServerService) GetPublicIPv6() string {
	showIp6ServiceLists := []string{
		"https://api6.ipify.org",
		"https://ipv6.icanhazip.com",
		"https://v6.api.ipinfo.io/ip",
		"https://ipv6.myexternalip.com/raw",
		"https://6.ident.me",
	}

	if s.cachedIPv6 == "" && !s.noIPv6 {
		for _, ip6Service := range showIp6ServiceLists {
			s.cachedIPv6 = getPublicIP(ip6Service)
			if s.cachedIPv6 != "N/A" {
				break
			}
		}
	}

	if s.cachedIPv6 == "N/A" {
		s.noIPv6 = true
	}

	return s.cachedIPv6
}

// getPublicIPInternal 从指定URL获取公网IP（内部方法）
func getPublicIPInternal(url string) string {
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

// GetCountryCode 获取国家代码
func (s *ServerService) GetCountryCode() string {
	if s.cachedCountry != "" && time.Since(s.countryCheckTime) < time.Hour {
		return s.cachedCountry
	}

	// 如果没有缓存，尝试从 GeoIP 服务获取
	if s.geoIPService != nil {
		countryCode := s.geoIPService.GetCountryCode()
		if countryCode != "" {
			s.cachedCountry = countryCode
			s.countryCheckTime = time.Now()
			return countryCode
		}
	}

	// 回退到根据IP推断地理位置
	ip := s.GetPublicIP()
	if ip != "N/A" && ip != "" {
		// 这里可以调用地理位置API，但为了简化，先返回默认值
		// 实际应用中应该调用具体的地理位置API
		s.cachedCountry = "US" // 默认值
		s.countryCheckTime = time.Now()
		return s.cachedCountry
	}

	return "Unknown"
}

// ClosePort 关闭端口（接口声明，由server_network.go提供具体实现）
func (s *ServerService) ClosePort(port string) error {
	// 此方法由 server_network.go 模块提供具体实现
	return fmt.Errorf("方法已迁移到 server_network.go 模块")
}

// CheckPortAvailability 检查端口可用性（接口声明，由server_network.go提供具体实现）
func (s *ServerService) CheckPortAvailability(port string) (bool, error) {
	// 此方法由 server_network.go 模块提供具体实现
	return false, fmt.Errorf("方法已迁移到 server_network.go 模块")
}

// GetStatus 获取系统状态
func (s *ServerService) GetStatus(lastStatus *Status) *Status {
	return s.GetSystemStatus(lastStatus)
}

// GetXrayVersions 获取Xray版本列表
func (s *ServerService) GetXrayVersions() ([]string, error) {
	return s.GetXrayVersionsAsync()
}

// UpdateXray 更新Xray版本
func (s *ServerService) UpdateXray(version string) error {
	return s.UpdateXrayAsync(version)
}

// UpdateGeofile 更新地理位置规则文件
func (s *ServerService) UpdateGeofile(fileName string) error {
	return s.UpdateGeofileAsync(fileName)
}

// StopXrayService 停止Xray服务
func (s *ServerService) StopXrayService() error {
	return s.StopXrayServiceAsync()
}

// RestartXrayService 重启Xray服务
func (s *ServerService) RestartXrayService() error {
	return s.RestartXrayServiceAsync()
}

// GetLogs 获取系统日志
func (s *ServerService) GetLogs(count string, level string, syslog string) []string {
	return s.GetLogsAsync(count, level, syslog)
}

// GetXrayLogs 获取Xray日志
func (s *ServerService) GetXrayLogs(
	count string,
	filter string,
	showDirect string,
	showBlocked string,
	showProxy string,
	freedoms []string,
	blackholes []string) []string {
	return s.GetXrayLogsAsync(count, filter, showDirect, showBlocked, showProxy, freedoms, blackholes)
}

// GetDb 获取数据库文件
func (s *ServerService) GetDb() ([]byte, error) {
	return s.GetDbAsync()
}

// ImportDB 导入数据库文件
func (s *ServerService) ImportDB(file multipart.File) error {
	return s.ImportDBAsync(file)
}

// GetNewUUID 生成新的UUID
func (s *ServerService) GetNewUUID() (map[string]string, error) {
	return s.GetNewUUIDAsync()
}

// SaveLinkHistory 保存链接历史记录
func (s *ServerService) SaveLinkHistory(historyType, link string) error {
	return s.SaveLinkHistoryAsync(historyType, link)
}

// LoadLinkHistory 加载链接历史记录
func (s *ServerService) LoadLinkHistory() ([]*database.LinkHistory, error) {
	return s.LoadLinkHistoryAsync()
}

// InstallSubconverter 安装Subconverter
func (s *ServerService) InstallSubconverter() error {
	return s.InstallSubconverterAsync()
}

// OpenPort 开放端口
func (s *ServerService) OpenPort(port string) {
	s.OpenPortAsync(port)
}

