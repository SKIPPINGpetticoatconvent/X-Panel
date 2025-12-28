package integration

import (
	"fmt"
	"os/exec"
	"time"
)

// ContainerManager 容器管理器
type ContainerManager struct {
	composeFile string
}

// NewContainerManager 创建新的容器管理器
func NewContainerManager(composeFile string) *ContainerManager {
	return &ContainerManager{
		composeFile: composeFile,
	}
}

// StartContainers 启动容器
func (cm *ContainerManager) StartContainers() error {
	cmd := exec.Command("docker-compose", "-f", cm.composeFile, "up", "-d")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("启动容器失败: %v, 输出: %s", err, string(output))
	}
	return nil
}

// StopContainers 停止容器
func (cm *ContainerManager) StopContainers() error {
	cmd := exec.Command("docker-compose", "-f", cm.composeFile, "down")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("停止容器失败: %v, 输出: %s", err, string(output))
	}
	return nil
}

// WaitForContainers 等待容器启动
func (cm *ContainerManager) WaitForContainers(timeout time.Duration, healthCheck func() bool) error {
	timeoutChan := time.After(timeout)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeoutChan:
			return fmt.Errorf("容器启动超时")
		case <-ticker.C:
			if healthCheck() {
				return nil
			}
		}
	}
}

// GetContainerLogs 获取容器日志
func (cm *ContainerManager) GetContainerLogs(serviceName string) (string, error) {
	cmd := exec.Command("docker-compose", "-f", cm.composeFile, "logs", serviceName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("获取容器日志失败: %v", err)
	}
	return string(output), nil
}