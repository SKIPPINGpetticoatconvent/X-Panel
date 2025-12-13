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
	"x-ui/xray"

	"github.com/google/uuid"
)

// 系统操作工具模块
// 负责Xray服务控制、系统管理、日志操作、数据库操作等核心功能

// RestartXrayServiceAsync 重启Xray服务
func (s *ServerService) RestartXrayServiceAsync() error {
	err := s.xrayService.RestartXray(true)
	if err != nil {
		logger.Error("start xray failed:", err)
		return err
	}
	return nil
}

// StopXrayServiceAsync 停止Xray服务
func (s *ServerService) StopXrayServiceAsync() error {
	err := s.xrayService.StopXray()
	if err != nil {
		logger.Error("stop xray failed:", err)
		return err
	}
	return nil
}

// RestartPanel 重启面板服务
func (s *ServerService) RestartPanelAsync() error {
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

// GetXrayVersions 获取可用的Xray版本列表
func (s *ServerService) GetXrayVersionsAsync() ([]string, error) {
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

// detectSystemArchitecture 检测系统实际架构
func detectSystemArchitectureAsync() string {
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

// downloadXRay 下载指定版本的Xray
func (s *ServerService) downloadXRayAsync(version string) (string, error) {
	osName := runtime.GOOS
	arch := runtime.GOARCH

	switch osName {
	case "darwin":
		osName = "macos"
	case "windows":
		osName = "windows"
	}

	// 获取系统实际架构
	systemArch := detectSystemArchitectureAsync()

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

// UpdateXray 更新Xray版本（异步执行）
func (s *ServerService) UpdateXrayAsync(version string) error {
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
		if err := s.StopXrayServiceAsync(); err != nil {
			logger.Warning("failed to stop xray before update:", err)
			updateErr = fmt.Errorf("停止Xray服务失败: %v", err)
		} else {
			// 3. Download the zip
			zipFileName, err := s.downloadXRayAsync(version)
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

// GetLogs 获取系统日志
func (s *ServerService) GetLogsAsync(count string, level string, syslog string) []string {
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

// GetXrayLogs 获取Xray日志
func (s *ServerService) GetXrayLogsAsync(
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
		if hasSuffixAsync(line, freedoms) {
			if showDirect == "false" {
				continue
			}
			line = line + " f"
		} else if hasSuffixAsync(line, blackholes) {
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

// hasSuffix 检查字符串是否有指定后缀
func hasSuffixAsync(line string, suffixes []string) bool {
	for _, sfx := range suffixes {
		if strings.HasSuffix(line, sfx+"]") {
			return true
		}
	}
	return false
}

// GetDb 获取数据库文件
func (s *ServerService) GetDbAsync() ([]byte, error) {
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

// ImportDB 导入数据库文件
func (s *ServerService) ImportDBAsync(file multipart.File) error {
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
	s.StopXrayServiceAsync()

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
	if err = s.RestartXrayServiceAsync(); err != nil {
		return common.NewErrorf("Imported DB but failed to start Xray: %v", err)
	}

	return nil
}

// UpdateGeofile 更新地理位置规则文件
func (s *ServerService) UpdateGeofileAsync(fileName string) error {
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

	err := s.RestartXrayServiceAsync()
	if err != nil {
		errorMessages = append(errorMessages, fmt.Sprintf("Updated Geofile '%s' but Failed to start Xray: %v", fileName, err))
	}

	if len(errorMessages) > 0 {
		return common.NewErrorf("%s", strings.Join(errorMessages, "\r\n"))
	}

	return nil
}

// GetNewUUID 生成新的UUID
func (s *ServerService) GetNewUUIDAsync() (map[string]string, error) {
	newUUID, err := uuid.NewRandom()
	if err != nil {
		return nil, fmt.Errorf("failed to generate UUID: %w", err)
	}

	return map[string]string{
		"uuid": newUUID.String(),
	}, nil
}

// SaveLinkHistory 保存链接历史记录
func (s *ServerService) SaveLinkHistoryAsync(historyType, link string) error {
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

// LoadLinkHistory 加载链接历史记录
func (s *ServerService) LoadLinkHistoryAsync() ([]*database.LinkHistory, error) {
	return database.GetLinkHistory()
}