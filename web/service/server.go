package service

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"x-ui/config"
	"x-ui/database"
	"x-ui/logger"
	"x-ui/util/common"
	"x-ui/util/sys"
	"x-ui/xray"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/net"
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

type ServerService struct {
	xrayService    *XrayService
	inboundService *InboundService
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

// NewServerService 创建 ServerService 实例
func NewServerService() *ServerService {
	return &ServerService{
		geoIPService: NewGeoIPService(),
		sniSelector:  NewSNISelector(nil),
	}
}

// 用于从外部注入 TelegramService 实例
func (s *ServerService) SetTelegramService(tgService TelegramService) {
	s.tgService = tgService
}

// SetXrayService 用于从外部注入 XrayService 实例
func (s *ServerService) SetXrayService(xrayService *XrayService) {
	s.xrayService = xrayService
}

// SetInboundService 用于从外部注入 InboundService 实例
func (s *ServerService) SetInboundService(inboundService *InboundService) {
	s.inboundService = inboundService
}

func getPublicIP(url string) string {
	client := &http.Client{
		Timeout: 3 * time.Second,
	}

	resp, err := client.Get(url)
	if err != nil {
		return "N/A"
	}
	defer func() { _ = resp.Body.Close() }()

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

func (s *ServerService) GetStatus(lastStatus *Status) *Status {
	now := time.Now()
	status := &Status{
		T: now,
	}

	// CPU stats
	percents, err := cpu.Percent(0, false)
	if err != nil {
		logger.Warning("get cpu percent failed:", err)
	} else {
		status.Cpu = percents[0]
	}

	status.CpuCores, err = cpu.Counts(false)
	if err != nil {
		logger.Warning("get cpu cores count failed:", err)
	}

	status.LogicalPro = runtime.NumCPU()

	cpuInfos, err := cpu.Info()
	if err != nil {
		logger.Warning("get cpu info failed:", err)
	} else if len(cpuInfos) > 0 {
		status.CpuSpeedMhz = cpuInfos[0].Mhz
	} else {
		logger.Warning("could not find cpu info")
	}

	// Uptime
	upTime, err := host.Uptime()
	if err != nil {
		logger.Warning("get uptime failed:", err)
	} else {
		status.Uptime = upTime
	}

	// Memory stats
	memInfo, err := mem.VirtualMemory()
	if err != nil {
		logger.Warning("get virtual memory failed:", err)
	} else {
		status.Mem.Current = memInfo.Used
		status.Mem.Total = memInfo.Total
	}

	swapInfo, err := mem.SwapMemory()
	if err != nil {
		logger.Warning("get swap memory failed:", err)
	} else {
		status.Swap.Current = swapInfo.Used
		status.Swap.Total = swapInfo.Total
	}

	// Disk stats
	diskInfo, err := disk.Usage("/")
	if err != nil {
		logger.Warning("get disk usage failed:", err)
	} else {
		status.Disk.Current = diskInfo.Used
		status.Disk.Total = diskInfo.Total
	}

	// Load averages
	avgState, err := load.Avg()
	if err != nil {
		logger.Warning("get load avg failed:", err)
	} else {
		status.Loads = []float64{avgState.Load1, avgState.Load5, avgState.Load15}
	}

	// Network stats
	ioStats, err := net.IOCounters(false)
	if err != nil {
		logger.Warning("get io counters failed:", err)
	} else if len(ioStats) > 0 {
		ioStat := ioStats[0]
		status.NetTraffic.Sent = ioStat.BytesSent
		status.NetTraffic.Recv = ioStat.BytesRecv

		if lastStatus != nil {
			duration := now.Sub(lastStatus.T)
			seconds := float64(duration) / float64(time.Second)
			up := uint64(float64(status.NetTraffic.Sent-lastStatus.NetTraffic.Sent) / seconds)
			down := uint64(float64(status.NetTraffic.Recv-lastStatus.NetTraffic.Recv) / seconds)
			status.NetIO.Up = up
			status.NetIO.Down = down
		}
	} else {
		logger.Warning("can not find io counters")
	}

	// TCP/UDP connections
	status.TcpCount, err = sys.GetTCPCount()
	if err != nil {
		logger.Warning("get tcp connections failed:", err)
	}

	status.UdpCount, err = sys.GetUDPCount()
	if err != nil {
		logger.Warning("get udp connections failed:", err)
	}

	// IP fetching with caching
	showIp4ServiceLists := []string{
		"https://api4.ipify.org",
		"https://ipv4.icanhazip.com",
		"https://v4.api.ipinfo.io/ip",
		"https://ipv4.myexternalip.com/raw",
		"https://4.ident.me",
		"https://check-host.net/ip",
	}
	showIp6ServiceLists := []string{
		"https://api6.ipify.org",
		"https://ipv6.icanhazip.com",
		"https://v6.api.ipinfo.io/ip",
		"https://ipv6.myexternalip.com/raw",
		"https://6.ident.me",
	}

	if s.cachedIPv4 == "" {
		for _, ip4Service := range showIp4ServiceLists {
			s.cachedIPv4 = getPublicIP(ip4Service)
			if s.cachedIPv4 != "N/A" {
				break
			}
		}
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

	status.PublicIP.IPv4 = s.cachedIPv4
	status.PublicIP.IPv6 = s.cachedIPv6

	// Xray status
	if s.xrayService != nil {
		if s.xrayService.IsXrayRunning() {
			status.Xray.State = Running
			status.Xray.ErrorMsg = ""
		} else {
			err := s.xrayService.GetXrayErr()
			if err != nil {
				status.Xray.State = Error
			} else {
				status.Xray.State = Stop
			}
			status.Xray.ErrorMsg = s.xrayService.GetXrayResult()
		}
		status.Xray.Version = s.xrayService.GetXrayVersion()
	} else {
		status.Xray.State = Stop
		status.Xray.ErrorMsg = "Xray service not initialized"
	}

	// Application stats
	var rtm runtime.MemStats
	runtime.ReadMemStats(&rtm)
	status.AppStats.Mem = rtm.Sys
	//nolint:gosec
	status.AppStats.Threads = uint32(runtime.NumGoroutine())
	if s.xrayService != nil && s.xrayService.IsXrayRunning() {
		status.AppStats.Uptime = s.xrayService.GetProcessUptime()
	} else {
		status.AppStats.Uptime = 0
	}

	return status
}

func (s *ServerService) GetLogs(count string, level string, syslog string) []string {
	var lines []string

	if syslog == "true" {
		// Check if running on Windows - journalctl is not available
		if runtime.GOOS == "windows" {
			return []string{"Syslog is not supported on Windows. Please use application logs instead by unchecking the 'Syslog' option."}
		}

		// Validate and sanitize count parameter
		countInt, err := strconv.Atoi(count)
		if err != nil || countInt < 1 || countInt > 10000 {
			return []string{"Invalid count parameter - must be a number between 1 and 10000"}
		}

		// Validate level parameter - only allow valid syslog levels
		validLevels := map[string]bool{
			"0": true, "emerg": true,
			"1": true, "alert": true,
			"2": true, "crit": true,
			"3": true, "err": true,
			"4": true, "warning": true,
			"5": true, "notice": true,
			"6": true, "info": true,
			"7": true, "debug": true,
		}
		if !validLevels[level] {
			return []string{"Invalid level parameter - must be a valid syslog level"}
		}

		// Use hardcoded command with validated parameters
		//nolint:gosec
		cmd := exec.Command("journalctl", "-u", "x-ui", "--no-pager", "-n", strconv.Itoa(countInt), "-p", level)
		var out bytes.Buffer
		cmd.Stdout = &out
		err = cmd.Run()
		if err != nil {
			return []string{"Failed to run journalctl command! Make sure systemd is available and x-ui service is registered."}
		}
		lines = strings.Split(out.String(), "\n")
	} else {
		c, _ := strconv.Atoi(count)
		lines = logger.GetLogs(c, level)
	}

	return lines
}

func (s *ServerService) GetXrayLogs(
	count string,
	filter string,
	showDirect string,
	showBlocked string,
	showProxy string,
	freedoms []string,
	blackholes []string,
) []string {
	countInt, _ := strconv.Atoi(count)
	var lines []string

	pathToAccessLog, err := xray.GetAccessLogPath()
	if err != nil {
		return lines
	}
	//nolint:gosec
	file, err := os.Open(pathToAccessLog)
	if err != nil {
		return lines
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "" || strings.Contains(line, "api -> api") {
			// skipping empty lines and api calls
			continue
		}

		if filter != "" && !strings.Contains(line, filter) {
			// applying filter if it's not empty
			continue
		}

		// adding suffixes to further distinguish entries by outbound
		if hasSuffix(line, freedoms) {
			if showDirect == "false" {
				continue
			}
			line = line + " f"
		} else if hasSuffix(line, blackholes) {
			if showBlocked == "false" {
				continue
			}
			line = line + " b"
		} else {
			if showProxy == "false" {
				continue
			}
			line = line + " p"
		}

		lines = append(lines, line)
	}

	if len(lines) > countInt {
		lines = lines[len(lines)-countInt:]
	}

	return lines
}

func hasSuffix(line string, suffixes []string) bool {
	for _, sfx := range suffixes {
		if strings.HasSuffix(line, sfx+"]") {
			return true
		}
	}
	return false
}

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

func (s *ServerService) GetDb() ([]byte, error) {
	// Update by manually trigger a checkpoint operation
	err := database.Checkpoint()
	if err != nil {
		return nil, err
	}
	// Open the file for reading
	file, err := os.Open(config.GetDBPath())
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	// Read the file contents
	fileContents, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	return fileContents, nil
}

func (s *ServerService) ImportDB(file multipart.File) error {
	// Check if the file is a SQLite database
	isValidDb, err := database.IsSQLiteDB(file)
	if err != nil {
		return common.NewErrorf("Error checking db file format: %v", err)
	}
	if !isValidDb {
		return common.NewError("Invalid db file format")
	}

	// Reset the file reader to the beginning
	_, err = file.Seek(0, 0)
	if err != nil {
		return common.NewErrorf("Error resetting file reader: %v", err)
	}

	// Save the file as a temporary file
	tempPath := fmt.Sprintf("%s.temp", config.GetDBPath())

	// Remove the existing temporary file (if any)
	if _, err := os.Stat(tempPath); err == nil {
		if errRemove := os.Remove(tempPath); errRemove != nil {
			return common.NewErrorf("Error removing existing temporary db file: %v", errRemove)
		}
	}

	// Create the temporary file
	//nolint:gosec
	tempFile, err := os.Create(tempPath)
	if err != nil {
		return common.NewErrorf("Error creating temporary db file: %v", err)
	}

	// Robust deferred cleanup for the temporary file
	defer func() {
		if tempFile != nil {
			if cerr := tempFile.Close(); cerr != nil {
				logger.Warningf("Warning: failed to close temp file: %v", cerr)
			}
		}
		if _, err := os.Stat(tempPath); err == nil {
			if rerr := os.Remove(tempPath); rerr != nil {
				logger.Warningf("Warning: failed to remove temp file: %v", rerr)
			}
		}
	}()

	// Save uploaded file to temporary file
	if _, err = io.Copy(tempFile, file); err != nil {
		return common.NewErrorf("Error saving db: %v", err)
	}

	// Close temp file before opening via sqlite
	if err = tempFile.Close(); err != nil {
		return common.NewErrorf("Error closing temporary db file: %v", err)
	}
	tempFile = nil

	// Validate integrity (no migrations / side effects)
	if err = database.ValidateSQLiteDB(tempPath); err != nil {
		return common.NewErrorf("Invalid or corrupt db file: %v", err)
	}

	// Stop Xray (ignore error but log)
	if errStop := s.StopXrayService(); errStop != nil {
		logger.Warningf("Failed to stop Xray before DB import: %v", errStop)
	}

	// Close existing DB to release file locks (especially on Windows)
	if errClose := database.CloseDB(); errClose != nil {
		logger.Warningf("Failed to close existing DB before replacement: %v", errClose)
	}

	// Backup the current database for fallback
	fallbackPath := fmt.Sprintf("%s.backup", config.GetDBPath())

	// Remove the existing fallback file (if any)
	if _, err := os.Stat(fallbackPath); err == nil {
		if errRemove := os.Remove(fallbackPath); errRemove != nil {
			return common.NewErrorf("Error removing existing fallback db file: %v", errRemove)
		}
	}

	// Move the current database to the fallback location
	if err = os.Rename(config.GetDBPath(), fallbackPath); err != nil {
		return common.NewErrorf("Error backing up current db file: %v", err)
	}

	// Defer fallback cleanup ONLY if everything goes well
	defer func() {
		if _, err := os.Stat(fallbackPath); err == nil {
			if rerr := os.Remove(fallbackPath); rerr != nil {
				logger.Warningf("Warning: failed to remove fallback file: %v", rerr)
			}
		}
	}()

	// Move temp to DB path
	if err = os.Rename(tempPath, config.GetDBPath()); err != nil {
		// Restore from fallback
		if errRename := os.Rename(fallbackPath, config.GetDBPath()); errRename != nil {
			return common.NewErrorf("Error moving db file and restoring fallback: %v", errRename)
		}
		return common.NewErrorf("Error moving db file: %v", err)
	}

	// Migrate DB
	if err = database.InitDB(config.GetDBPath()); err != nil {
		if errRename := os.Rename(fallbackPath, config.GetDBPath()); errRename != nil {
			return common.NewErrorf("Error migrating db and restoring fallback: %v", errRename)
		}
		return common.NewErrorf("Error migrating db: %v", err)
	}

	s.inboundService.MigrateDB()

	// Start Xray
	if err = s.RestartXrayService(); err != nil {
		return common.NewErrorf("Imported DB but failed to start Xray: %v", err)
	}

	return nil
}

// SaveLinkHistory 保存一个新的链接记录，并确保其被永久写入数据库文件。
func (s *ServerService) SaveLinkHistory(historyType, link string) error {
	record := &database.LinkHistory{
		Type:      historyType,
		Link:      link,
		CreatedAt: time.Now(),
	}

	// 第一步，调用重构后的 AddLinkHistory 函数。
	// 这个函数现在是一个原子事务。如果它没有返回错误，就意味着数据已经成功提交到了 .wal 日志文件。
	err := database.AddLinkHistory(record)
	if err != nil {
		return err // 如果事务失败，直接返回错误，不执行后续操作
	}

	// 第二步，在事务成功提交后，我们在这里调用 Checkpoint。
	// 此时 .wal 文件中已经包含了我们的新数据，调用 Checkpoint 可以确保这些数据被立即写入主数据库文件。
	return database.Checkpoint()
}

// LoadLinkHistory loads the latest 10 links from the database
func (s *ServerService) LoadLinkHistory() ([]*database.LinkHistory, error) {
	return database.GetLinkHistory()
}
