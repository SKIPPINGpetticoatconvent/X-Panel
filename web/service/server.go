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
	"runtime"
	"strconv"
	"strings"
	"time"

	"x-ui/config"
	"x-ui/database"
	"x-ui/logger"
	"x-ui/util/common"
	"x-ui/util/sys"
	"x-ui/web/service/firewall"
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

type Release struct {
	TagName string `json:"tag_name"`
}

type ServerService struct {
	xrayService     XrayService
	inboundService  InboundService
	tgService       TelegramService
	cachedIPv4      string
	cachedIPv6      string
	noIPv6          bool
	FirewallService firewall.FirewallService // 新增防火墙服务字段
}

// 【新增方法】: 用于从外部注入 TelegramService 实例
func (s *ServerService) SetTelegramService(tgService TelegramService) {
	s.tgService = tgService
}

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

	// TCP/UDP connections (使用 Rust 实现)
	status.TcpCount, err = sys.GetTCPCountRust()
	if err != nil {
		logger.Warning("get tcp connections failed:", err)
	}

	status.UdpCount, err = sys.GetUDPCountRust()
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
	defer resp.Body.Close()

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
	defer resp.Body.Close()

	// 检查HTTP状态码
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("下载失败，GitHub返回状态码: %d", resp.StatusCode)
	}

	os.Remove(fileName)
	file, err := os.Create(fileName)
	if err != nil {
		return "", fmt.Errorf("创建文件失败: %v", err)
	}
	defer file.Close()

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
			startMessage := fmt.Sprintf("🔄 **开始更新 Xray 版本**\n\n正在更新到版本: `%s`\n\n⏳ 请稍候，这可能需要几分钟时间...", version)
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
				defer os.Remove(zipFileName)

				zipFile, err := os.Open(zipFileName)
				if err != nil {
					logger.Error("打开zip文件失败:", err)
					updateErr = fmt.Errorf("打开zip文件失败: %v", err)
				} else {
					defer zipFile.Close()

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
								defer zipFile.Close()
								os.MkdirAll(filepath.Dir(fileName), 0755)
								os.Remove(fileName)
								file, err := os.OpenFile(fileName, os.O_CREATE|os.O_RDWR|os.O_TRUNC, fs.ModePerm)
								if err != nil {
									return err
								}
								defer file.Close()
								_, err = io.Copy(file, zipFile)
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
				successMessage := fmt.Sprintf("✅ **Xray 更新成功！**\n\n版本: `%s`\n\n🎉 Xray 已成功更新并重新启动！", version)
				if err := s.tgService.SendMessage(successMessage); err != nil {
					logger.Warningf("发送Xray更新成功通知失败: %v", err)
				}
			} else {
				// 更新失败通知
				failMessage := fmt.Sprintf("❌ **Xray 更新失败**\n\n版本: `%s`\n\n错误信息: %v\n\n请检查日志以获取更多信息。", version, updateErr)
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
	c, _ := strconv.Atoi(count)
	var lines []string

	if syslog == "true" {
		cmdArgs := []string{"journalctl", "-u", "x-ui", "--no-pager", "-n", count, "-p", level}
		// Run the command
		cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
		var out bytes.Buffer
		cmd.Stdout = &out
		err := cmd.Run()
		if err != nil {
			return []string{"Failed to run journalctl command!"}
		}
		lines = strings.Split(out.String(), "\n")
	} else {
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
	blackholes []string) []string {

	countInt, _ := strconv.Atoi(count)
	var lines []string

	pathToAccessLog, err := xray.GetAccessLogPath()
	if err != nil {
		return lines
	}

	file, err := os.Open(pathToAccessLog)
	if err != nil {
		return lines
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "" || strings.Contains(line, "api -> api") {
			//skipping empty lines and api calls
			continue
		}

		if filter != "" && !strings.Contains(line, filter) {
			//applying filter if it's not empty
			continue
		}

		//adding suffixes to further distinguish entries by outbound
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
	defer file.Close()

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

	// Check if we can init the db or not
	if err = database.InitDB(tempPath); err != nil {
		return common.NewErrorf("Error checking db: %v", err)
	}

	// Stop Xray
	s.StopXrayService()

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
		defer resp.Body.Close()

		// 检查HTTP状态码
		if resp.StatusCode != http.StatusOK {
			return common.NewErrorf("下载失败，服务器返回状态码: %d", resp.StatusCode)
		}

		file, err := os.Create(destPath)
		if err != nil {
			return common.NewErrorf("Failed to create Geofile %s: %v", destPath, err)
		}
		defer file.Close()

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
		destPath := fmt.Sprintf("%s/%s", config.GetBinFolderPath(), fileName)

		var fileURL string
		for _, file := range files {
			if file.FileName == fileName {
				fileURL = file.URL
				break
			}
		}

		if fileURL == "" {
			errorMessages = append(errorMessages, fmt.Sprintf("File '%s' not found in the list of Geofiles", fileName))
		}

		if err := downloadFile(fileURL, destPath); err != nil {
			errorMessages = append(errorMessages, fmt.Sprintf("Error downloading Geofile '%s': %v", fileName, err))
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
		"publicKey": publicKey, // 修复：U+00A0 替换为标准空格
	}

	return keyPair, nil
}

func (s *ServerService) GetNewmldsa65() (any, error) {
	// Run the command
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

	// 【核心修正】: 第一步，调用重构后的 AddLinkHistory 函数。
	// 这个函数现在是一个原子事务。如果它没有返回错误，就意味着数据已经成功提交到了 .wal 日志文件。
	err := database.AddLinkHistory(record)
	if err != nil {
		return err // 如果事务失败，直接返回错误，不执行后续操作
	}

	// 【核心修正】: 第二步，在事务成功提交后，我们在这里调用 Checkpoint。
	// 此时 .wal 文件中已经包含了我们的新数据，调用 Checkpoint 可以确保这些数据被立即写入主数据库文件。
	return database.Checkpoint()
}

// LoadLinkHistory loads the latest 10 links from the database
func (s *ServerService) LoadLinkHistory() ([]*database.LinkHistory, error) {
	return database.GetLinkHistory()
}

// 〔新增方法〕: 安装 Subconverter (异步执行)
// 〔中文注释〕: 此方法用于接收前端或 TG 的请求，并执行 x-ui.sh 脚本中的 subconverter 函数
func (s *ServerService) InstallSubconverter() error {
	// 〔中文注释〕: 使用一个新的 goroutine 来执行耗时的安装任务，这样 API 可以立即返回
	go func() {
        
        // 【新增功能】：执行端口放行操作
        var ufwWarning string
        if ufwErr := s.openSubconverterPorts(); ufwErr != nil {
            // 不中断流程，只生成警告消息
            logger.Warningf("自动放行 Subconverter 端口失败: %v", ufwErr)
            ufwWarning = fmt.Sprintf("⚠️ **警告：订阅转换端口放行失败**\n\n自动执行 UFW 命令失败，请务必**手动**在您的 VPS 上放行端口 `8000` 和 `15268`，否则服务将无法访问。失败详情：%v\n\n", ufwErr)
        }

		// 〔中文注释〕: 检查全局的 TgBot 实例是否存在并且正在运行
		if s.tgService == nil || !s.tgService.IsRunning() {
			logger.Warning("TgBot 未运行，无法发送【订阅转换】状态通知。")
			// 即使机器人未运行，安装流程也应继续，只是不发通知
            ufwWarning = "" // 如果机器人不在线，不发送任何警告/消息
		}

		// 脚本路径为 /usr/bin/x-ui
		// 〔中文注释〕: 通常，安装脚本会将主命令软链接或复制到 /usr/bin/ 目录下，使其成为一个系统命令。
		// 直接调用这个命令比调用源文件路径更规范，也能确保执行的是用户在命令行中使用的同一个脚本。
		scriptPath := "/usr/bin/x-ui"

		// 〔中文注释〕: 检查脚本文件是否存在
		if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
			errMsg := fmt.Sprintf("订阅转换安装失败：关键脚本文件 `%s` 未找到。", scriptPath)
			logger.Error(errMsg)
			if s.tgService != nil && s.tgService.IsRunning() {
				// 〔中文注释〕: 使用 Markdown 格式发送错误消息
				s.tgService.SendMessage("❌ " + errMsg)
			}
			return
		}

		// 〔中文注释〕: 正确的调用方式是：命令是 "x-ui"，参数是 "subconverter"。
		cmd := exec.Command(scriptPath, "subconverter")

		// 〔中文注释〕: 执行命令并获取其合并的输出（标准输出 + 标准错误），方便排查问题。
		// 〔重要〕: 这个命令可能需要几分钟才能执行完毕，Go程序会在此等待直到脚本执行完成。
		output, err := cmd.CombinedOutput()

		if err != nil {
			if s.tgService != nil && s.tgService.IsRunning() {
				// 构造失败消息
				message := fmt.Sprintf("❌ **订阅转换安装失败**！\n\n**错误信息**: %v\n**输出**: %s", err, string(output))
				s.tgService.SendMessage(message)
			}
			logger.Errorf("订阅转换安装失败: %v\n输出: %s", err, string(output))
			return
		} else {
            
            // 【新增逻辑】：如果之前端口放行失败，先发送警告消息
            if ufwWarning != "" {
                s.tgService.SendMessage(ufwWarning)
            }

			// 安装成功后，发送通知到 TG 机器人
			if s.tgService != nil && s.tgService.IsRunning() {
				// 获取面板域名，注意：t.getDomain() 是 Tgbot 的方法
				domain, getDomainErr := s.tgService.GetDomain()
				if getDomainErr != nil {
					logger.Errorf("TG Bot: 订阅转换安装成功，但获取域名失败: %v", getDomainErr)
				} else {
					// 构造消息，使用用户指定的格式
					message := fmt.Sprintf(
						"🎉 **恭喜！【订阅转换】模块已成功安装！**\n\n"+
							"您现在可以使用以下地址访问 Web 界面：\n\n"+
							"🔗 **登录地址**: `https://%s:15268`\n\n"+
							"默认用户名: `admin`\n"+
							"默认 密码: `123456`\n\n"+
							"可登录订阅转换后台修改您的密码！", domain)

					// 发送成功消息
					if sendErr := s.tgService.SendMessage(message); sendErr != nil {
						logger.Errorf("TG Bot: 订阅转换安装成功，但发送通知失败: %v", sendErr)
					} else {
						logger.Info("TG Bot: 订阅转换安装成功通知已发送。")
					}
				}
			}

			logger.Info("订阅转换安装成功。")
			return
		}
	}()

	return nil // 立即返回，表示指令已接收
}

// openSubconverterPorts 检查/安装 ufw 并放行 8000 和 15268 端口
// 【重构后】: 使用新的防火墙服务替代原始的 Shell 脚本逻辑
func (s *ServerService) openSubconverterPorts() error {
	// 订阅转换所需的端口列表
	subconverterPorts := []int{8000, 15268}
	
	// 依次放行所有需要的端口（默认同时开放 TCP 和 UDP）
	for _, port := range subconverterPorts {
		err := s.FirewallService.OpenPort(port, "")
		if err != nil {
			return fmt.Errorf("放行订阅转换端口 %d 失败: %v", port, err)
		}
		logger.Infof("订阅转换端口 %d 已成功放行", port)
	}
	
	return nil
}


// 【新增方法实现】: 后台前端开放指定端口
// OpenPort 供前端调用，自动检查/安装 ufw 并放行指定的端口。
// 〔中文注释〕: 整个函数逻辑被放入一个 go func() 协程中，实现异步后台执行。
// 〔中文注释〕: 函数签名不再返回 error，因为它会立即返回，无法得知后台任务的最终结果。
// 【重构后】: 使用新的防火墙服务替代原始的 Shell 脚本逻辑
func (s *ServerService) OpenPort(port string) {
	// 〔中文注释〕: 启动一个新的协程来处理耗时任务，这样 HTTP 请求可以立刻返回。
	go func() {
		// 1. 将 port string 转换为 int
		portInt, err := strconv.Atoi(port)
		if err != nil {
			// 〔中文注释〕: 在后台任务中，如果出错，我们只能记录日志，因为无法再返回给前端。
			logger.Errorf("端口号格式错误，无法转换为数字: %s", port)
			return
		}

		// 2. 使用新的防火墙服务异步放行端口（默认同时开放 TCP 和 UDP）
		// 使用异步方法，以便不会阻塞协程
		isSuccess, err := s.FirewallService.OpenPortAsync(portInt, "")
		
		// 3. 记录日志，以便诊断
		if err != nil {
			errorMsg := fmt.Sprintf("后台执行端口 %s 自动放行失败。错误: %v", port, err)
			logger.Error(errorMsg)
			// 〔可选〕: 未来可以在这里加入 Telegram 机器人通知等功能，来通知管理员任务失败。
		} else if isSuccess {
			logger.Infof("端口 %s 已成功放行", port)
		} else {
			logger.Warningf("端口 %s 放行操作未完成或超时", port)
		}
	}()
}

// 〔中文注释〕: 【新增函数】 - 重启面板服务
// 这个函数会执行 /usr/bin/x-ui restart 命令来重启整个面板服务。
func (s *ServerService) RestartPanel() error {
	// 〔中文注释〕: 定义脚本的绝对路径，确保执行的命令是正确的。
	scriptPath := "/usr/bin/x-ui"

	// 〔中文注释〕: 检查脚本文件是否存在，增加健壮性。
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		errMsg := fmt.Sprintf("关键脚本文件 `%s` 未找到，无法执行重启。", scriptPath)
		logger.Error(errMsg)
		return fmt.Errorf("%s", errMsg)
	}
	
	// 〔中文注释〕: 定义要执行的命令和参数。
	cmd := exec.Command(scriptPath, "restart")

	// 〔中文注释〕: 执行命令并捕获组合输出（标准输出和标准错误）。
	output, err := cmd.CombinedOutput()
	if err != nil {
		// 〔中文注释〕: 如果命令执行失败，记录详细日志并返回错误。
		logger.Errorf("执行 '%s restart' 失败: %v, 输出: %s", scriptPath, err, string(output))
		return fmt.Errorf("命令执行失败: %v", err)
	}

	// 〔中文注释〕: 如果命令成功执行，记录成功的日志。
	logger.Infof("'%s restart' 命令已成功执行。", scriptPath)
	return nil
}
