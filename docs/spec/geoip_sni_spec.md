# GeoIP 集成与 SNI 智能选择规格说明 (Specification)

## 1. 背景与目标
为了提高节点的抗封锁能力和伪装效果，我们需要根据 VPS 所在的地理位置，自动选择符合当地特征的 SNI 域名。
本规格说明定义了如何集成 `MyIP.la` API 来获取 VPS 的地理位置，并据此动态加载对应国家的 SNI 列表。

## 2. 核心需求

### 2.1 GeoIP 服务
*   **主要 API 源**: 使用 `https://api.myip.la/en?json` 获取当前服务器的公网 IP 信息。
*   **备用 API 源**: 使用 `https://api.ip.sb/geoip` 作为备用地理位置查询服务。
*   **数据提取**: 解析 JSON 响应，提取 `country_code` 字段（例如 "US", "JP", "CN"）。
*   **隐私与安全**: 仅使用公开 API，不发送任何敏感数据。API URL 可配置，但默认指向 MyIP.la 和 IP.sb。
*   **容错与回退**:
    *   设置合理的超时时间（例如 10 秒）。
    *   如果主要 API 请求失败或解析错误，自动切换到备用 API。
    *   如果两个 API 都失败，应返回默认的国家代码（例如 "US" 或空字符串）。
    *   支持指数退避重试策略，提高获取成功率。

### 2.2 SNI 智能选择
*   **动态加载**: `SNISelector` 初始化或更新时，应根据获取到的 `country_code` 尝试加载 `sni/{CountryCode}/sni_domains.txt`。
*   **回退机制**:
    *   如果对应国家的目录不存在或文件为空，回退到默认列表（例如 `sni/US/sni_domains.txt` 或硬编码列表）。
    *   如果获取 GeoIP 失败，直接使用默认列表。
*   **缓存**: GeoIP 信息在服务启动后获取一次即可，或者低频缓存（例如每天更新一次），避免频繁请求 API。

### 2.3 备用 API 机制
*   **主备切换逻辑**：当主要 API (`api.myip.la`) 失败时，自动切换到备用 API (`api.ip.sb`)
*   **数据格式兼容性**：备用 API 返回的 JSON 格式需要转换为统一的数据结构
*   **错误处理**：两个 API 都失败时，返回统一的错误信息，触发回退逻辑
*   **性能优化**：主 API 优先使用，只有在失败时才尝试备用 API
*   **日志记录**：记录 API 切换过程，便于故障排查和监控

## 3. 架构设计

### 3.1 接口定义

#### `GeoIPService`
负责获取地理位置信息。

```go
type LocationInfo struct {
    CountryCode string `json:"country_code"`
    // 其他字段如 IP, City 等按需添加，目前只需 CountryCode
}

type GeoIPService interface {
    FetchLocation() (*LocationInfo, error)
}
```

#### `SNISelector` (更新)
集成 `GeoIPService`，在初始化阶段决定加载哪个列表。

### 3.2 目录结构
```text
sni/
├── US/
│   └── sni_domains.txt
├── JP/
│   └── sni_domains.txt
├── CN/
│   └── sni_domains.txt
└── ...
```

## 4. 伪代码 (Pseudocode)

### 4.1 GeoIPService

