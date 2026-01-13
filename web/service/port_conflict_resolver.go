package service

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"syscall"
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
		} else if isTimeout(err) {
			// 如果是本地检测出现超时，大概率是防火墙 DROP 了，或者没有服务在监听但也没有 REJECT
			// 在本地环回接口上，如果端口被占用，connect 应该是立即成功
			// 如果端口未被占用，通常是立即 connection refused
			// 如果超时，通常是防火墙规则导致的，但也可能意味着端口实际上是不可达的（即未被正常服务占用，或者服务不可达）
			// 对于 ACME 挑战，如果本地都连不上（超时），那外网大概率也连不上，所以这里视为“未被面板占用，但也无法使用”
			// 但我们更关心的是“是否有进程在占用该端口”。
			// 如果超时，我们无法确定是否有进程。
			// 策略：尝试检测 IPv6 或认为它是被防火墙屏蔽的空闲端口 (风险：万一是有个僵死进程?)
			// 保守策略：认为是占用的。但为了解决用户的 i/o timeout 问题，我们可以尝试更激进的策略。
			// 如果是 127.0.0.1 超时，很可能是防火墙 DROP。
			// 我们可以尝试 bind 一下端口来验证是否真的被占用。
			// 我们可以尝试 bind 一下端口来验证是否真的被占用。
			if err := canBind(80); err == nil {
				// 能够绑定，说明端口是空闲的
				occupied = false
			} else {
				// 绑定失败，记录具体错误以便排查
				// 如果是地址被占用，则确认被占用
				// 如果是权限不足，也视为被占用（无法使用） but log it
				logger.Warningf("Port 80 check timeout, and bind failed: %v", err)
				occupied = true
			}
		} else {
			// 其他错误，视为占用以防万一
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
	logger.Info("Checking port 80 status...")
	occupied, ownedByPanel, err := p.CheckPort80()
	if err != nil {
		logger.Errorf("Failed to check port 80 status: %v", err)
		return err
	}

	logger.Infof("Port 80 status - Occupied: %v, Owned by panel: %v", occupied, ownedByPanel)

	if !occupied {
		logger.Info("Port 80 is free, proceeding with ACME challenge")
		return nil // 端口空闲，直接可用
	}

	if !ownedByPanel {
		logger.Error("Port 80 is occupied by external process - this will prevent ACME HTTP-01 challenge")
		logger.Error("Check if another web server is running on port 80")
		return WrapError(ErrCodePort80External, nil)
	}

	// 面板占用，执行暂停
	logger.Info("Panel owns port 80, pausing panel HTTP listener...")
	if err := p.webController.PauseHTTPListener(); err != nil {
		logger.Errorf("Failed to pause HTTP listener: %v", err)
		return fmt.Errorf("Failed to pause HTTP listener: %w", err)
	}

	// 双重检查：暂停后端口是否真的释放了？
	logger.Info("Verifying port 80 is released after pausing panel...")
	for i := 0; i < 5; i++ {
		time.Sleep(100 * time.Millisecond)
		occupied, _, _ := p.CheckPort80()
		if !occupied {
			logger.Info("Port 80 successfully released")
			return nil
		}
	}

	// 如果暂停后仍被占用，尝试恢复并报错
	logger.Error("Port 80 still occupied after pausing panel - this indicates the panel failed to release the port")
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
	// 使用 errors.Is 检查是否为 ECONNREFUSED
	if errors.Is(err, syscall.ECONNREFUSED) {
		return true
	}

	// 遍历错误链
	for {
		// 检查是否为 "connection refused" 字符串错误
		if strings.Contains(err.Error(), "connection refused") {
			return true
		}

		// 尝试解包
		if unwravable, ok := err.(interface{ Unwrap() error }); ok {
			err = unwravable.Unwrap()
			if err == nil {
				break
			}
		} else {
			break
		}
	}
	return false
}

// isTimeout 检查是否为超时错误
func isTimeout(err error) bool {
	if err == nil {
		return false
	}
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		return true
	}
	return strings.Contains(err.Error(), "i/o timeout")
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

// canBind 尝试绑定端口以验证是否被占用
func canBind(port int) error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return err
	}
	listener.Close()
	return nil
}
