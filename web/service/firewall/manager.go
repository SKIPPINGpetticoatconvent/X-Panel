package firewall

import (
	"os/exec"
	"strings"

	"x-ui/logger"
)

// NewFirewallService 根据系统环境自动返回合适的防火墙实现
func NewFirewallService() (FirewallService, error) {
	// 优先级：Firewalld > UFW > iptables
	firewallServices := []struct {
		name     string
		checkCmd string
		create   func() FirewallService
	}{
		{
			name:     "Firewalld",
			checkCmd: "firewall-cmd",
			create:   func() FirewallService { return NewFirewalldService() },
		},
		{
			name:     "UFW",
			checkCmd: "ufw",
			create:   func() FirewallService { return NewUfwService() },
		},
		{
			name:     "iptables",
			checkCmd: "iptables",
			create:   func() FirewallService { return NewIptablesService() },
		},
	}

	for _, service := range firewallServices {
		if isCommandAvailable(service.checkCmd) {
			logger.Infof("检测到系统防火墙: %s", service.name)
			return service.create(), nil
		}
	}

	// 如果都没有找到，返回一个模拟的iptables服务作为兜底
	logger.Warning("未检测到任何支持的防火墙服务，使用 iptables 作为兜底方案")
	return NewIptablesService(), nil
}

// isCommandAvailable 检查系统命令是否存在
func isCommandAvailable(cmd string) bool {
	// 检查命令是否存在
	checkCmd := exec.Command("which", cmd)
	if err := checkCmd.Run(); err != nil {
		return false
	}

	// 验证命令确实可用
	output, err := checkCmd.Output()
	if err != nil {
		return false
	}

	// 检查输出是否包含有效的路径
	return len(strings.TrimSpace(string(output))) > 0
}

// GetAvailableFirewalls 获取所有可用的防火墙列表
func GetAvailableFirewalls() []string {
	var available []string
	
	firewallServices := []struct {
		name     string
		checkCmd string
	}{
		{"Firewalld", "firewall-cmd"},
		{"UFW", "ufw"},
		{"iptables", "iptables"},
	}

	for _, service := range firewallServices {
		if isCommandAvailable(service.checkCmd) {
			available = append(available, service.name)
		}
	}

	return available
}

// RecommendFirewall 推荐最适合的防火墙
func RecommendFirewall() string {
	available := GetAvailableFirewalls()
	
	// 优先级推荐
	for _, priority := range []string{"Firewalld", "UFW", "iptables"} {
		for _, availableFirewall := range available {
			if availableFirewall == priority {
				return availableFirewall
			}
		}
	}
	
	return "iptables" // 默认推荐
}