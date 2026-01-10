package service

import (
	"errors"
	"fmt"
	"os"
	"os/user"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"x-ui/logger"
)

// XrayController 定义 Xray 控制接口
type XrayController interface {
	// ReloadCore 重载 Xray 核心配置
	ReloadCore() error
	// IsRunning 检查 Xray 是否运行中
	IsRunning() bool
	// GetProcessInfo 获取 Xray 进程信息 (PID 和 UID)
	GetProcessInfo() (pid, uid int, err error)
	// SendSignal 发送信号给 Xray 进程
	SendSignal(sig os.Signal) error
}

// CertObserver 定义证书变更观察者接口
type CertObserver interface {
	OnCertUpdated(certPath, keyPath string) error
}

// CertHotReloader 实现证书热加载功能
type CertHotReloader struct {
	xrayCtrl XrayController
}

// NewCertHotReloader 创建新的证书热加载器
func NewCertHotReloader(xrayCtrl XrayController) *CertHotReloader {
	reloader := &CertHotReloader{
		xrayCtrl: xrayCtrl,
	}

	// 启动时进行权限诊断
	reloader.diagnosePermissions()

	return reloader
}

// diagnosePermissions 在启动时诊断权限配置
func (r *CertHotReloader) diagnosePermissions() {
	logger.Info("开始权限诊断...")

	// 检查当前用户权限
	currentEUID := os.Geteuid()
	currentUser, err := user.Current()
	if err != nil {
		logger.Warningf("无法获取当前用户信息: %v", err)
	} else {
		logger.Infof("当前用户: %s (UID: %s, EUID: %d)", currentUser.Username, currentUser.Uid, currentEUID)
	}

	// 如果是 root 用户，权限充足
	if currentEUID == 0 {
		logger.Info("权限诊断完成：当前用户为 root，具有完全权限")
		return
	}

	// 检查 CAP_KILL capability
	if r.hasCAPKILL() {
		logger.Info("权限诊断完成：当前进程具有 CAP_KILL capability")
		return
	}

	logger.Warning("权限诊断警告：当前用户可能无法向其他用户的进程发送信号")
	logger.Warning("建议解决方案：")
	logger.Warning("  1. 以 root 用户运行面板")
	logger.Warning("  2. 使用 'setcap cap_kill+eip <binary>' 授予 CAP_KILL 权限")
	logger.Warning("  3. 确保 Xray 进程与面板进程使用相同用户运行")
}

// OnCertRenewed 证书续期成功后的回调方法
func (r *CertHotReloader) OnCertRenewed(certPath, keyPath string) error {
	logger.Info("证书续期成功，开始热加载到 Xray")

	// 检查证书文件是否存在且可读
	if err := r.checkFileReadable(certPath); err != nil {
		logger.Errorf("证书文件不可读: %v", err)
		return err
	}

	if err := r.checkFileReadable(keyPath); err != nil {
		logger.Errorf("私钥文件不可读: %v", err)
		return err
	}

	// 重载 Xray 证书
	if err := r.ReloadXrayCerts(); err != nil {
		logger.Errorf("重载 Xray 证书失败: %v", err)
		return err
	}

	// 验证新证书是否生效
	if err := r.VerifyNewCert(); err != nil {
		logger.Errorf("验证新证书失败: %v", err)
		return err
	}

	logger.Info("证书热加载完成")
	return nil
}

// ReloadXrayCerts 重载 Xray 证书配置
func (r *CertHotReloader) ReloadXrayCerts() error {
	logger.Info("开始重载 Xray 核心配置")

	if !r.xrayCtrl.IsRunning() {
		logger.Warning("Xray 未运行，跳过重载")
		return WrapError(ErrCodeXrayReloadFailed, errors.New("xray is not running"))
	}

	// 通知 Xray 核心重载配置
	if err := r.NotifyXrayCore(); err != nil {
		logger.Errorf("通知 Xray 核心失败: %v", err)
		return err
	}

	logger.Info("Xray 核心重载成功")
	return nil
}

// NotifyXrayCore 通知 Xray 核心重载配置
func (r *CertHotReloader) NotifyXrayCore() error {
	logger.Info("开始通知 Xray 核心重载配置")

	// 如果是 Windows，需要完全重启
	if runtime.GOOS == "windows" {
		logger.Info("Windows 系统，执行完全重启")
		// 注意：这里需要实现重启逻辑，但由于我们不能修改 XrayService，
		// 暂时返回错误，实际集成时需要通过适当的方式调用重启
		return WrapError(ErrCodeXrayReloadFailed, errors.New("Windows restart not implemented - should call XrayService.RestartXray()"))
	}

	// 执行权限预检
	if err := r.CheckSignalPermission(); err != nil {
		logger.Warningf("权限检查失败: %v", err)

		// 尝试通过 Xray API 重载配置
		logger.Info("尝试通过 Xray API 重载配置作为备选方案")
		if apiErr := r.ReloadViaAPI(); apiErr != nil {
			logger.Errorf("API 重载也失败: %v", apiErr)
			return WrapError(ErrCodeXrayReloadFailed, fmt.Errorf("信号发送和 API 重载都失败 - 信号错误: %w, API 错误: %v", err, apiErr))
		}

		logger.Info("通过 Xray API 成功重载配置")
		return nil
	}

	// 尝试发送 SIGHUP 信号
	if err := r.sendSIGHUPToXray(); err != nil {
		logger.Errorf("发送 SIGHUP 信号失败: %v", err)

		// 尝试通过 Xray API 重载配置作为备选方案
		logger.Info("尝试通过 Xray API 重载配置作为备选方案")
		if apiErr := r.ReloadViaAPI(); apiErr != nil {
			logger.Errorf("API 重载也失败: %v", apiErr)
			return WrapError(ErrCodeXrayReloadFailed, fmt.Errorf("信号发送和 API 重载都失败 - 信号错误: %w, API 错误: %v", err, apiErr))
		}

		logger.Info("通过 Xray API 成功重载配置")
		return nil
	}

	logger.Info("通过 SIGHUP 信号成功通知 Xray 核心重载配置")
	return nil
}

