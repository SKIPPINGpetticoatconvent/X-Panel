package firewall

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"x-ui/logger"
)

// UfwService UFW 防火墙的具体实现
type UfwService struct {
	defaultPorts []int // 默认需要放行的端口列表
}

// NewUfwService 创建新的 UFW 防火墙服务实例
func NewUfwService() *UfwService {
	return &UfwService{
		defaultPorts: []int{22, 80, 443, 13688, 8443}, // 默认端口列表
	}
}

// Name 返回防火墙名称
func (f *UfwService) Name() string {
	return "UFW"
}

// IsRunning 检查防火墙服务是否正在运行
func (f *UfwService) IsRunning() bool {
	cmd := exec.Command("ufw", "status")
	if err := cmd.Run(); err != nil {
		return false
	}
	
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	
	return strings.Contains(string(output), "Status: active")
}

// ensureUfwInstalled 检查并安装 UFW
func (f *UfwService) ensureUfwInstalled() error {
	// 检查 ufw 是否已安装
	cmd := exec.Command("which", "ufw")
	if err := cmd.Run(); err == nil {
		return nil // ufw 已安装
	}
	
	// 安装 ufw
	logger.Info("UFW 防火墙未安装，正在自动安装...")
	
	// 更新包列表
	cmd = exec.Command("bash", "-c", "DEBIAN_FRONTEND=noninteractive /usr/bin/apt-get update -qq")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("更新包列表失败: %v", err)
	}
	
	// 安装 ufw
	cmd = exec.Command("bash", "-c", "DEBIAN_FRONTEND=noninteractive /usr/bin/apt-get install -y -qq ufw")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("安装 UFW 失败: %v", err)
	}
	
	logger.Info("UFW 防火墙安装成功")
	return nil
}

// openDefaultPorts 放行默认端口
func (f *UfwService) openDefaultPorts() error {
	for _, port := range f.defaultPorts {
		if err := f.openSinglePort(port, "tcp"); err != nil {
			return fmt.Errorf("放行默认端口 %d 失败: %v", port, err)
		}
	}
	return nil
}

// openSinglePort 放行单个端口
func (f *UfwService) openSinglePort(port int, protocol string) error {
	if protocol == "" {
		protocol = "tcp"
	}
	
	// 检查规则是否已存在
	cmd := exec.Command("ufw", "status")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("检查端口规则失败: %v", err)
	}
	
	rule := fmt.Sprintf("%d/%s", port, protocol)
	if strings.Contains(string(output), rule) {
		// 规则已存在，跳过
		return nil
	}
	
	// 添加规则
	cmd = exec.Command("ufw", "allow", rule)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("放行端口 %d 失败: %v", port, err)
	}
	
	return nil
}

// openPortWithProtocols 处理端口开放逻辑，支持同时开放 TCP 和 UDP
func (f *UfwService) openPortWithProtocols(port int, protocol string) error {
	// 如果 protocol 为空、"both" 或 "tcp/udp"，则同时开放 TCP 和 UDP
	if protocol == "" || protocol == "both" || protocol == "tcp/udp" {
		// 同时开放 TCP 和 UDP
		if err := f.openSinglePort(port, "tcp"); err != nil {
			return fmt.Errorf("放行 TCP 端口 %d 失败: %v", port, err)
		}
		if err := f.openSinglePort(port, "udp"); err != nil {
			return fmt.Errorf("放行 UDP 端口 %d 失败: %v", port, err)
		}
		logger.Infof("端口 %d 已同时开放 TCP 和 UDP", port)
		return nil
	}
	
	// 只开放指定的协议
	return f.openSinglePort(port, protocol)
}

// activateUfw 激活 UFW 防火墙
func (f *UfwService) activateUfw() error {
	cmd := exec.Command("ufw", "status")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("检查 UFW 状态失败: %v", err)
	}
	
	if !strings.Contains(string(output), "Status: active") {
		// 激活 UFW
		cmd = exec.Command("ufw", "--force", "enable")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("激活 UFW 失败: %v", err)
		}
		logger.Info("UFW 防火墙已激活")
	}
	
	return nil
}

// OpenPort 放行指定端口
func (f *UfwService) OpenPort(port int, protocol string) error {
	// 1. 检查并安装 UFW
	if err := f.ensureUfwInstalled(); err != nil {
		return fmt.Errorf("UFW 安装检查失败: %v", err)
	}
	
	// 2. 放行默认端口
	if err := f.openDefaultPorts(); err != nil {
		logger.Warningf("放行默认端口时出现警告: %v", err)
		// 不返回错误，因为默认端口放行失败不应该影响主要端口的放行
	}
	
	// 3. 放行指定端口
	if err := f.openPortWithProtocols(port, protocol); err != nil {
		return fmt.Errorf("放行端口 %d 失败: %v", port, err)
	}
	
	// 4. 激活防火墙
	if err := f.activateUfw(); err != nil {
		logger.Warningf("UFW 激活时出现警告: %v", err)
		// 不返回错误，因为防火墙激活失败不应该影响端口放行
	}
	
	logger.Infof("端口 %d 已成功放行 (协议: %s)", port, protocol)
	return nil
}

// ClosePort 关闭指定端口
func (f *UfwService) ClosePort(port int, protocol string) error {
	if protocol == "" {
		protocol = "tcp"
	}
	
	cmd := exec.Command("ufw", "delete", "allow", fmt.Sprintf("%d/%s", port, protocol))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("关闭端口 %d 失败: %v", port, err)
	}
	
	logger.Infof("端口 %d 已成功关闭 (协议: %s)", port, protocol)
	return nil
}

// Reload 重载防火墙配置
func (f *UfwService) Reload() error {
	cmd := exec.Command("ufw", "reload")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("重载 UFW 配置失败: %v", err)
	}
	
	logger.Info("UFW 配置已重载")
	return nil
}

// EnsureInstalled 检查并尝试安装防火墙
func (f *UfwService) EnsureInstalled() error {
	return f.ensureUfwInstalled()
}

// OpenPortAsync 异步放行端口，返回是否成功的布尔值和错误信息
func (f *UfwService) OpenPortAsync(port int, protocol string) (bool, error) {
	errChan := make(chan error, 1)
	
	go func() {
		errChan <- f.OpenPort(port, protocol)
	}()
	
	// 等待执行结果，最多等待 2 分钟
	select {
	case err := <-errChan:
		if err != nil {
			logger.Errorf("异步端口放行失败 (端口 %d): %v", port, err)
			return false, err
		}
		return true, nil
	case <-time.After(2 * time.Minute):
		return false, fmt.Errorf("端口放行操作超时")
	}
}