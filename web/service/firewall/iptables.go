package firewall

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"x-ui/logger"
)

// IptablesService iptables 防火墙的具体实现（兜底方案）
type IptablesService struct {
	defaultPorts []int // 默认需要放行的端口列表
}

// NewIptablesService 创建新的 iptables 防火墙服务实例
func NewIptablesService() *IptablesService {
	return &IptablesService{
		defaultPorts: []int{22, 80, 443, 13688, 8443}, // 默认端口列表
	}
}

// Name 返回防火墙名称
func (f *IptablesService) Name() string {
	return "iptables"
}

// IsRunning 检查防火墙服务是否正在运行
func (f *IptablesService) IsRunning() bool {
	// 检查 iptables -L 是否能成功执行
	cmd := exec.Command("iptables", "-L")
	if err := cmd.Run(); err != nil {
		return false
	}
	
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	
	// 检查输出是否包含有效的链信息
	outputStr := strings.TrimSpace(string(output))
	return strings.Contains(outputStr, "Chain") && len(outputStr) > 0
}

// ensureIptablesAvailable 检查 iptables 是否可用
func (f *IptablesService) ensureIptablesAvailable() error {
	// 检查 iptables 是否已安装
	cmd := exec.Command("which", "iptables")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("iptables 未安装，请手动安装: %v", err)
	}
	
	// 检查 iptables 权限
	cmd = exec.Command("iptables", "--version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("iptables 权限不足，请确保以 root 权限运行: %v", err)
	}
	
	return nil
}

// openDefaultPorts 放行默认端口
func (f *IptablesService) openDefaultPorts() error {
	for _, port := range f.defaultPorts {
		if err := f.openSinglePort(port, ProtocolTCP); err != nil {
			return fmt.Errorf("放行默认端口 %d 失败: %v", port, err)
		}
	}
	return nil
}

// openSinglePort 放行单个端口
func (f *IptablesService) openSinglePort(port int, protocol string) error {
	if protocol == "" {
		protocol = ProtocolTCP
	}
	
	// 检查规则是否已存在
	cmd := exec.Command("iptables", "-C", "INPUT", "-p", protocol, "--dport", fmt.Sprintf("%d", port), "-j", "ACCEPT")
	if err := cmd.Run(); err == nil {
		// 规则已存在，跳过
		return nil
	}
	
	// 添加规则
	cmd = exec.Command("iptables", "-A", "INPUT", "-p", protocol, "--dport", fmt.Sprintf("%d", port), "-j", "ACCEPT")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("放行端口 %d 失败: %v", port, err)
	}
	
	// 保存规则（仅在支持的系统中）
	f.saveRules()
	
	return nil
}

// openPortWithProtocols 处理端口开放逻辑，支持同时开放 TCP 和 UDP
func (f *IptablesService) openPortWithProtocols(port int, protocol string) error {
	// 如果 protocol 为空、"both" 或 "tcp/udp"，则同时开放 TCP 和 UDP
	if protocol == "" || protocol == "both" || protocol == "tcp/udp" {
		// 同时开放 TCP 和 UDP
		if err := f.openSinglePort(port, ProtocolTCP); err != nil {
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

// saveRules 保存 iptables 规则
func (f *IptablesService) saveRules() {
	// 尝试多种保存方法
	saveCommands := [][]string{
		{"iptables-save"},
		{"sh", "-c", "iptables-save > /etc/iptables/rules.v4"}, // Debian/Ubuntu
		{"sh", "-c", "service iptables save"},                  // CentOS/RHEL 传统
		{"sh", "-c", "netfilter-persistent save"},              // Debian 持久化
	}
	
	for _, cmd := range saveCommands {
		cmd := exec.Command(cmd[0], cmd[1:]...)
		if err := cmd.Run(); err == nil {
			break // 成功保存，退出
		}
	}
}

// OpenPort 放行指定端口
func (f *IptablesService) OpenPort(port int, protocol string) error {
	// 1. 检查 iptables 是否可用
	if err := f.ensureIptablesAvailable(); err != nil {
		return fmt.Errorf("iptables 可用性检查失败: %v", err)
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
	
	logger.Infof("端口 %d 已成功放行 (协议: %s)", port, protocol)
	return nil
}

// ClosePort 关闭指定端口
func (f *IptablesService) ClosePort(port int, protocol string) error {
	if protocol == "" {
		protocol = ProtocolTCP
	}
	
	// 移除规则
	cmd := exec.Command("iptables", "-D", "INPUT", "-p", protocol, "--dport", fmt.Sprintf("%d", port), "-j", "ACCEPT")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("关闭端口 %d 失败: %v", port, err)
	}
	
	// 保存规则
	f.saveRules()
	
	logger.Infof("端口 %d 已成功关闭 (协议: %s)", port, protocol)
	return nil
}

// Reload 重载防火墙配置
func (f *IptablesService) Reload() error {
	// iptables 不需要重载，规则立即生效
	// 但我们可以尝试从配置文件重新加载
	
	loadCommands := [][]string{
		{"iptables-restore", "<", "/etc/iptables/rules.v4"}, // Debian/Ubuntu
		{"sh", "-c", "iptables-restore < /etc/sysconfig/iptables"}, // CentOS/RHEL
	}
	
	for _, cmd := range loadCommands {
		cmd := exec.Command(cmd[0], cmd[1:]...)
		if err := cmd.Run(); err == nil {
			logger.Info("iptables 配置已重载")
			return nil
		}
	}
	
	logger.Info("iptables 规则已刷新")
	return nil
}

// EnsureInstalled 检查并尝试安装防火墙
func (f *IptablesService) EnsureInstalled() error {
	return f.ensureIptablesAvailable()
}

// OpenPortAsync 异步放行端口，返回是否成功的布尔值和错误信息
func (f *IptablesService) OpenPortAsync(port int, protocol string) (bool, error) {
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