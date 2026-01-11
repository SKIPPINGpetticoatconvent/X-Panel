# Lego + CertMagic 证书管理规范

## 1. 概述 (Overview)

本规范旨在重构 X-Panel 的证书管理模块，移除对 `acme.sh` Shell 脚本的依赖，转为使用 Go 原生库实现。
系统将采用双引擎策略：
- **Lego**: 专门负责 IP 证书的申请与管理（底层控制力更强，适配 IP 证书的特殊性）。
- **CertMagic**: 专门负责域名证书的全自动化管理（自动续期、OCSP Stapling、多 Challenge 支持）。

## 2. 功能需求 (Functional Requirements)

### 2.1 Lego IP 证书服务 (`LegoIPCertService`)
- **提供商支持**: 支持 Let's Encrypt (HTTP-01) 和 ZeroSSL (HTTP-01)。
- **证书申请**:
  - 接收 IP 地址和邮箱。
  - 自动生成私钥 (ECC P256/P384)。
  - 注册 ACME 账户。
  - 执行 HTTP-01 Challenge。
- **证书存储**:
  - 存储路径: `bin/cert/ip/<ip>/`
  - 文件格式: `<ip>.cer` (公钥/证书链), `<ip>.key` (私钥)。
- **生命周期管理**:
  - 手动申请。
  - 手动续期。
  - *注：IP 证书通常不通过 CertMagic 自动续期，因为需要严格的端口控制，建议保持手动或简单的定时任务触发。*

### 2.2 CertMagic 域名证书服务 (`CertMagicDomainService`)
- **自动化管理**:
  - 只要在配置中添加了域名，服务启动时自动申请/加载证书。
  - 后台自动监控有效期并续期（默认过期前 30 天）。
- **Challenge 支持**:
  - **HTTP-01**: 默认方式，需占用 80 端口。
  - **TLS-ALPN-01**: 备选方式，需占用 443 端口。
  - **DNS-01**: 高级功能，支持 Cloudflare 等主流 DNS 提供商（需提供 API Key）。
- **证书存储**:
  - 使用 CertMagic 默认的 `FileStorage`。
  - 存储路径: `bin/cert/domains/` (自定义 CertMagic 的 Storage Path)。

### 2.3 通用功能
- **端口冲突解决**: 集成 `PortConflictResolver`，在申请证书期间自动协调 80/443 端口的使用。
- **热重载**: 证书变更（申请/续期成功）后，触发 `CertHotReloader` 通知 Xray 核心。
- **统一视图**: 提供统一的接口获取当前所有证书（IP 和 域名）的状态、过期时间等信息。

## 3. 非功能需求 (Non-Functional Requirements)

### 3.1 安全性 (Security)
- **私钥保护**: 所有私钥文件权限必须设置为 `0600`。
- **内存安全**: 私钥在内存中使用后应尽快释放（Go GC 会处理，但需避免全局持有）。
- **权限控制**: 确保证书文件由运行面板的用户所有，防止其他用户读取。

### 3.2 性能与可靠性 (Performance & Reliability)
- **并发控制**: 证书申请应使用互斥锁或队列，避免对同一目标同时发起多个申请。
- **重试机制**: 网络请求失败应有指数退避重试（Lego/CertMagic 内部已有部分实现，需封装顶层超时）。
- **无阻塞**: 证书申请操作不得阻塞面板的主 HTTP 服务线程（需异步执行）。

### 3.3 可维护性 (Maintainability)
- **日志记录**: 详细记录申请过程中的每一步（注册、Challenge、验证、下载），便于排错。
- **模块化**: `Lego` 和 `CertMagic` 的逻辑完全分离，通过接口对外暴露。

## 4. 接口设计 (Interface Design)

### 4.1 核心接口定义

```go
// ICertManager 定义统一的证书管理接口
type ICertManager interface {
    // ListCerts 列出所有管理的证书信息
    ListCerts() ([]CertInfo, error)
    // GetCertPath 获取指定标识(IP或域名)的证书路径
    GetCertPath(identifier string) (certPath, keyPath string, err error)
}

// CertInfo 证书信息摘要
type CertInfo struct {
    Identifier string    // IP 或 域名
    Type       string    // "IP" 或 "Domain"
    Provider   string    // "Let's Encrypt", "ZeroSSL", etc.
    Expiry     time.Time // 过期时间
    AutoRenew  bool      // 是否自动续期
}

// ILegoIPService Lego IP 证书服务接口
type ILegoIPService interface {
    // ObtainIPCert 申请 IP 证书
    ObtainIPCert(ctx context.Context, ip string, email string) error
    // RenewIPCert 续期 IP 证书
    RenewIPCert(ctx context.Context, ip string) error
}

// ICertMagicService CertMagic 域名服务接口
type ICertMagicService interface {
    // ManageDomains 开始管理一组域名（自动申请/续期）
    ManageDomains(ctx context.Context, domains []string, email string) error
    // UpdateDNSConfig 更新 DNS Challenge 配置
    UpdateDNSConfig(provider string, credentials map[string]string) error
}
```

### 4.2 集成点

- **PortConflictResolver**:
  - 在 `LegoIPService.ObtainIPCert` 开始前，调用 `AcquirePort80`。
  - 申请结束后，调用 `ReleasePort80`。
  - CertMagic 需要自定义 `Solver` 来集成端口冲突解决逻辑。

- **CertHotReloader**:
  - 在证书文件写入成功后，调用 `OnCertUpdated(cert, key)`。

## 5. 边界条件和约束 (Edge Cases & Constraints)

