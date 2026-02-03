package service

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
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
	"x-ui/logger"
	"x-ui/util/common"
	"x-ui/xray"
)

// =============================================================================
// Xray 版本管理
// =============================================================================

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

// =============================================================================
// GeoFile 管理
// =============================================================================

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
