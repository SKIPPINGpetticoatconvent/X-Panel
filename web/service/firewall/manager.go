package firewall

import (
	"fmt"
	"os/exec"
	"strings"

	"x-ui/logger"
)

// NewFirewallService 根据系统环境自动返回合适的防火墙实现
func NewFirewallService() (FirewallService, error) {
	// 优先级：Firewalld > UFW
	firewallServices := []struct {
		name     string
		create   func() FirewallService
		isRunning func() bool
	}{
		{
			name:     "Firewalld",
			create:   func() FirewallService { return NewFirewalldService() },
			isRunning: func() bool { return NewFirewalldService().IsRunning() },
		},
		{
			name:     "UFW",
			create:   func() FirewallService { return NewUfwService() },
			isRunning: func() bool { return NewUfwService().IsRunning() },
		},
	}

	// 第一阶段：检查正在运行的服务
	for _, service := range firewallServices {
		if service.isRunning() {
			logger.Infof("检测到正在运行的防火墙服务: %s", service.name)
			return service.create(), nil
		}
	}

	// 第二阶段：如果没有服务正在运行，检查命令是否存在
	// 优先级：Firewalld > UFW
	if isCommandAvailable("firewall-cmd") {
		logger.Infof("检测到系统防火墙: Firewalld")
		return NewFirewalldService(), nil
	}
	
	if isCommandAvailable("ufw") {
		logger.Infof("检测到系统防火墙: UFW")
		return NewUfwService(), nil
	}

	// 如果都没有找到，返回错误
	return nil, fmt.Errorf("未检测到任何支持的防火墙服务 (Firewalld 或 UFW)")
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