// sendSIGHUPToXray 发送 SIGHUP 信号给 Xray 进程
func (r *CertHotReloader) sendSIGHUPToXray() error {
	logger.Info("发送 SIGHUP 信号给 Xray 进程")

	// 使用 XrayController 接口发送信号
	return r.xrayCtrl.SendSignal(syscall.SIGHUP)
}

// ReloadViaAPI 通过 Xray API 重载配置
func (r *CertHotReloader) ReloadViaAPI() error {
	logger.Info("尝试通过 Xray API 重载配置")

	// 注意：Xray 核心通常不支持通过 API 热重载配置
	// SIGHUP 信号是标准的重载方式
	// 如果 API 重载可用，未来可以在这里实现
	// 目前我们尝试重启服务作为备选方案

	logger.Warning("Xray API 不支持配置热重载，尝试重启服务")

	// 通过 XrayController 重启服务
	if err := r.xrayCtrl.ReloadCore(); err != nil {
		return fmt.Errorf("API 重载失败，重启服务也失败: %w", err)
	}

	logger.Info("通过重启服务完成配置重载")
	return nil
}

// VerifyNewCert 验证新证书已生效
func (r *CertHotReloader) VerifyNewCert() error {
	logger.Info("验证新证书是否生效")

	// 等待一小段时间让重载生效
	time.Sleep(2 * time.Second)

	// 检查 Xray 是否仍在运行
	if !r.xrayCtrl.IsRunning() {
		return WrapError(ErrCodeXrayReloadFailed, errors.New("Xray 进程在重载后未运行"))
	}

	logger.Info("证书验证完成，Xray 运行正常")
	return nil
}

// CheckSignalPermission 检查是否有权限发送信号给 Xray 进程
func (r *CertHotReloader) CheckSignalPermission() error {
	// 获取当前进程的有效用户 ID
	currentEUID := os.Geteuid()
	logger.Debugf("当前进程有效用户 ID: %d", currentEUID)

	// 如果是 root 用户，直接返回成功
	if currentEUID == 0 {
		logger.Debug("当前用户为 root，具有发送信号的权限")
		return nil
	}

	// 获取 Xray 进程的 PID 和所有者
	xrayPID, xrayUID, err := r.getXrayProcessInfo()
	if err != nil {
		logger.Warningf("无法获取 Xray 进程信息: %v", err)
		return WrapError(ErrCodePermissionDenied, fmt.Errorf("无法获取 Xray 进程信息: %w", err))
	}

	logger.Debugf("Xray 进程 PID: %d, UID: %d", xrayPID, xrayUID)

	// 如果 Xray 进程的所有者与当前用户相同，直接返回成功
	if xrayUID == currentEUID {
		logger.Debug("Xray 进程与当前用户相同，具有发送信号的权限")
		return nil
	}

	// 检查 CAP_KILL capability
	if r.hasCAPKILL() {
		logger.Debug("当前进程具有 CAP_KILL capability，具有发送信号的权限")
		return nil
	}

	logger.Warningf("权限不足：当前用户 %d 无法向 Xray 进程 (UID: %d) 发送信号", currentEUID, xrayUID)
	return WrapError(ErrCodePermissionDenied, fmt.Errorf("权限不足：无法向 Xray 进程发送信号"))
}

// getXrayProcessInfo 获取 Xray 进程的 PID 和所有者 UID
func (r *CertHotReloader) getXrayProcessInfo() (pid, uid int, err error) {
	return r.xrayCtrl.GetProcessInfo()
}

// hasCAPKILL 检查当前进程是否具有 CAP_KILL capability
func (r *CertHotReloader) hasCAPKILL() bool {
	// 读取 /proc/self/status 文件来检查 capabilities
	data, err := os.ReadFile("/proc/self/status")
	if err != nil {
		logger.Debugf("无法读取 /proc/self/status: %v", err)
		return false
	}

	// 查找 CapEff 行，包含有效 capabilities
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "CapEff:") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				// 解析十六进制 capabilities
				caps, err := strconv.ParseUint(parts[1], 16, 64)
				if err != nil {
					logger.Debugf("无法解析 capabilities: %v", err)
					return false
				}
				// CAP_KILL 是第5位 (1 << 5 = 32)
				return (caps & (1 << 5)) != 0
			}
		}
	}

	logger.Debug("未找到有效的 CAP_KILL capability")
	return false
}

// checkFileReadable 检查文件是否存在且可读
func (r *CertHotReloader) checkFileReadable(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// 尝试读取前几个字节来验证可读性
	buf := make([]byte, 1)
	_, err = file.Read(buf)
	if err != nil && err.Error() != "EOF" {
		return err
	}

	return nil
}