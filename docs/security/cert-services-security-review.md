# Lego IP & CertMagic 证书服务安全审查报告

**日期**: 2024-05-23
**审查者**: Security Reviewer
**审查对象**:
- `web/service/lego_ip_service.go`
- `web/service/certmagic_domain_service.go`
- `web/service/lego_ip_service_test.go`
- `web/service/certmagic_domain_service_test.go`

## 1. 发现的安全问题 (Security Findings)

### 1.1 高风险 (High Severity)

*   **未完成的实现 (Incomplete Implementation)**
    *   **位置**: `web/service/certmagic_domain_service.go`
    *   **描述**: `CertMagicDomainService` 中存在大量注释掉的代码和 TODO，特别是 `initCertMagic` 和 `ObtainDomainCert` 方法。依赖项 `github.com/caddyserver/certmagic` 被注释掉。
    *   **影响**: 如果此代码部署到生产环境，将无法提供域名证书服务，且可能导致运行时错误或恐慌（Panic）。
    *   **建议**: 在依赖项可用之前，不要启用此服务，或者使用 Feature Flag 禁用。

### 1.2 中风险 (Medium Severity)

*   **硬编码的默认邮箱 (Hardcoded Default Email)**
    *   **位置**: `web/service/lego_ip_service.go:366` (`getStoredEmail`)
    *   **描述**: `getStoredEmail` 方法返回硬编码的 `admin@example.com`。
    *   **影响**: 如果调用者未提供有效邮箱，将使用此默认邮箱申请证书，可能导致 ACME 账户关联错误或无法收到过期通知。
    *   **建议**: 实现从配置文件或数据库读取真实的用户邮箱。

*   **PII 泄露 (PII Leakage in Logs)**
    *   **位置**: `web/service/lego_ip_service.go:98`, `web/service/certmagic_domain_service.go:119`
    *   **描述**: `logger.Infof` 在日志中记录了用户的 Email 地址。
    *   **影响**: 用户的个人身份信息 (PII) 暴露在日志文件中，可能违反隐私合规要求。
    *   **建议**: 在日志中对 Email 进行脱敏处理，或仅记录 Email 的哈希值。

*   **文件写入竞态条件 (File Write Race Condition)**
    *   **位置**: `web/service/lego_ip_service.go:298` (`saveCertificate`)
    *   **描述**: `LegoIPService` 缺乏针对特定 IP 的文件锁。如果并发请求同一个 IP 的证书，可能会导致文件写入冲突或损坏。
    *   **影响**: 证书文件可能损坏，导致服务不可用。
    *   **建议**: 使用 `sync.Mutex` 或文件锁（如 `flock`）来同步对同一 IP 证书目录的写入操作。

### 1.3 低风险 (Low Severity)

*   **HTTP-01 监听所有接口 (HTTP-01 Listens on All Interfaces)**
    *   **位置**: `web/service/lego_ip_service.go:138`
    *   **描述**: `http01.NewProviderServer("", "80")` 绑定到所有网络接口。
    *   **影响**: 虽然这是 ACME 验证所需的，但在某些严格的网络环境中，可能希望限制监听的 IP。
    *   **建议**: 考虑是否需要配置监听特定 IP，或者确认这是预期行为。

## 2. 合规性检查结果 (Compliance Checklist)

| 检查项 | 状态 | 说明 |
| :--- | :---: | :--- |
| **Secrets 管理** | ⚠️ 部分通过 | 环境变量使用正确，但存在硬编码邮箱。 |
| **私钥保护** | ✅ 通过 | 私钥文件权限设置为 `0600`。 |
| **输入验证** | ✅ 通过 | IP 和域名格式验证逻辑完善。 |
| **错误处理** | ✅ 通过 | 错误被正确包装和返回，未发现敏感信息泄露（除 Email）。 |
| **文件操作** | ✅ 通过 | 路径遍历防护有效，文件权限设置正确。 |
| **网络安全** | ✅ 通过 | 使用 HTTPS ACME 端点。 |
| **并发安全** | ⚠️ 部分通过 | 端口冲突解决已实现，但缺乏文件写入锁。 |
| **模块边界** | ✅ 通过 | 文件行数均小于 500 行，职责清晰。 |

## 3. 修复建议 (Remediation Plan)

1.  **完善 CertMagic 实现**:
    *   恢复 `go.mod` 中的依赖。
    *   取消 `certmagic_domain_service.go` 中代码的注释。
    *   实现 `setupDNSProvider` 逻辑。

2.  **移除硬编码**:
    *   修改 `getStoredEmail` 以从配置读取。

3.  **增强并发控制**:
    *   在 `LegoIPService` 中添加 `map[string]*sync.Mutex` 来锁定特定 IP 的操作。

4.  **日志脱敏**:
    *   修改日志语句，隐藏 Email 地址。

5.  **单元测试**:
    *   完善 `lego_ip_service_test.go` 和 `certmagic_domain_service_test.go` 中的 Mock 测试，覆盖更多边缘情况。
