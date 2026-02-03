package service

import (
	"archive/zip"
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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

	"github.com/google/uuid"
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

	// Application stats
	var rtm runtime.MemStats
	runtime.ReadMemStats(&rtm)
	status.AppStats.Mem = rtm.Sys
	//nolint:gosec
	status.AppStats.Threads = uint32(runtime.NumGoroutine())
	if p != nil && p.IsRunning() {
		status.AppStats.Uptime = p.GetUptime()
	} else {
		status.AppStats.Uptime = 0
	}

	return status
}

func (s *ServerService) GetXrayVersions() ([]string, error) {
	const (
		XrayURL    = "https://api.github.com/repos/XTLS/Xray-core/releases"
		bufferSize = 8192
	)

	// 使用带超时的HTTP客户端
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// 添加User-Agent头部以避免被GitHub拒绝
	req, err := http.NewRequest("GET", XrayURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Xray-UI-Panel/1.0")

	resp, err := client.Do(req)
	if err != nil {
		logger.Warning("Failed to fetch Xray versions from GitHub:", err)
		return nil, fmt.Errorf("无法获取Xray版本信息，请检查网络连接: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// 检查HTTP状态码
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API返回错误状态码: %d", resp.StatusCode)
	}

	buffer := bytes.NewBuffer(make([]byte, bufferSize))
	buffer.Reset()
	if _, err := buffer.ReadFrom(resp.Body); err != nil {
		return nil, fmt.Errorf("读取响应数据失败: %v", err)
	}

	var releases []Release
	if err := json.Unmarshal(buffer.Bytes(), &releases); err != nil {
		return nil, fmt.Errorf("解析JSON响应失败: %v", err)
	}

	var versions []string
	for _, release := range releases {
		tagVersion := release.TagName
		// 保留对 v 前缀的检查
		if !strings.HasPrefix(tagVersion, "v") {
			continue
		}

		// 验证版本格式是否正确
		versionWithoutPrefix := strings.TrimPrefix(tagVersion, "v")
		tagParts := strings.Split(versionWithoutPrefix, ".")
		if len(tagParts) != 3 {
			continue
		}

		// 验证版本号是否为有效数字
		_, err1 := strconv.Atoi(tagParts[0])
		_, err2 := strconv.Atoi(tagParts[1])
		_, err3 := strconv.Atoi(tagParts[2])
		if err1 != nil || err2 != nil || err3 != nil {
			continue
		}

		versions = append(versions, tagVersion)
	}

	// 如果没有找到版本，返回友好的错误信息
	if len(versions) == 0 {
		return nil, fmt.Errorf("未找到任何有效的Xray版本")
	}

	// 按版本号排序（最新在前）并只返回最新的3个版本
	if len(versions) > 3 {
		versions = versions[:3]
	}

	logger.Infof("成功获取到最新的 %d 个Xray版本", len(versions))
	return versions, nil
}

func (s *ServerService) StopXrayService() error {
	err := s.xrayService.StopXray()
	if err != nil {
		logger.Error("stop xray failed:", err)
		return err
	}
	return nil
}

func (s *ServerService) RestartXrayService() error {
	err := s.xrayService.RestartXray(true)
	if err != nil {
		logger.Error("start xray failed:", err)
		return err
	}
	return nil
}

// detectSystemArchitecture 检测系统实际架构
func detectSystemArchitecture() string {
	// 尝试使用 uname -m 检测系统架构
	cmd := exec.Command("uname", "-m")
	output, err := cmd.Output()
	if err == nil {
		systemArch := strings.TrimSpace(string(output))
		// 如果检测到 x86_64 或 amd64，说明系统支持64位
		if systemArch == "x86_64" || systemArch == "amd64" {
			return "64"
		}
		// 如果检测到 aarch64，说明系统支持64位 ARM
		if systemArch == "aarch64" {
			return "arm64-v8a"
		}
		// 其他情况返回系统报告的架构
		return systemArch
	}

	// 如果 uname 命令失败，回退到 runtime.GOARCH 检测
	return runtime.GOARCH
}

func (s *ServerService) downloadXRay(version string) (string, error) {
	osName := runtime.GOOS
	arch := runtime.GOARCH

	switch osName {
	case "darwin":
		osName = "macos"
	case "windows":
		osName = "windows"
	}

	// 获取系统实际架构
	systemArch := detectSystemArchitecture()

	switch arch {
	case "amd64":
		arch = "64"
	case "arm64":
		arch = "arm64-v8a"
	case "armv7":
		arch = "arm32-v7a"
	case "armv6":
		arch = "arm32-v6"
	case "armv5":
		arch = "arm32-v5"
	case "386":
		// 关键修复：如果 Go 程序运行在 386 模式下，但实际系统是 64 位，
		// 则下载 64 位版本，避免 "exit code 8" 错误
		if systemArch == "64" {
			arch = "64"
			logger.Info("检测到 32 位面板运行在 64 位系统上，使用 64 位 Xray")
		} else {
			arch = "32"
		}
	case "s390x":
		arch = "s390x"
	default:
		// 对于未知架构，尝试使用系统检测结果
		if systemArch != runtime.GOARCH {
			arch = systemArch
			logger.Infof("使用系统检测到的架构: %s", arch)
		}
	}

	fileName := fmt.Sprintf("Xray-%s-%s.zip", osName, arch)
	url := fmt.Sprintf("https://github.com/XTLS/Xray-core/releases/download/%s/%s", version, fileName)

	// 使用带超时的HTTP客户端
	client := &http.Client{
		Timeout: 120 * time.Second, // 下载需要更长时间
	}

	// 创建请求并添加User-Agent
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("创建下载请求失败: %v", err)
	}
	req.Header.Set("User-Agent", "Xray-UI-Panel/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("下载Xray失败: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// 检查HTTP状态码
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("下载失败，GitHub返回状态码: %d", resp.StatusCode)
	}

	_ = os.Remove(fileName)
	//nolint:gosec
	file, err := os.Create(fileName)
	if err != nil {
		return "", fmt.Errorf("创建文件失败: %v", err)
	}
	defer func() { _ = file.Close() }()

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return "", fmt.Errorf("写入文件失败: %v", err)
	}

	return fileName, nil
}

