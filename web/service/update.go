package service

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"x-ui/logger"
)

type Release struct {
	TagName string `json:"tag_name"`
}

// GetPanelLatestVersion 获取面板的最新版本
func (s *ServerService) GetPanelLatestVersion() (string, error) {
	const (
		XPanelURL    = "https://api.github.com/repos/SKIPPINGpetticoatconvent/X-Panel/releases/latest"
		bufferSize = 8192
	)

	// 使用带超时的HTTP客户端
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// 添加User-Agent头部以避免被GitHub拒绝
	req, err := http.NewRequest("GET", XPanelURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "X-Panel/1.0")

	resp, err := client.Do(req)
	if err != nil {
		logger.Warning("Failed to fetch X-Panel latest version from GitHub:", err)
		return "", fmt.Errorf("无法获取X-Panel最新版本信息，请检查网络连接: %v", err)
	}
	defer resp.Body.Close()

	// 检查HTTP状态码
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API返回错误状态码: %d", resp.StatusCode)
	}

	buffer := bytes.NewBuffer(make([]byte, bufferSize))
	buffer.Reset()
	if _, err := buffer.ReadFrom(resp.Body); err != nil {
		return "", fmt.Errorf("读取响应数据失败: %v", err)
	}

	var release Release
	if err := json.Unmarshal(buffer.Bytes(), &release); err != nil {
		return "", fmt.Errorf("解析JSON响应失败: %v", err)
	}

	logger.Infof("成功获取到X-Panel最新版本: %s", release.TagName)
	return release.TagName, nil
}

// detectPanelArch 检测并返回支持的面板架构
func detectPanelArch() (string, error) {
	// 使用 uname -m 检测系统架构，参考 install.sh 的逻辑
	cmd := exec.Command("uname", "-m")
	output, err := cmd.Output()
	if err == nil {
		systemArch := strings.TrimSpace(string(output))
		switch systemArch {
		case "x86_64", "x64", "amd64":
			return "amd64", nil
		case "i386", "i486", "i586", "i686", "x86":
			return "386", nil
		case "armv8", "armv8l", "arm64", "aarch64":
			return "arm64", nil
		case "armv7", "armv7l", "arm":
			return "armv7", nil
		case "armv6", "armv6l":
			return "armv6", nil
		case "armv5", "armv5l":
			return "armv5", nil
		case "s390x":
			return "s390x", nil
		default:
			// 如果检测到未知架构，回退到 runtime.GOARCH
			logger.Warningf("检测到未知系统架构 %s，使用 runtime.GOARCH: %s", systemArch, runtime.GOARCH)
			return runtime.GOARCH, nil
		}
	}

	// 如果 uname 命令失败，回退到 runtime.GOARCH
	logger.Warning("uname -m 命令失败，使用 runtime.GOARCH:", runtime.GOARCH)
	return runtime.GOARCH, nil
}

// updateXUICommandScript 下载并更新 x-ui.sh 脚本到 /usr/bin/x-ui
func updateXUICommandScript() error {
	scriptURL := "https://raw.githubusercontent.com/SKIPPINGpetticoatconvent/X-Panel/main/x-ui.sh"

	// 下载脚本到临时位置
	tempScript, err := os.CreateTemp("", "x-ui-script-")
	if err != nil {
		return fmt.Errorf("创建临时脚本文件失败: %v", err)
	}
	defer os.Remove(tempScript.Name())
	defer tempScript.Close()

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("GET", scriptURL, nil)
	if err != nil {
		return fmt.Errorf("创建脚本下载请求失败: %v", err)
	}
	req.Header.Set("User-Agent", "X-Panel/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("脚本下载失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("脚本下载失败，状态码: %d", resp.StatusCode)
	}

	_, err = io.Copy(tempScript, resp.Body)
	if err != nil {
		return fmt.Errorf("写入脚本文件失败: %v", err)
	}

	// 先备份现有脚本
	backupPath := "/usr/bin/x-ui.bak"
	if _, err := os.Stat("/usr/bin/x-ui"); err == nil {
		err := exec.Command("cp", "/usr/bin/x-ui", backupPath).Run()
		if err != nil {
			logger.Warningf("备份 x-ui 脚本失败: %v", err)
		} else {
			logger.Info("成功备份 x-ui 脚本")
		}
	}

	// 移动新脚本到 /usr/bin/x-ui
	err = os.Rename(tempScript.Name(), "/usr/bin/x-ui-temp")
	if err != nil {
		return fmt.Errorf("重命名临时脚本失败: %v", err)
	}

	err = exec.Command("mv", "-f", "/usr/bin/x-ui-temp", "/usr/bin/x-ui").Run()
	if err != nil {
		// 如果移动失败，尝试恢复备份
		if _, err2 := os.Stat(backupPath); err2 == nil {
			exec.Command("mv", "-f", backupPath, "/usr/bin/x-ui").Run()
			logger.Warning("脚本更新失败，已恢复备份")
		}
		return fmt.Errorf("更新 x-ui 脚本失败: %v", err)
	}

	// 设置执行权限
	err = os.Chmod("/usr/bin/x-ui", 0755)
	if err != nil {
		return fmt.Errorf("设置脚本执行权限失败: %v", err)
	}

	logger.Info("成功更新 x-ui 脚本")
	return nil
}

