# X-Panel IP 证书功能改进详细规格说明书

## 1. 引言

本文档基于 `docs/arch/ip_cert_improvements_arch.md` 架构设计，详细定义了 IP 证书功能改进的四个核心模块的功能规格、接口定义和伪代码实现。

**目标读者**: 开发人员、测试人员。
**文档状态**: 草稿 (Draft)

---

## 2. 模块规格与伪代码

### 2.1 端口冲突自愈模块 (PortConflictResolver)

#### 2.1.1 功能规格
该模块负责在申请/续期证书前确保 80 端口可用。
1.  **检测**: 检查 80 端口是否被占用。
2.  **识别**: 区分占用者是 X-Panel 自身还是外部进程。
3.  **接管**: 如果是 X-Panel 占用，临时释放端口供 `certmagic` 使用，验证完成后恢复。
4.  **避让**: 如果是外部占用，返回特定错误，不强行杀进程。

#### 2.1.2 接口定义 (Go)

```go
package service

import "context"

// PortManager 定义端口管理接口
type PortManager interface {
    // CheckPort80 检查 80 端口状态
    // 返回: occupied (bool), ownedByPanel (bool), err (error)
    CheckPort80() (bool, bool, error)

    // AcquirePort80 尝试获取 80 端口控制权
    // 如果是面板占用，则暂停面板的 HTTP 监听
    AcquirePort80(ctx context.Context) error

    // ReleasePort80 释放 80 端口控制权
    // 恢复面板的 HTTP 监听
    ReleasePort80() error
}

// WebServerController 定义 Web 服务器控制接口 (由 Panel 实现)
type WebServerController interface {
    // PauseHTTPListener 暂停 80 端口监听
    PauseHTTPListener() error
    // ResumeHTTPListener 恢复 80 端口监听
    ResumeHTTPListener() error
    // IsListeningOnPort80 检查当前配置是否启用了 80 端口监听
    IsListeningOnPort80() bool
}
```

#### 2.1.3 伪代码与逻辑

```go
// PortConflictResolver 实现 PortManager
type PortConflictResolver struct {
    webController WebServerController
    logger        Logger
}

// TDD: Test_CheckPort80_OccupiedByExternal
// TDD: Test_CheckPort80_OccupiedByPanel
// TDD: Test_CheckPort80_Free
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
            // 可能是超时或其他网络问题，视为占用以防万一，或者根据具体错误判断
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

// TDD: Test_AcquirePort80_Success
// TDD: Test_AcquirePort80_ExternalConflict
func (p *PortConflictResolver) AcquirePort80(ctx context.Context) error {
    occupied, ownedByPanel, err := p.CheckPort80()
    if err != nil {
        return err
    }

    if !occupied {
        return nil // 端口空闲，直接可用
    }

    if !ownedByPanel {
        return NewError("Port 80 is occupied by an external process")
    }

    // 面板占用，执行暂停
    p.logger.Info("Pausing panel HTTP listener on port 80...")
    if err := p.webController.PauseHTTPListener(); err != nil {
        return WrapError(err, "Failed to pause HTTP listener")
    }

    // 双重检查：暂停后端口是否真的释放了？
    // 简单的重试机制
    for i := 0; i < 5; i++ {
        time.Sleep(100 * time.Millisecond)
        occupied, _, _ := p.CheckPort80()
        if !occupied {
            return nil
        }
    }
    
    // 如果暂停后仍被占用（极罕见情况），尝试恢复并报错
    p.webController.ResumeHTTPListener()
    return NewError("Port 80 still occupied after pausing listener")
}

// TDD: Test_ReleasePort80
func (p *PortConflictResolver) ReleasePort80() error {
    if p.webController.IsListeningOnPort80() {
        p.logger.Info("Resuming panel HTTP listener on port 80...")
        return p.webController.ResumeHTTPListener()
    }
    return nil
}
```

---

### 2.2 激进续期策略模块 (AggressiveRenewalManager)

#### 2.2.1 功能规格
针对 IP 证书有效期短（通常 7 天）的特点，实施高频检查和提前续期。
1.  **配置**: 续期阈值设为 3 天（有效期 < 3 天即续期）。
2.  **调度**: 每 6 小时检查一次。
3.  **重试**: 失败后采用指数退避策略，避免短时间内频繁请求 CA。

#### 2.2.2 接口定义

```go
type RenewalConfig struct {
    CheckInterval  time.Duration // e.g., 6 hours
    RenewThreshold time.Duration // e.g., 3 days
    MaxRetries     int           // e.g., 5
}

type CertRenewer interface {
    StartLoop()
    StopLoop()
    ForceRenew() error
}
```

