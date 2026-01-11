# IP 证书申请功能修复方案设计文档

## 1. 背景与问题分析

### 1.1 问题描述
当前 IP 证书申请功能存在两个主要问题：
1.  **CLI 命令缺陷**：在 CLI 模式下运行 `cert-request-ip` 命令时，`CertService` 被初始化但未注入 `ServerService` 和 `TelegramService`。由于 `tryInitImprovements` 方法强制检查这些依赖是否为 `nil`，导致 `acmeShService` 等核心模块无法初始化，最终抛出 "acme.sh service not available" 错误。
2.  **Telegram Bot 强依赖**：`CertService` 的初始化逻辑错误地将 Telegram Bot 作为硬性依赖。如果用户未启用 Telegram Bot，IP 证书功能将无法正常工作。

### 1.2 根本原因
代码位于 `web/service/cert.go` 的 `tryInitImprovements` 方法中：
```go
func (c *CertService) tryInitImprovements() {
    if c.serverService == nil || c.tgbot == nil {
        return // <--- 强制依赖检查
    }
    // ... 初始化逻辑
}
```
此检查阻止了即使不需要 `tgbot` 或 `serverService` 也能工作的模块（如 `acmeShService`）的初始化。

## 2. 修复方案设计

### 2.1 设计目标
1.  **解除强依赖**：移除对 Telegram Bot 和 ServerService 的强制依赖，使 IP 证书功能（特别是 CLI 模式）能够独立运行。
2.  **按需初始化**：根据可用依赖动态初始化可选模块（如告警、热重载）。
3.  **保持兼容性**：确保在完整 Web 模式下，所有功能（包括告警和热重载）依然正常工作。

### 2.2 核心修改逻辑 (`web/service/cert.go`)

修改 `tryInitImprovements` 方法，采用"尽力而为"的初始化策略：

1.  **移除** 顶部的 `if c.serverService == nil || c.tgbot == nil { return }` 检查。
2.  **始终初始化** 核心基础模块：
    -   `AcmeShService`: 仅依赖文件系统和网络，无外部服务依赖。
    -   `PortConflictResolver`: 依赖 `SettingService`（已存在）。
3.  **条件初始化** 可选增强模块：
    -   `CertAlertFallback`:
        -   如果 `c.tgbot` 存在，注入 `certAlertService`。
        -   如果 `c.tgbot` 不存在，注入 `nil` 作为 `AlertService`。
        -   `CertAlertFallback` 内部已处理 `alertService` 为 `nil` 的情况（仅跳过发送通知，不影响回退逻辑）。
    -   `AggressiveRenewalManager`:
        -   始终初始化，依赖 `CertAlertFallback`（已处理 nil 情况）。
    -   `CertHotReloader`:
        -   仅当 `c.serverService` 不为 `nil` 时初始化。
        -   CLI 模式下通常不需要热重载（或者无法直接操作运行中的服务），因此跳过是安全的。

### 2.3 伪代码实现

```go
func (c *CertService) tryInitImprovements() {
    // 移除强制依赖检查
    
    c.initOnce.Do(func() {
        logger.Info("Initializing IP Certificate Improvement Modules...")

        // 1. 始终初始化核心服务
        c.acmeShService = NewAcmeShService()

        webCtrl := &certWebServerController{settingService: c.settingService}
        c.portResolver = NewPortConflictResolver(webCtrl)

        // 2. 条件初始化告警服务
        var alertSvc AlertService
        if c.tgbot != nil {
            alertSvc = &certAlertService{tgbot: c.tgbot}
        }
        // 即使 alertSvc 为 nil，Fallback 模块也能工作（只是不发通知）
        c.alertFallback = NewCertAlertFallback(alertSvc, c, c.settingService)

        // 3. 始终初始化续期管理器
        renewalConfig := RenewalConfig{
            // ... 配置保持不变
        }
        c.renewalManager = NewAggressiveRenewalManager(renewalConfig, c, c.portResolver, c.alertFallback)

        // 4. 条件初始化热重载 (依赖 ServerService)
        if c.serverService != nil {
            xrayCtrl := &certXrayController{serverService: c.serverService}
            c.hotReloader = NewCertHotReloader(xrayCtrl)
        }

        logger.Info("IP Certificate Improvement Modules initialized successfully")
    })
}
```

## 3. 影响分析

### 3.1 CLI 模式 (`cert-request-ip`)
-   **依赖状态**: `serverService` 为 `nil`, `tgbot` 为 `nil`。
-   **行为**:
    -   `acmeShService`: 初始化成功。
    -   `portResolver`: 初始化成功。
    -   `alertFallback`: 初始化成功（无告警能力）。
    -   `hotReloader`: 未初始化 (`nil`)。
-   **结果**: `ObtainIPCert` 可以正常调用 `acmeShService` 申请证书。证书申请成功后，尝试调用 `hotReloader` 时会因 `nil` 检查而安全跳过。**CLI 功能修复成功。**

### 3.2 Web 模式 (未启用 Telegram)
-   **依赖状态**: `serverService` 存在, `tgbot` 为 `nil`。
-   **行为**:
    -   所有模块初始化成功，唯独 `alertFallback` 无法发送 Telegram 通知。
    -   `hotReloader` 正常工作，证书更新后会自动重载 Xray。
-   **结果**: 功能正常，符合预期（未启用 Bot 自然无通知）。

### 3.3 Web 模式 (启用 Telegram)
-   **依赖状态**: `serverService` 存在, `tgbot` 存在。
-   **行为**:
    -   所有模块完整初始化。
-   **结果**: 功能完全正常，包含告警通知。

## 4. 修改文件列表

1.  `web/service/cert.go`

## 5. 实施步骤

1.  修改 `web/service/cert.go` 中的 `tryInitImprovements` 方法，实现上述解耦逻辑。
2.  无需修改 `main.go`，因为 `CertService` 现在可以处理缺失的依赖。
3.  无需修改 `web/service/cert_alert_fallback.go`，经检查其 `SendTelegramAlert` 方法已包含 `if c.alertService == nil` 检查，是安全的。

## 6. 潜在风险

-   **CertAlertFallback 的 nil 安全性**: 必须确保 `NewCertAlertFallback` 及其方法在 `alertService` 为 `nil` 时不会 panic。已验证代码，`SendTelegramAlert` 有空指针检查，风险极低。
-   **HotReloader 的 nil 安全性**: `ObtainIPCert` 方法中调用 `c.hotReloader.OnCertRenewed` 前必须检查 `c.hotReloader != nil`。已验证现有代码包含此检查 (`if c.hotReloader != nil`)，风险极低。
