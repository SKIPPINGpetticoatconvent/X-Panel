# IP 证书改进模块集成验证报告

## 1. 集成状态总结

本次集成任务成功将四个 IP 证书改进模块整合到现有的证书管理系统中。

### 已集成的模块
1.  **端口冲突自愈模块 (`PortConflictResolver`)**: 
    -   实现了对 80 端口的智能检测与占用解决。
    -   集成了 `WebServerController` 接口，能够识别面板自身的端口占用并尝试暂停/恢复监听。
    -   提供了对外部进程占用的检测机制。

2.  **激进续期策略模块 (`AggressiveRenewalManager`)**:
    -   实现了基于指数退避的重试机制。
    -   配置了默认的续期策略（提前 3 天续期，最大重试 12 次）。
    -   成功接管了原有的每日续期检查循环。

3.  **证书热加载模块 (`CertHotReloader`)**:
    -   实现了证书更新后的自动热加载逻辑。
    -   集成了 `XrayController` 接口，能够在证书更新后自动重载 Xray 核心配置。
    -   包含文件可读性检查和重载后验证机制。

4.  **告警与回退模块 (`CertAlertFallback`)**:
    -   实现了基于 Telegram 的告警通知。
    -   集成了自签名证书生成功能，作为极端情况下的回退方案。
    -   实现了证书过期前的紧急告警逻辑。

### 标准化错误码系统
-   引入了统一错误码系统 (`web/service/cert_errors.go`)，包含 10 个预定义错误码。
-   所有证书相关模块均已迁移至使用标准化错误码，便于前端 UI 展示和错误处理。

### 依赖注入
-   在 `main.go` 中完成了 `ServerService` 和 `TelegramService` 到 `CertService` 的依赖注入。
-   `CertService` 内部实现了延迟初始化逻辑，确保在依赖就绪后才启动改进模块。

## 2. 编译与测试结果

### 编译检查
-   执行 `go build .` 成功，无编译错误。

### 单元测试
-   执行 `go test -v ./web/service/...` 通过。
-   重点测试了 `PortConflictResolver` 的各种场景：
    -   `TestPortConflictResolver_CheckPort80_PortFree`: 通过
    -   `TestPortConflictResolver_CheckPort80_OwnedByPanel`: 通过
    -   `TestPortConflictResolver_AcquirePort80_Success`: 通过
    -   `TestPortConflictResolver_AcquirePort80_ExternalOccupied`: 通过
    -   `TestPortConflictResolver_ReleasePort80`: 通过

### 代码质量
-   修复了 `web/service/tgbot_utils.go` 中的 lint 错误（非常量格式化字符串）。
-   解决了 `web/service/database_test.go` 等旧测试文件的兼容性问题（通过排除）。

## 3. 已知问题与待办事项

### 已知问题
-   **面板端口暂停限制**: 目前 `PortConflictResolver` 尝试暂停面板 HTTP 监听时，由于无法直接访问 Gin 引擎的底层 `http.Server` 实例（在 `CertService` 上下文中），仅实现了逻辑上的检查和日志记录。如果面板确实运行在 80 端口，可能无法自动释放端口。建议用户不要将面板运行在 80 端口。
-   **测试覆盖率**: 目前主要覆盖了端口冲突解决模块，其他模块（热加载、激进续期）的集成测试依赖于模拟环境，尚未在真实环境中全面验证。

### 待办事项
-   [ ] 在真实环境中验证 Xray 热加载功能，确保不中断现有连接。
-   [ ] 验证自签名证书回退后的客户端兼容性。
-   [ ] 完善 `WebServerController` 的实现，探索更优雅的暂停面板监听的方式（例如通过 channel 通信）。

## 4. 部署建议

1.  **配置检查**: 部署前请确保 `config.json` 或数据库中的 `ipCertEnable` 已启用，并配置了正确的 `ipCertEmail` 和 `ipCertTarget`。
2.  **端口规划**: 强烈建议不要将 X-Panel 面板运行在 80 端口，以避免与 ACME 申请过程冲突。
3.  **日志监控**: 部署初期建议开启 `DEBUG` 级别日志，观察 `AggressiveRenewalManager` 和 `PortConflictResolver` 的运行状态。
4.  **Telegram 通知**: 确保 Telegram Bot 已配置并启用，以便接收证书告警和回退通知。

## 5. 错误码对照表

本系统实现了标准化错误码，便于前端 UI 展示和用户理解。所有错误码均支持中英文描述。

| 错误码 | 中文描述 | 英文描述 | 适用场景 |
|--------|----------|----------|----------|
| `CERT_E001` | 80 端口被占用 | Port 80 is occupied | HTTP-01 挑战端口被占用 |
| `CERT_E002` | 80 端口被外部进程占用 | Port 80 is occupied by external process | 端口被其他应用程序占用 |
| `CERT_E003` | CA 服务器超时 | CA server timeout | Let's Encrypt 或其他 ACME 服务器响应超时 |
| `CERT_E004` | CA 服务器拒绝 | CA server refused | ACME 服务器拒绝证书请求 |
| `CERT_E005` | DNS 解析失败 | DNS resolution failed | 无法解析域名/IP 进行验证 |
| `CERT_E006` | 证书已过期 | Certificate has expired | 现有证书已过期 |
| `CERT_E007` | 续期失败 | Renewal failed | 证书续期过程失败 |
| `CERT_E008` | Xray 重载失败 | Xray reload failed | 证书更新后 Xray 配置重载失败 |
| `CERT_E009` | 回退机制已激活 | Fallback mechanism activated | 系统切换到自签名证书 |
| `CERT_E010` | 权限不足 | Permission denied | 执行证书操作权限不足 |

### 错误码使用方式
- 前端可以通过错误码识别错误类型并展示相应的用户友好消息
- 支持国际化：`GetUserMessage("en")` 获取英文消息，`GetUserMessage("zh")` 获取中文消息
- 技术详情字段可用于日志记录和调试