// downloadAndExtractPanel 从指定URL下载并解压面板二进制文件
func downloadAndExtractPanel(url string) (string, error) {
	// 创建临时文件用于下载tar.gz
	tempFile, err := os.CreateTemp("", "x-panel-*.tar.gz")
	if err != nil {
		return "", fmt.Errorf("创建临时文件失败: %v", err)
	}
	defer os.Remove(tempFile.Name())

	// 下载文件
	client := &http.Client{Timeout: 120 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %v", err)
	}
	req.Header.Set("User-Agent", "X-Panel/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("下载失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("下载失败，状态码: %d", resp.StatusCode)
	}

	_, err = io.Copy(tempFile, resp.Body)
	if err != nil {
		return "", fmt.Errorf("写入临时文件失败: %v", err)
	}
	tempFile.Close()

	// 解压tar.gz并提取x-ui二进制文件
	file, err := os.Open(tempFile.Name())
	if err != nil {
		return "", fmt.Errorf("打开临时文件失败: %v", err)
	}
	defer file.Close()

	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return "", fmt.Errorf("创建gzip读取器失败: %v", err)
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("读取tar失败: %v", err)
		}

		if header.Name == "x-ui" {
			// 提取二进制文件到临时位置
			tempBin, err := os.CreateTemp("", "x-ui-")
			if err != nil {
				return "", fmt.Errorf("创建临时二进制文件失败: %v", err)
			}

			_, err = io.Copy(tempBin, tarReader)
			tempBin.Close()
			if err != nil {
				os.Remove(tempBin.Name())
				return "", fmt.Errorf("提取二进制文件失败: %v", err)
			}

			return tempBin.Name(), nil
		}
	}

	return "", fmt.Errorf("在tar.gz中未找到x-ui二进制文件")
}

// updateXrayCore 下载并更新 Xray 核心
func updateXrayCore(arch string) error {
	// 从 Xray 官方仓库下载最新版本
	xrayURL := "https://github.com/XTLS/Xray-core/releases/latest/download/Xray-linux-" + arch + ".zip"

	logger.Infof("开始下载 Xray 核心: %s", xrayURL)

	// 下载 Xray
	tempFile, err := os.CreateTemp("", "xray-*.zip")
	if err != nil {
		return fmt.Errorf("创建 Xray 临时文件失败: %v", err)
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	client := &http.Client{Timeout: 120 * time.Second}
	req, err := http.NewRequest("GET", xrayURL, nil)
	if err != nil {
		return fmt.Errorf("创建 Xray 下载请求失败: %v", err)
	}
	req.Header.Set("User-Agent", "X-Panel/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("Xray 下载失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Xray 下载失败，状态码: %d", resp.StatusCode)
	}

	_, err = io.Copy(tempFile, resp.Body)
	if err != nil {
		return fmt.Errorf("写入 Xray 文件失败: %v", err)
	}

	// 解压并安装 Xray
	installDir := "/usr/local/x-ui/bin"
	if _, err := os.Stat(installDir); os.IsNotExist(err) {
		os.MkdirAll(installDir, 0755)
	}

	// 使用 unzip 命令解压 (需要确保 unzip 已安装)
	tempDir, err := os.MkdirTemp("", "xray-extract-")
	if err != nil {
		return fmt.Errorf("创建解压目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cmd := exec.Command("unzip", "-o", tempFile.Name(), "-d", tempDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("解压 Xray 失败: %v, 输出: %s", err, string(output))
	}

	// 查找解压后的 Xray 二进制文件
	var xrayBin string
	entries, err := os.ReadDir(tempDir)
	if err != nil {
		return fmt.Errorf("读取解压目录失败: %v", err)
	}

	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "xray") && !strings.HasSuffix(entry.Name(), ".sig") {
			xrayBin = filepath.Join(tempDir, entry.Name())
			break
		}
	}

	if xrayBin == "" {
		return fmt.Errorf("在解压文件中未找到 Xray 二进制文件")
	}

	// 处理 ARM 架构的文件重命名
	targetName := "xray-linux-" + arch
	if arch == "armv5" || arch == "armv6" || arch == "armv7" {
		targetName = "xray-linux-arm"
	}

	targetPath := filepath.Join(installDir, targetName)
	backupPath := filepath.Join(installDir, targetName+".bak")

	// 备份现有 Xray
	if _, err := os.Stat(targetPath); err == nil {
		err := exec.Command("cp", targetPath, backupPath).Run()
		if err != nil {
			logger.Warningf("备份 Xray 失败: %v", err)
		} else {
			logger.Info("成功备份 Xray 核心")
		}
	}

	// 移动新 Xray 到目标位置
	err = exec.Command("cp", xrayBin, targetPath).Run()
	if err != nil {
		// 恢复备份
		if _, err2 := os.Stat(backupPath); err2 == nil {
			exec.Command("cp", backupPath, targetPath).Run()
			logger.Warning("Xray 更新失败，已恢复备份")
		}
		return fmt.Errorf("更新 Xray 失败: %v", err)
	}

	// 设置执行权限
	err = os.Chmod(targetPath, 0755)
	if err != nil {
		return fmt.Errorf("设置 Xray 执行权限失败: %v", err)
	}

	logger.Info("成功更新 Xray 核心")
	return nil
}