1.  **端口 80 被非面板进程占用**:
    - 如果是 Nginx/Apache 等外部进程占用 80，`PortConflictResolver` 无法暂停它们。
    - **策略**: 报错并提示用户手动停止占用进程，或配置 DNS Challenge (仅限域名)。

2.  **端口 80 被面板自身占用**:
    - 面板监听在 80 端口。
    - **策略**: `PortConflictResolver` 暂停面板 HTTP Listener -> 申请证书 -> 恢复 Listener。

3.  **迁移策略**:
    - 系统启动时，检查 `acme.sh` 目录。
    - 如果存在旧证书，尝试将其复制到新的标准目录结构中，或保持原样直到用户点击“续期/重新申请”。
    - 建议提供“一键迁移”按钮，强制重新申请所有证书以接管管理权。

4.  **网络隔离环境**:
    - 如果服务器无法访问 Let's Encrypt API。
    - **策略**: 快速失败，返回明确的网络错误日志。

## 6. 伪代码设计 (Pseudocode with TDD Anchors)

### 6.1 LegoIPCertService

```go
// web/service/cert/lego_service.go

type LegoIPService struct {
    portResolver *service.PortConflictResolver
    fileStorage  *CertFileStorage
}

// TDD Anchor: Test_Lego_ObtainIPCert_Flow
func (s *LegoIPService) ObtainIPCert(ctx context.Context, ip string, email string) error {
    // 1. Validate IP
    if !isValidIP(ip) {
        return ErrInvalidIP
    }

    // 2. Resolve Port 80 Conflict
    // TDD Anchor: Test_Port_Acquisition
    err := s.portResolver.AcquirePort80(ctx)
    if err != nil {
        return fmt.Errorf("failed to acquire port 80: %w", err)
    }
    defer s.portResolver.ReleasePort80()

    // 3. Initialize Lego User & Client
    user, err := NewLegoUser(email)
    config := lego.NewConfig(user)
    client, err := lego.NewClient(config)

    // 4. Setup HTTP Provider
    // 使用 Lego 内置的 HTTP Server，监听 :80
    // 因为我们已经通过 PortResolver 确保 80 空闲
    err = client.Challenge.SetHTTP01Provider(http01.NewProviderServer("", "80"))

    // 5. Register Account
    reg, err := client.Registration.Register(registration.RegisterOptions{TermsOfServiceAgreed: true})

    // 6. Obtain Certificate
    request := certificate.ObtainRequest{
        Domains: []string{ip},
        Bundle:  true,
    }
    // TDD Anchor: Test_Lego_Obtain_Success
    certResource, err := client.Certificate.Obtain(request)
    if err != nil {
        return fmt.Errorf("lego obtain failed: %w", err)
    }

    // 7. Save to Disk
    // TDD Anchor: Test_Save_Cert_Files
    err = s.fileStorage.SaveIPCert(ip, certResource.Certificate, certResource.PrivateKey)
    
    // 8. Trigger Hot Reload
    // TDD Anchor: Test_Hot_Reload_Trigger
    go service.GetCertHotReloader().OnCertUpdated(s.fileStorage.GetCertPath(ip))

    return nil
}
```

### 6.2 CertMagicDomainService

```go
// web/service/cert/certmagic_service.go

type CertMagicService struct {
    config *certmagic.Config
}

// TDD Anchor: Test_CertMagic_Init
func NewCertMagicService(email string) *CertMagicService {
    // 配置 CertMagic 使用文件存储
    certmagic.DefaultACME.Email = email
    certmagic.DefaultACME.Agreed = true
    
    // 自定义 Storage 路径
    certmagic.Default.Storage = &certmagic.FileStorage{Path: "bin/cert/domains"}
    
    return &CertMagicService{}
}

// TDD Anchor: Test_Manage_Domains
func (s *CertMagicService) ManageDomains(ctx context.Context, domains []string) error {
    // CertMagic 的 Manage 是异步的，但首次申请可能会阻塞
    // 我们需要确保它不会阻塞主线程太久，或者在后台运行
    
    // 关键：处理端口冲突
    // CertMagic 默认会尝试绑定 80/443。
    // 我们需要实现自定义的 Listener 或者在调用前解决冲突。
    // 由于 CertMagic 是长期运行的，简单的 Acquire/Release 模式可能不适用。
    // 更好的方式是：
    // 1. 如果面板占用 80，CertMagic 无法工作，除非面板代理 Challenge 请求（复杂）。
    // 2. 或者，我们只在“申请时”短暂接管端口。
    
    // 策略调整：使用 CertMagic 的 "OnDemand" 模式或手动触发 "Obtain" 而不是长期 "Manage"
    // 或者，实现一个与 PortConflictResolver 协作的 Solver。
    
    // 简化版实现：手动触发 Obtain，类似 Lego 流程，但利用 CertMagic 的库优势
    
    cfg := certmagic.NewDefault()
    
    // 包装 HTTP Challenge 过程中的端口接管
    // TDD Anchor: Test_CertMagic_Port_Coordination
    err := service.GetPortConflictResolver().AcquirePort80(ctx)
    if err != nil {
        return err
    }
    defer service.GetPortConflictResolver().ReleasePort80()
    
    err = cfg.ObtainCert(ctx, domains[0], false) // Sync call
    if err != nil {
        return err
    }
    
    return nil
}
```

### 6.3 目录结构规范

```text
/home/ub/X-Panel/
├── bin/
│   └── cert/
│       ├── ip/
│       │   └── 1.2.3.4/
│       │       ├── 1.2.3.4.cer
│       │       └── 1.2.3.4.key
│       └── domains/
│           └── certificates/ (CertMagic 默认结构)
│               └── example.com/
│                   ├── example.com.crt
│                   └── example.com.key
```