#### 2.2.3 伪代码与逻辑

```go
type AggressiveRenewer struct {
    config      RenewalConfig
    certService *CertService
    stopChan    chan struct{}
}

// TDD: Test_CalculateNextCheckTime
// TDD: Test_ShouldRenew
func (r *AggressiveRenewer) StartLoop() {
    ticker := time.NewTicker(r.config.CheckInterval)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            r.checkAndRenew()
        case <-r.stopChan:
            return
        }
    }
}

func (r *AggressiveRenewer) checkAndRenew() {
    certInfo, err := r.certService.GetCertInfo()
    if err != nil {
        // 证书不存在或无法读取，尝试申请
        r.attemptRenewWithBackoff()
        return
    }

    remaining := certInfo.Expiry.Sub(time.Now())
    
    // 激进策略：剩余时间小于阈值（3天）
    if remaining < r.config.RenewThreshold {
        r.logger.Infof("Certificate expires in %v, triggering aggressive renewal", remaining)
        r.attemptRenewWithBackoff()
    }
}

// TDD: Test_ExponentialBackoff
func (r *AggressiveRenewer) attemptRenewWithBackoff() {
    retryDelay := 1 * time.Minute
    
    for i := 0; i < r.config.MaxRetries; i++ {
        err := r.certService.RenewIPCert()
        if err == nil {
            r.logger.Info("Renewal successful")
            return
        }

        r.logger.Warnf("Renewal failed (attempt %d/%d): %v. Retrying in %v", 
            i+1, r.config.MaxRetries, err, retryDelay)
        
        // 简单的指数退避
        time.Sleep(retryDelay)
        retryDelay *= 2
        
        // 设置上限，例如最大等待 1 小时
        if retryDelay > 1*time.Hour {
            retryDelay = 1*time.Hour
        }
    }
    
    // 所有重试失败，触发告警
    r.certService.TriggerAlert("Renewal failed after max retries")
}
```

---

### 2.3 证书热加载模块 (CertHotReloader)

#### 2.3.1 功能规格
证书文件更新后，无需重启整个面板，仅通知 Xray 核心重载配置。
1.  **监听**: 监听证书更新成功事件。
2.  **动作**: 发送 `SIGHUP` 信号给 Xray 进程，或调用 Xray API（如果可用）。
3.  **验证**: 检查 Xray 是否存活。

#### 2.3.2 接口定义

```go
type XrayController interface {
    // ReloadCore 重载 Xray 核心配置
    ReloadCore() error
    // IsRunning 检查 Xray 是否运行中
    IsRunning() bool
}

// CertObserver 定义证书变更观察者
type CertObserver interface {
    OnCertUpdated(certPath, keyPath string) error
}
```

#### 2.3.3 伪代码与逻辑

```go
type CertHotReloader struct {
    xrayCtrl XrayController
    logger   Logger
}

// TDD: Test_OnCertUpdated_ReloadsXray
func (h *CertHotReloader) OnCertUpdated(certPath, keyPath string) error {
    h.logger.Info("Certificate updated, initiating Xray hot reload...")

    if !h.xrayCtrl.IsRunning() {
        h.logger.Warn("Xray is not running, skipping reload")
        return nil
    }

    // 1. 确保新证书文件已落盘且 Xray 有权限读取
    // (通常由 CertService 保证，但此处可做防御性检查)
    if err := checkFileReadable(certPath); err != nil {
        return err
    }

    // 2. 执行重载
    err := h.xrayCtrl.ReloadCore()
    if err != nil {
        h.logger.Errorf("Failed to reload Xray core: %v", err)
        return err
    }

    h.logger.Info("Xray core reloaded successfully")
    return nil
}

// 在 XrayService 中实现 ReloadCore
// TDD: Test_ReloadCore_SendsSignal
func (s *XrayService) ReloadCore() error {
    process := s.GetProcess()
    if process == nil {
        return errors.New("process not found")
    }

    // 发送 SIGHUP 信号 (Linux/macOS)
    // Windows 下可能需要完全重启
    if runtime.GOOS == "windows" {
        return s.Restart()
    }
    
    return process.Signal(syscall.SIGHUP)
}
```

---

### 2.4 告警与回退模块 (AlertAndFallback)

#### 2.4.1 功能规格
当续期彻底失败且证书即将过期时，通知管理员并采取保底措施。
1.  **告警**: 通过 Telegram Bot 发送详细告警（剩余时间、失败原因）。
2.  **回退**: 如果证书已过期或剩余时间极短（< 1小时），生成自签名证书覆盖，防止 Xray 启动失败。