// replacePanelBinary 备份并替换面板二进制文件
func replacePanelBinary(newBinPath string) error {
	installDir := "/usr/local/x-ui/"
	binPath := filepath.Join(installDir, "x-ui")
	bakPath := filepath.Join(installDir, "x-ui.bak")

	// 检查安装目录是否存在
	if _, err := os.Stat(installDir); os.IsNotExist(err) {
		return fmt.Errorf("安装目录不存在: %s", installDir)
	}

	// 备份现有二进制文件
	if _, err := os.Stat(binPath); err == nil {
		err := os.Rename(binPath, bakPath)
		if err != nil {
			return fmt.Errorf("备份现有二进制文件失败: %v", err)
		}
		logger.Info("成功备份现有面板二进制文件")
	}

	// 移动新二进制文件到安装位置
	err := os.Rename(newBinPath, binPath)
	if err != nil {
		// 如果替换失败，尝试恢复备份
		if _, err2 := os.Stat(bakPath); err2 == nil {
			os.Rename(bakPath, binPath)
			logger.Warning("替换失败，已恢复备份文件")
		}
		return fmt.Errorf("替换二进制文件失败: %v", err)
	}

	// 设置执行权限
	err = os.Chmod(binPath, 0755)
	if err != nil {
		return fmt.Errorf("设置执行权限失败: %v", err)
	}

	logger.Info("成功替换面板二进制文件")
	return nil
}

// runMigrationCommand 执行数据库迁移命令
func runMigrationCommand() error {
	cmd := exec.Command("/usr/local/x-ui/x-ui", "migrate")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("执行数据库迁移失败: %v, 输出: %s", err, string(output))
	}
	logger.Info("数据库迁移执行成功")
	return nil
}

// restartPanelService 重启面板服务
func restartPanelService() error {
	cmd := exec.Command("systemctl", "restart", "x-ui")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("重启面板服务失败: %v, 输出: %s", err, string(output))
	}
	logger.Info("成功重启面板服务")
	return nil
}