```go
// web/service/geoip_service.go

package service

import (
    "encoding/json"
    "fmt"
    "net/http"
    "time"
    "x-ui/logger"
)

// TDD Anchor: TestGeoIPService_FetchLocation_Success
// 模拟 HTTP 响应，验证能否正确解析 country_code
// TDD Anchor: TestGeoIPService_FetchLocation_Failure
// 模拟超时或 500 错误，验证错误处理和备用 API 回退

// 主要 API 响应结构
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

// 备用 API 响应结构
type IPSBLocation struct {
    IP           string  `json:"ip"`
    CountryCode  string  `json:"country_code"`
    CountryName  string  `json:"country_name"`
    Region       string  `json:"region"`
    RegionName   string  `json:"region_name"`
    City         string  `json:"city"`
    Latitude     float64 `json:"latitude"`
    Longitude    float64 `json:"longitude"`
    Timezone     string  `json:"timezone"`
    ISP          string  `json:"isp"`
    Organization string  `json:"organization"`
}

type GeoIPService struct {
    client *http.Client
}

func NewGeoIPService() *GeoIPService {
    return NewGeoIPServiceWithClient(nil)
}

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

// 主要 API 获取位置信息
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

// 备用 API 获取位置信息
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

// 核心方法：自动回退获取位置信息
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

// 带重试的获取方法
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

// 数据转换方法：将备用 API 格式转换为主 API 格式
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

// 获取国家代码的便捷方法
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
```

### 4.2 集成到 SNISelector

我们需要修改 `ServerService` 或 `SNISelector` 的初始化逻辑。建议在 `ServerService` 中协调，因为 `SNISelector` 应该保持纯粹的逻辑，不直接依赖网络请求。

```go
// web/service/server.go (更新逻辑)

// InitSNISelector 初始化 SNI 选择器
func (s *ServerService) InitSNISelector() {
    // 1. 尝试获取地理位置
    geoService := NewGeoIPService()
    location, err := geoService.FetchLocation()
    
    countryCode := "US" // 默认回退
    if err == nil && location != nil {
        countryCode = location.CountryCode
    }

    // 2. 根据国家代码加载 SNI 列表
    domains := s.loadSNIByCountry(countryCode)
    
    // TEST: Verify fallback to default SNI
    if len(domains) == 0 && countryCode != "US" {
        logger.Infof("No SNI list for %s, falling back to US", countryCode)
        domains = s.loadSNIByCountry("US")
    }

    // 3. 初始化选择器
    s.sniSelector = NewSNISelector(domains)
}

// loadSNIByCountry 辅助方法
func (s *ServerService) loadSNIByCountry(countryCode string) []string {
    filePath := fmt.Sprintf("sni/%s/sni_domains.txt", countryCode)
    // 读取文件逻辑...
    // 如果文件不存在，返回空切片
}
```

## 5. 测试计划 (TDD Anchors)

1.  **`TestGeoIP_ParseJSON`**:
    *   输入: `{"ip":"1.1.1.1", "country_code":"JP", ...}`
    *   期望: `LocationInfo.CountryCode` == "JP"

2.  **`TestGeoIP_NetworkError`**:
    *   模拟网络超时。
    *   期望: 返回 error，不 panic。

3.  **`TestGeoIPService_MainAPISuccess`**:
    *   模拟主要 API 成功响应。
    *   期望: 正确解析并返回地理位置信息。

4.  **`TestGeoIPService_BackupAPISuccess`**:
    *   模拟主要 API 失败，备用 API 成功。
    *   期望: 自动切换到备用 API 并正确返回地理位置信息。

5.  **`TestGeoIPService_BothAPIFail`**:
    *   模拟主要和备用 API 都失败。
    *   期望: 返回错误信息，触发默认回退逻辑。

6.  **`TestGeoIPService_DataConversion`**:
    *   测试备用 API 数据格式转换为主 API 格式。
    *   期望: IPSBLocation 正确转换为 GeoIPLocation。

7.  **`TestGeoIPService_RetryMechanism`**:
    *   模拟 API 调用失败后的重试机制。
    *   期望: 指数退避策略正常工作，最终成功或返回错误。

8.  **`TestServerService_LoadSNI_Fallback`**:
    *   场景: `FetchLocation` 返回 "XX" (不存在的国家)。
    *   期望: `loadSNIByCountry("XX")` 返回空 -> 触发回退逻辑 -> 加载 "US" 列表。

9.  **`TestServerService_LoadSNI_Success`**:
    *   场景: `FetchLocation` 返回 "JP"。
    *   期望: 加载 `sni/JP/sni_domains.txt` 的内容。