func (s *ServerService) UpdateXray(version string) error {
	// 启动异步任务进行Xray版本更新，避免阻塞HTTP请求
	go func() {
		logger.Infof("开始异步更新Xray到版本: %s", version)

		// 检查Telegram服务是否可用
		tgAvailable := s.tgService != nil && s.tgService.IsRunning()

		// 1. 在异步更新任务开始时发送开始通知
		if tgAvailable {
			startMessage := fmt.Sprintf("🔄 <b>开始更新 Xray 版本</b>\n\n正在更新到版本: <code>%s</code>\n\n⏳ 请稍候，这可能需要几分钟时间...", version)
			if err := s.tgService.SendMessage(startMessage); err != nil {
				logger.Warningf("发送Xray更新开始通知失败: %v", err)
			}
		}

		var updateErr error

		// 2. Stop xray before doing anything
		if err := s.StopXrayService(); err != nil {
			logger.Warning("failed to stop xray before update:", err)
			updateErr = fmt.Errorf("停止Xray服务失败: %v", err)
		} else {
			// 3. Download the zip
			zipFileName, err := s.downloadXRay(version)
			if err != nil {
				logger.Error("下载Xray失败:", err)
				updateErr = fmt.Errorf("下载Xray失败: %v", err)
			} else {
				defer func() { _ = os.Remove(zipFileName) }()
				//nolint:gosec
				zipFile, err := os.Open(zipFileName)
				if err != nil {
					logger.Error("打开zip文件失败:", err)
					updateErr = fmt.Errorf("打开zip文件失败: %v", err)
				} else {
					defer func() { _ = zipFile.Close() }()

					stat, err := zipFile.Stat()
					if err != nil {
						logger.Error("获取zip文件信息失败:", err)
						updateErr = fmt.Errorf("获取zip文件信息失败: %v", err)
					} else {
						reader, err := zip.NewReader(zipFile, stat.Size())
						if err != nil {
							logger.Error("创建zip reader失败:", err)
							updateErr = fmt.Errorf("创建zip reader失败: %v", err)
						} else {
							// 4. Helper to extract files
							copyZipFile := func(zipName string, fileName string) error {
								zipFile, err := reader.Open(zipName)
								if err != nil {
									return err
								}
								defer func() { _ = zipFile.Close() }()
								_ = os.MkdirAll(filepath.Dir(fileName), 0o750)
								_ = os.Remove(fileName)
								//nolint:gosec
								file, err := os.OpenFile(fileName, os.O_CREATE|os.O_RDWR|os.O_TRUNC, fs.ModePerm)
								if err != nil {
									return err
								}
								defer func() { _ = file.Close() }()
								// Limit decompression size to 100MB to prevent DoS (G110)
								//nolint:gosec
								_, err = io.Copy(file, io.LimitReader(zipFile, 100*1024*1024))
								return err
							}

							// 5. Extract correct binary
							if runtime.GOOS == "windows" {
								targetBinary := filepath.Join("bin", "xray-windows-amd64.exe")
								err = copyZipFile("xray.exe", targetBinary)
							} else {
								err = copyZipFile("xray", xray.GetBinaryPath())
							}
							if err != nil {
								logger.Error("解压Xray文件失败:", err)
								updateErr = fmt.Errorf("解压Xray文件失败: %v", err)
							} else {
								// 6. Restart xray
								if err := s.xrayService.RestartXray(true); err != nil {
									logger.Error("重启Xray失败:", err)
									updateErr = fmt.Errorf("重启Xray失败: %v", err)
								}
							}
						}
					}
				}
			}
		}

		// 7. 根据更新结果发送相应的通知
		if tgAvailable {
			if updateErr == nil {
				// 更新成功通知
				successMessage := fmt.Sprintf("✅ <b>Xray 更新成功！</b>\n\n版本: <code>%s</code>\n\n🎉 Xray 已成功更新并重新启动！", version)
				if err := s.tgService.SendMessage(successMessage); err != nil {
					logger.Warningf("发送Xray更新成功通知失败: %v", err)
				}
			} else {
				// 更新失败通知
				failMessage := fmt.Sprintf("❌ <b>Xray 更新失败</b>\n\n版本: <code>%s</code>\n\n错误信息: %v\n\n请检查日志以获取更多信息。", version, updateErr)
				if err := s.tgService.SendMessage(failMessage); err != nil {
					logger.Warningf("发送Xray更新失败通知失败: %v", err)
				}
			}
		}

		if updateErr != nil {
			logger.Errorf("Xray版本更新失败: %v", updateErr)
		} else {
			logger.Infof("Xray版本更新成功: %s", version)
		}
	}()

	return nil
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

// IsValidGeofileName validates that the filename is safe for geofile operations.
// It checks for path traversal attempts and ensures the filename contains only safe characters.
func (s *ServerService) IsValidGeofileName(filename string) bool {
	if filename == "" {
		return false
	}

	// Check for path traversal attempts
	if strings.Contains(filename, "..") {
		return false
	}

	// Check for path separators (both forward and backward slash)
	if strings.ContainsAny(filename, `/\`) {
		return false
	}

	// Check for absolute path indicators
	if filepath.IsAbs(filename) {
		return false
	}

	// Additional security: only allow alphanumeric, dots, underscores, and hyphens
	// This is stricter than the general filename regex
	validGeofilePattern := `^[a-zA-Z0-9._-]+\.dat$`
	matched, _ := regexp.MatchString(validGeofilePattern, filename)
	return matched
}

func (s *ServerService) UpdateGeofile(fileName string) error {
	files := []struct {
		URL      string
		FileName string
	}{
		{"https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geoip.dat", "geoip.dat"},
		{"https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geosite.dat", "geosite.dat"},
		{"https://github.com/chocolate4u/Iran-v2ray-rules/releases/latest/download/geoip.dat", "geoip_IR.dat"},
		{"https://github.com/chocolate4u/Iran-v2ray-rules/releases/latest/download/geosite.dat", "geosite_IR.dat"},
		{"https://github.com/runetfreedom/russia-v2ray-rules-dat/releases/latest/download/geoip.dat", "geoip_RU.dat"},
		{"https://github.com/runetfreedom/russia-v2ray-rules-dat/releases/latest/download/geosite.dat", "geosite_RU.dat"},
	}

	downloadFile := func(url, destPath string) error {
		// 创建带超时的HTTP客户端
		client := &http.Client{
			Timeout: 60 * time.Second, // 60秒超时
		}

		// 创建请求并添加User-Agent头部
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return common.NewErrorf("创建下载请求失败: %v", err)
		}
		req.Header.Set("User-Agent", "Xray-UI-Panel/1.0")

		resp, err := client.Do(req)
		if err != nil {
			return common.NewErrorf("Failed to download Geofile from %s: %v", url, err)
		}
		defer func() { _ = resp.Body.Close() }()

		// 检查HTTP状态码
		if resp.StatusCode != http.StatusOK {
			return common.NewErrorf("下载失败，服务器返回状态码: %d", resp.StatusCode)
		}

		//nolint:gosec
		file, err := os.Create(destPath)
		if err != nil {
			return common.NewErrorf("Failed to create Geofile %s: %v", destPath, err)
		}
		defer func() { _ = file.Close() }()

		_, err = io.Copy(file, resp.Body)
		if err != nil {
			return common.NewErrorf("Failed to save Geofile %s: %v", destPath, err)
		}

		return nil
	}

	var errorMessages []string

	if fileName == "" {
		for _, file := range files {
			destPath := fmt.Sprintf("%s/%s", config.GetBinFolderPath(), file.FileName)

			if err := downloadFile(file.URL, destPath); err != nil {
				errorMessages = append(errorMessages, fmt.Sprintf("Error downloading Geofile '%s': %v", file.FileName, err))
			}
		}
	} else {
		// Use the centralized validation function
		if !s.IsValidGeofileName(fileName) {
			return common.NewErrorf("Invalid geofile name: contains unsafe path characters: %s", fileName)
		}

		// Ensure the filename matches exactly one from our allowlist
		isAllowed := false
		for _, file := range files {
			if fileName == file.FileName {
				isAllowed = true
				break
			}
		}

		if !isAllowed {
			return common.NewErrorf("Invalid geofile name: %s not in allowlist", fileName)
		}

		destPath := fmt.Sprintf("%s/%s", config.GetBinFolderPath(), fileName)

		var fileURL string
		for _, file := range files {
			if file.FileName == fileName {
				fileURL = file.URL
				break
			}
		}

		if fileURL == "" {
			// This should practically not be reached because of the isAllowed check above
			errorMessages = append(errorMessages, fmt.Sprintf("File '%s' not found in the list of Geofiles", fileName))
		} else {
			if err := downloadFile(fileURL, destPath); err != nil {
				errorMessages = append(errorMessages, fmt.Sprintf("Error downloading Geofile '%s': %v", fileName, err))
			}
		}
	}

	err := s.RestartXrayService()
	if err != nil {
		errorMessages = append(errorMessages, fmt.Sprintf("Updated Geofile '%s' but Failed to start Xray: %v", fileName, err))
	}

	if len(errorMessages) > 0 {
		return common.NewErrorf("%s", strings.Join(errorMessages, "\r\n"))
	}

	return nil
}

func (s *ServerService) GetNewX25519Cert() (any, error) {
	// Run the command
	//nolint:gosec
	cmd := exec.Command(xray.GetBinaryPath(), "x25519")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(out.String(), "\n")

	privateKeyLine := strings.Split(lines[0], ":")
	publicKeyLine := strings.Split(lines[1], ":")

	privateKey := strings.TrimSpace(privateKeyLine[1])
	publicKey := strings.TrimSpace(publicKeyLine[1])

	keyPair := map[string]any{
		"privateKey": privateKey,
		"publicKey":  publicKey, // 修复：U+00A0 替换为标准空格
	}

	return keyPair, nil
}

func (s *ServerService) GetNewmldsa65() (any, error) {
	// Run the command
	//nolint:gosec
	cmd := exec.Command(xray.GetBinaryPath(), "mldsa65")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(out.String(), "\n")

	SeedLine := strings.Split(lines[0], ":")
	VerifyLine := strings.Split(lines[1], ":")

	seed := strings.TrimSpace(SeedLine[1])
	verify := strings.TrimSpace(VerifyLine[1])

	keyPair := map[string]any{
		"seed":   seed,
		"verify": verify,
	}

	return keyPair, nil
}

func (s *ServerService) GetNewEchCert(sni string) (interface{}, error) {
	// Run the command
	//nolint:gosec
	cmd := exec.Command(xray.GetBinaryPath(), "tls", "ech", "--serverName", sni)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(out.String(), "\n")
	if len(lines) < 4 {
		return nil, common.NewError("invalid ech cert")
	}

	configList := lines[1]
	serverKeys := lines[3]

	return map[string]interface{}{
		"echServerKeys": serverKeys,
		"echConfigList": configList,
	}, nil
}

func (s *ServerService) GetNewVlessEnc() (any, error) {
	//nolint:gosec
	cmd := exec.Command(xray.GetBinaryPath(), "vlessenc")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil, err
	}

	lines := strings.Split(out.String(), "\n")

	var auths []map[string]string
	var current map[string]string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Authentication:") {
			if current != nil {
				auths = append(auths, current)
			}
			current = map[string]string{
				"label": strings.TrimSpace(strings.TrimPrefix(line, "Authentication:")),
			}
		} else if strings.HasPrefix(line, `"decryption"`) || strings.HasPrefix(line, `"encryption"`) {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 && current != nil {
				key := strings.Trim(parts[0], `" `)
				val := strings.Trim(parts[1], `" `)
				current[key] = val
			}
		}
	}

	if current != nil {
		auths = append(auths, current)
	}

	return map[string]any{
		"auths": auths,
	}, nil
}

func (s *ServerService) GetNewUUID() (map[string]string, error) {
	newUUID, err := uuid.NewRandom()
	if err != nil {
		return nil, fmt.Errorf("failed to generate UUID: %w", err)
	}

	return map[string]string{
		"uuid": newUUID.String(),
	}, nil
}

func (s *ServerService) GetNewmlkem768() (any, error) {
	// Run the command
	//nolint:gosec
	cmd := exec.Command(xray.GetBinaryPath(), "mlkem768")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(out.String(), "\n")

	SeedLine := strings.Split(lines[0], ":")
	ClientLine := strings.Split(lines[1], ":")

	seed := strings.TrimSpace(SeedLine[1])
	client := strings.TrimSpace(ClientLine[1])

	keyPair := map[string]any{
		"seed":   seed,
		"client": client,
	}

	return keyPair, nil
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