// UpdatePanel 更新面板到指定版本或最新版本（完全还原脚本逻辑）
func (s *ServerService) UpdatePanel(version string) error {
	// 启动异步任务进行面板更新，避免阻塞HTTP请求
	go func() {
		logger.Infof("开始异步更新X-Panel（完全还原脚本逻辑）")

		// 检查Telegram服务是否可用
		tgAvailable := s.tgService != nil && s.tgService.IsRunning()

		// 1. 发送开始通知
		if tgAvailable {
			startMessage := "🔄 **开始更新 X-Panel**\n\n正在检查最新版本...\n\n⏳ 请稍候，这可能需要几分钟时间..."
			if err := s.tgService.SendMessage(startMessage); err != nil {
				logger.Warningf("发送X-Panel更新开始通知失败: %v", err)
			}
		}

		var updateErr error
		var tempBinPath string
		var detectedArch string

		// 2. 获取版本号（如果未指定）
		if version == "" {
			logger.Info("未指定版本，获取最新版本...")
			latestVersion, err := s.GetPanelLatestVersion()
			if err != nil {
				updateErr = fmt.Errorf("获取最新版本失败: %v", err)
				logger.Errorf("获取最新版本失败: %v", err)
			} else {
				version = latestVersion
				logger.Infof("使用最新版本: %s", version)
			}
		}

		if updateErr == nil {
			// 3. 检测架构
			arch, err := detectPanelArch()
			if err != nil {
				updateErr = fmt.Errorf("架构检测失败: %v", err)
				logger.Errorf("架构检测失败: %v", err)
			} else {
				detectedArch = arch
				logger.Infof("检测到架构: %s", arch)
			}
		}

		if updateErr == nil {
			// 4. 下载并更新 x-ui.sh 脚本
			logger.Info("开始更新 x-ui.sh 脚本...")
			err := updateXUICommandScript()
			if err != nil {
				logger.Warningf("更新 x-ui.sh 脚本失败，将继续其他更新: %v", err)
				// 不设为致命错误，因为脚本更新失败不应该阻止核心更新
			} else {
				logger.Info("x-ui.sh 脚本更新成功")
			}
		}

		if updateErr == nil {
			// 5. 构建面板下载URL并下载解压
			downloadURL := fmt.Sprintf("https://github.com/SKIPPINGpetticoatconvent/X-Panel/releases/download/%s/x-ui-linux-%s.tar.gz", version, detectedArch)
			logger.Infof("下载面板URL: %s", downloadURL)

			var err error
			tempBinPath, err = downloadAndExtractPanel(downloadURL)
			if err != nil {
				updateErr = fmt.Errorf("下载并解压面板失败: %v", err)
				logger.Errorf("下载并解压面板失败: %v", err)
			} else {
				logger.Info("成功下载并解压面板二进制文件")
			}
		}

		if updateErr == nil {
			// 6. 更新 Xray 核心
			logger.Info("开始更新 Xray 核心...")
			err := updateXrayCore(detectedArch)
			if err != nil {
				logger.Warningf("更新 Xray 核心失败，继续其他步骤: %v", err)
				// 不设为致命错误，让更新继续
			} else {
				logger.Info("Xray 核心更新成功")
			}
		}

		if updateErr == nil {
			// 7. 备份并替换面板二进制文件 (热替换)
			err := replacePanelBinary(tempBinPath)
			if err != nil {
				updateErr = fmt.Errorf("替换面板二进制文件失败: %v", err)
				logger.Errorf("替换面板二进制文件失败: %v", err)
			}
		}

		if updateErr == nil {
			// 8. 执行数据库迁移
			logger.Info("执行数据库迁移...")
			err := runMigrationCommand()
			if err != nil {
				logger.Warningf("执行数据库迁移失败，继续其他步骤: %v", err)
				// 不设为致命错误，因为新版本可能自动迁移
			} else {
				logger.Info("数据库迁移执行成功")
			}
		}

		if updateErr == nil {
			// 9. 重新加载 systemd 配置并重启服务
			logger.Info("重新加载 systemd 配置并重启面板服务...")
			cmd := exec.Command("systemctl", "daemon-reload")
			output, err := cmd.CombinedOutput()
			if err != nil {
				logger.Warningf("重新加载 systemd 失败: %v, 输出: %s", err, string(output))
			}

			cmd = exec.Command("systemctl", "restart", "x-ui")
			output, err = cmd.CombinedOutput()
			if err != nil {
				updateErr = fmt.Errorf("重启面板服务失败: %v, 输出: %s", err, string(output))
				logger.Errorf("重启面板服务失败: %v, 输出: %s", err, string(output))
			} else {
				logger.Info("面板服务重启成功")
			}

			// 停止其他可能的服务
			exec.Command("systemctl", "stop", "warp-go").Run()
			exec.Command("wg-quick", "down", "wgcf").Run()
		}

		// 清理临时文件
		if tempBinPath != "" {
			os.Remove(tempBinPath)
		}

		// 11. 发送结果通知
		if tgAvailable {
			if updateErr == nil {
				// 更新成功通知
				successMessage := fmt.Sprintf("🎉 **X-Panel 更新成功！**\n\n版本: `%s`\n✅ 脚本已更新\n✅ 面板二进制已替换\n✅ Xray 核心已更新\n🔄 服务已成功重启\n✨ 感谢您的耐心等待", version)
				if err := s.tgService.SendMessage(successMessage); err != nil {
					logger.Warningf("发送X-Panel更新成功通知失败: %v", err)
				}
			} else {
				// 更新失败通知
				failMessage := fmt.Sprintf("❌ **X-Panel 更新失败**\n\n版本: `%s`\n错误信息: %v\n\n请检查日志以获取更多信息。", version, updateErr)
				if err := s.tgService.SendMessage(failMessage); err != nil {
					logger.Warningf("发送X-Panel更新失败通知失败: %v", err)
				}
			}
		}

		if updateErr != nil {
			logger.Errorf("X-Panel更新失败: %v", updateErr)
		} else {
			logger.Infof("X-Panel更新成功，版本: %s", version)
		}
	}()

	return nil
}