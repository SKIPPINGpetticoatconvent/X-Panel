package firewall

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"x-ui/logger"
)

// FirewalldService Firewalld 防火墙的具体实现
type FirewalldService struct {
	defaultPorts []int // 默认需要放行的端口列表
}

// NewFirewalldService 创建新的 Firewalld 防火墙服务实例
func NewFirewalldService() *FirewalldService {
	return &FirewalldService{
		defaultPorts: []int{22, 80, 443, 13688, 8443}, // 默认端口列表
	}
}

// Name 返回防火墙名称
func (f *FirewalldService) Name() string {
	return "Firewalld"
}

// IsRunning 检查防火墙服务是否正在运行
func (f *FirewalldService) IsRunning() bool {
	cmd := exec.Command("firewall-cmd", "--state")
	if err := cmd.Run(); err != nil {
		return false
	}
	
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	
	// 检查输出是否包含 "running" 状态
	outputStr := strings.TrimSpace(string(output))
	return strings.Contains(outputStr, "running")
}

// ensureFirewalldInstalled 检查并安装 Firewalld
func (f *FirewalldService) ensureFirewalldInstalled() error {
	// 检查 firewall-cmd 是否已安装
	cmd := exec.Command("which", "firewall-cmd")
	if err := cmd.Run(); err == nil {
		return nil // firewall-cmd 已安装
	}
	
	// 安装 firewalld
	logger.Info("Firewalld 防火墙未安装，正在自动安装...")
	
	// 检测操作系统类型
	if _, err := exec.Command("which", "apt-get").Output(); err == nil {
		// Debian/Ubuntu
		cmd = exec.Command("bash", "-c", "DEBIAN_FRONTEND=noninteractive /usr/bin/apt-get update -qq")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("更新包列表失败: %v", err)
		}
		
		cmd = exec.Command("bash", "-c", "DEBIAN_FRONTEND=noninteractive /usr/bin/apt-get install -y -qq firewalld")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("安装 Firewalld 失败: %v", err)
		}
	} else if _, err := exec.Command("which", "yum").Output(); err == nil {
		// CentOS/RHEL
		cmd = exec.Command("bash", "-c", "/usr/bin/yum install -y firewalld")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("安装 Firewalld 失败: %v", err)
		}
	} else if _, err := exec.Command("which", "dnf").Output(); err == nil {
		// Fedora
		cmd = exec.Command("bash", "-c", "/usr/bin/dnf install -y firewalld")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("安装 Firewalld 失败: %v", err)
		}
	} else {
		return fmt.Errorf("未识别的包管理器，无法自动安装 Firewalld")
	}
	
	// 启动并启用 firewalld 服务
	cmd = exec.Command("systemctl", "enable", "--now", "firewalld")
	if err := cmd.Run(); err != nil {
		logger.Warningf("启动 Firewalld 服务失败: %v", err)
	}
	
	logger.Info("Firewalld 防火墙安装并启动成功")
	return nil
}

// openDefaultPorts 放行默认端口
func (f *FirewalldService) openDefaultPorts() error {
	for _, port := range f.defaultPorts {
		if err := f.openSinglePort(port, "tcp"); err != nil {
			return fmt.Errorf("放行默认端口 %d 失败: %v", port, err)
		}
	}
	return nil
}

// openSinglePort 放行单个端口
func (f *FirewalldService) openSinglePort(port int, protocol string) error {
	if protocol == "" {
		protocol = ProtocolTCP
	}
	
	// 检查端口是否已开放
	cmd := exec.Command("firewall-cmd", "--permanent", "--query-port", fmt.Sprintf("%d/%s", port, protocol))
	if err := cmd.Run(); err == nil {
		// 端口已开放，跳过
		return nil
	}
	
	// 添加端口规则
	cmd = exec.Command("firewall-cmd", "--permanent", "--add-port", fmt.Sprintf("%d/%s", port, protocol))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("放行端口 %d 失败: %v", port, err)
	}
	
	// 重新加载配置使更改生效
	cmd = exec.Command("firewall-cmd", "--reload")
	if err := cmd.Run(); err != nil {
		logger.Warningf("重载 Firewalld 配置失败: %v", err)
	}
	
	return nil
}