#### 2.4.2 接口定义

```go
type AlertService interface {
    SendAlert(title, message string, level string) error
}

type FallbackManager interface {
    // CheckAndFallback 检查状态并执行回退
    CheckAndFallback(certPath string) error
}
```

#### 2.4.3 伪代码与逻辑

```go
type AlertAndFallbackModule struct {
    alertSvc AlertService
    certSvc  CertService
    logger   Logger
}

// TDD: Test_CheckExpiryAndAlert_Critical
// TDD: Test_CheckExpiryAndAlert_Warning
func (m *AlertAndFallbackModule) CheckExpiryAndAlert(certInfo CertInfo) {
    remaining := certInfo.Expiry.Sub(time.Now())

    // 严重告警：剩余 < 24 小时
    if remaining < 24*time.Hour {
        msg := fmt.Sprintf(
            "⚠️ **IP 证书紧急告警**\n\n" +
            "IP: `%s`\n" +
            "剩余时间: %s\n" +
            "状态: **即将过期**\n\n" +
            "请立即检查面板日志或手动续期！",
            certInfo.IP, remaining.String(),
        )
        m.alertSvc.SendAlert("Certificate Critical", msg, "CRITICAL")
    }
}

// TDD: Test_PerformFallback_GeneratesSelfSigned
func (m *AlertAndFallbackModule) PerformFallback(certPath, keyPath, ip string) error {
    m.logger.Warn("Executing emergency fallback: Generating self-signed certificate")

    // 1. 生成自签名证书内容
    certPEM, keyPEM, err := generateSelfSignedCert(ip)
    if err != nil {
        return err
    }

    // 2. 备份旧证书 (可选)
    backupFiles(certPath, keyPath)

    // 3. 写入新证书
    if err := writeFiles(certPath, certPEM, keyPath, keyPEM); err != nil {
        return err
    }

    // 4. 发送通知
    m.alertSvc.SendAlert(
        "Fallback Executed", 
        "已自动切换为自签名证书以维持服务运行。请尽快修复受信任的证书。", 
        "WARNING",
    )
    
    return nil
}

// 辅助函数：生成自签名证书
func generateSelfSignedCert(ip string) ([]byte, []byte, error) {
    // 使用 crypto/x509 生成简单的自签名证书
    // 有效期设为 30 天
    // ... (标准 Go 标准库实现)
    return certBytes, keyBytes, nil
}
```

---

## 3. 边缘情况处理 (Edge Cases)

1.  **网络隔离环境**:
    *   *场景*: 服务器无法连接 Let's Encrypt CA。
    *   *处理*: `AggressiveRenewer` 会重试多次后失败。`AlertAndFallback` 模块会在剩余 24 小时触发告警，最终回退到自签名证书。

2.  **端口 80 被 Nginx 永久占用**:
    *   *场景*: 用户安装了 Nginx 且未配置反代。
    *   *处理*: `PortConflictResolver` 检测到外部占用，返回错误。日志记录 "Port 80 occupied by external process"。用户需手动干预。

3.  **Xray 进程僵死**:
    *   *场景*: `CertHotReloader` 尝试发送 SIGHUP，但进程无响应。
    *   *处理*: `ReloadCore` 应设置超时。如果信号发送无错误但服务未恢复，通常由外部守护进程（如 systemd）管理，面板侧仅记录日志。

4.  **系统时间偏差**:
    *   *场景*: 服务器时间错误导致证书验证失败。
    *   *处理*: 依赖 NTP。代码层面难以完全解决，但日志中会体现 "certificate has expired or is not yet valid"。

## 4. 测试计划 (Test Plan)

### 4.1 单元测试 (Unit Tests)
*   `TestPortConflictResolver`: Mock `net.Dial` 和 `WebServerController`，验证占用检测和接管逻辑。
*   `TestAggressiveRenewer`: Mock `CertService`，验证时间计算和重试计数器。
*   `TestCertHotReloader`: 验证文件更新回调是否触发了 `ReloadCore`。

### 4.2 集成测试 (Integration Tests)
*   **模拟环境**: 使用 Docker 容器模拟端口占用。
*   **流程验证**:
    1.  启动模拟 Web Server 占用 80。
    2.  触发证书申请。
    3.  验证 Web Server 暂停 -> 证书申请 -> Web Server 恢复。
    4.  验证 Xray 收到 SIGHUP 信号（通过日志或进程状态）。

### 4.3 手动验证 (Manual Verification)
*   在真实 VPS 上部署，安装 Nginx 制造冲突，观察日志。
*   修改系统时间模拟证书即将过期，验证 Telegram 告警。
