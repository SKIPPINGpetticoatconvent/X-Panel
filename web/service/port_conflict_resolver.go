package service

import (
	"context"
	"fmt"
	"net"
	"time"

	"x-ui/logger"
)

// PortStatus 表示端口状态
type PortStatus struct {
	Occupied     bool
	OwnedByPanel bool
}

// ProcessInfo 表示进程信息
type ProcessInfo struct {
	PID  int
	Name string
	Cmd  string
}

// PortManager 定义端口管理接口
type PortManager interface {
	// CheckPort80 检查 80 端口状态
	CheckPort80() (bool, bool, error)

	// AcquirePort80 尝试获取 80 端口控制权
	AcquirePort80(ctx context.Context) error

	// ReleasePort80 释放 80 端口控制权
	ReleasePort80() error
}

// WebServerController 定义 Web 服务器控制接口
type WebServerController interface {
	// PauseHTTPListener 暂停 80 端口监听
	PauseHTTPListener() error
	// ResumeHTTPListener 恢复 80 端口监听
	ResumeHTTPListener() error
	// IsListeningOnPort80 检查当前配置是否启用了 80 端口监听
	IsListeningOnPort80() bool
}

// PortConflictResolver 实现端口冲突自愈逻辑
type PortConflictResolver struct {
	webController WebServerController
}

// NewPortConflictResolver 创建新的端口冲突解决器
func NewPortConflictResolver(webController WebServerController) *PortConflictResolver {
	return &PortConflictResolver{
		webController: webController,
	}
}

// CheckPort80 检查 80 端口状态
// 返回: occupied (bool), ownedByPanel (bool), err (error)
func (p *PortConflictResolver) CheckPort80() (occupied bool, ownedByPanel bool, err error) {
	// 1. 尝试建立 TCP 连接检测占用
	conn, err := net.DialTimeout("tcp", ":80", 1*time.Second)
	if err == nil {
		conn.Close()
		occupied = true
	} else {
		// 连接失败通常意味着端口未被监听，或者被防火墙拦截
		// 需进一步区分是 "connection refused" (未占用) 还是其他错误
		if isConnectionRefused(err) {
			occupied = false
		} else {
			// 可能是超时或其他网络问题，视为占用以防万一
			occupied = true
		}
	}

	// 2. 检查面板自身配置
	if p.webController.IsListeningOnPort80() {
		ownedByPanel = true
	} else {
		ownedByPanel = false
	}

	// 修正逻辑：如果物理检测未占用，但配置显示占用（可能刚启动未绑定），以配置为准
	// 如果物理检测占用，但配置显示未占用，则是外部占用
	if occupied && !ownedByPanel {
		return true, false, nil // 外部占用
	}
	if ownedByPanel {
		return true, true, nil // 面板占用 (逻辑上)
	}

	return false, false, nil
}

// AcquirePort80 尝试获取 80 端口控制权
func (p *PortConflictResolver) AcquirePort80(ctx context.Context) error {
	occupied, ownedByPanel, err := p.CheckPort80()
	if err != nil {
		return err
	}

	if !occupied {
		return nil // 端口空闲，直接可用
	}

	if !ownedByPanel {
		return WrapError(ErrCodePort80External, nil)
	}

	// 面板占用，执行暂停
	logger.Info("Pausing panel HTTP listener on port 80...")
	if err := p.webController.PauseHTTPListener(); err != nil {
		return fmt.Errorf("Failed to pause HTTP listener: %w", err)
	}

	// 双重检查：暂停后端口是否真的释放了？
	for i := 0; i < 5; i++ {
		time.Sleep(100 * time.Millisecond)
		occupied, _, _ := p.CheckPort80()
		if !occupied {
			return nil
		}
	}

	// 如果暂停后仍被占用，尝试恢复并报错
	p.webController.ResumeHTTPListener()
	return WrapError(ErrCodePort80Occupied, nil)
}

// ReleasePort80 释放 80 端口控制权
func (p *PortConflictResolver) ReleasePort80() error {
	if p.webController.IsListeningOnPort80() {
		logger.Info("Resuming panel HTTP listener on port 80...")
		return p.webController.ResumeHTTPListener()
	}
	return nil
}

// isConnectionRefused 检查错误是否为连接被拒绝
func isConnectionRefused(err error) bool {
	if err == nil {
		return false
	}
	// 检查网络错误类型
	if netErr, ok := err.(net.Error); ok {
		if opErr, ok := netErr.(*net.OpError); ok {
			if syscallErr, ok := opErr.Err.(*net.OpError); ok {
				// 检查是否为 "connection refused"
				return syscallErr.Err.Error() == "connect: connection refused"
			}
		}
	}
	return false
}

// IdentifyOccupier 识别占用指定端口的进程 (当前未实现)
func (p *PortConflictResolver) IdentifyOccupier(port int) (*ProcessInfo, error) {
	// TODO: 实现端口占用者识别逻辑
	// 这需要使用系统特定的命令如 lsof, netstat 等
	// 当前返回未实现错误
	return nil, fmt.Errorf("IdentifyOccupier not implemented yet")
}

// TemporarilyTakeOver80 临时接管 80 端口 (当前未实现)
func (p *PortConflictResolver) TemporarilyTakeOver80() error {
	// TODO: 实现临时接管逻辑
	// 这需要与 certmagic 集成
	return fmt.Errorf("TemporarilyTakeOver80 not implemented yet")
}