// openPortWithProtocols 处理端口开放逻辑，支持同时开放 TCP 和 UDP
func (f *FirewalldService) openPortWithProtocols(port int, protocol string) error {
	// 如果 protocol 为空、"both" 或 "tcp/udp"，则同时开放 TCP 和 UDP
	if protocol == "" || protocol == "both" || protocol == "tcp/udp" {
		// 同时开放 TCP 和 UDP
		if err := f.openSinglePort(port, "tcp"); err != nil {
			return fmt.Errorf("放行 TCP 端口 %d 失败: %v", port, err)
		}
		if err := f.openSinglePort(port, ProtocolUDP); err != nil {
			return fmt.Errorf("放行 UDP 端口 %d 失败: %v", port, err)
		}
		logger.Infof("端口 %d 已同时开放 TCP 和 UDP", port)
		return nil
	}
	
	// 只开放指定的协议
	return f.openSinglePort(port, protocol)
}

// OpenPort 放行指定端口
func (f *FirewalldService) OpenPort(port int, protocol string) error {
	// 1. 检查并安装 Firewalld
	if err := f.ensureFirewalldInstalled(); err != nil {
		return fmt.Errorf("Firewalld 安装检查失败: %v", err)
	}
	
	// 2. 检查服务是否运行
	if !f.IsRunning() {
		// 尝试启动 firewalld 服务
		cmd := exec.Command("systemctl", "start", "firewalld")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("启动 Firewalld 服务失败: %v", err)
		}
		
		// 等待服务启动
		for i := 0; i < 10; i++ {
			if f.IsRunning() {
				break
			}
			time.Sleep(1 * time.Second)
		}
		
		if !f.IsRunning() {
			return fmt.Errorf("Firewalld 服务启动超时")
		}
	}
	
	// 3. 放行默认端口
	if err := f.openDefaultPorts(); err != nil {
		logger.Warningf("放行默认端口时出现警告: %v", err)
	}
	
	// 4. 放行指定端口
	if err := f.openPortWithProtocols(port, protocol); err != nil {
		return fmt.Errorf("放行端口 %d 失败: %v", port, err)
	}
	
	logger.Infof("端口 %d 已成功放行 (协议: %s)", port, protocol)
	return nil
}

// ClosePort 关闭指定端口
func (f *FirewalldService) ClosePort(port int, protocol string) error {
	if protocol == "" {
		protocol = ProtocolTCP
	}
	
	// 移除端口规则
	cmd := exec.Command("firewall-cmd", "--permanent", "--remove-port", fmt.Sprintf("%d/%s", port, protocol))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("关闭端口 %d 失败: %v", port, err)
	}
	
	// 重新加载配置使更改生效
	cmd = exec.Command("firewall-cmd", "--reload")
	if err := cmd.Run(); err != nil {
		logger.Warningf("重载 Firewalld 配置失败: %v", err)
	}
	
	logger.Infof("端口 %d 已成功关闭 (协议: %s)", port, protocol)
	return nil
}

// Reload 重载防火墙配置
func (f *FirewalldService) Reload() error {
	cmd := exec.Command("firewall-cmd", "--reload")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("重载 Firewalld 配置失败: %v", err)
	}
	
	logger.Info("Firewalld 配置已重载")
	return nil
}

// EnsureInstalled 检查并尝试安装防火墙
func (f *FirewalldService) EnsureInstalled() error {
	return f.ensureFirewalldInstalled()
}

// OpenPortAsync 异步放行端口，返回是否成功的布尔值和错误信息
func (f *FirewalldService) OpenPortAsync(port int, protocol string) (bool, error) {
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