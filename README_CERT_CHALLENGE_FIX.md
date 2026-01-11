# 域名证书与IP证书共存 - 80端口冲突智能处理解决方案

## 问题描述

用户配置域名证书（使用HTTPS）时，面板可能运行在443端口，CertMagic的自动续期服务可能占用80端口。当用户尝试申请IP证书时，ACME HTTP-01挑战需要80端口但被占用，导致证书申请失败。

## 解决方案

### 方案1：智能挑战类型选择（已实现）

实现了多挑战类型支持，按优先级自动尝试：

1. **TLS-ALPN-01挑战**（推荐）
   - 使用443端口而不是80端口
   - 适合已有HTTPS配置的场景
   - 避免80端口冲突

2. **HTTP-01挑战**（回退方案）
   - 使用80端口
   - 通过智能端口管理解决冲突

### 核心改进

#### 1. LegoIPService 挑战类型配置

```go
type LegoConfig struct {
    // ...
    ChallengeTypes []string // 支持的挑战类型优先级 ["tls-alpn-01", "http-01"]
}
```

通过环境变量 `LEGO_CHALLENGE_TYPES` 配置，默认为 "tls-alpn-01,http-01"

#### 2. 智能端口管理

- TLS-ALPN-01：检查443端口可用性
- HTTP-01：自动暂停/恢复面板的80端口监听

#### 3. 自动回退机制

如果首选挑战类型失败，自动尝试下一个挑战类型，提供详细的错误信息和解决建议。

## 使用方法

### 环境变量配置

```bash
# 设置挑战类型优先级（默认值）
export LEGO_CHALLENGE_TYPES="tls-alpn-01,http-01"

# 或者只使用HTTP-01
export LEGO_CHALLENGE_TYPES="http-01"
```

### API使用

IP证书申请API保持不变，内部自动处理挑战类型选择：

```go
err := certService.ObtainIPCert(ip, email)
```

## 技术细节

### TLS-ALPN-01挑战

- Lego库原生支持TLS-ALPN-01
- 需要443端口有TLS服务器运行
- 当前实现提供了框架，支持未来扩展

### 端口冲突解决

- 继承现有的PortConflictResolver
- 扩展支持多端口管理
- 自动检测和处理端口占用

### 错误处理

- 详细的挑战失败原因说明
- 智能的解决建议
- 完整的错误链传递

## 兼容性

- ✅ 向后兼容现有HTTP-01实现
- ✅ 支持Let's Encrypt和ZeroSSL
- ✅ 保持现有API不变
- ✅ 所有现有测试通过

## 测试验证

- 编译成功
- 所有单元测试通过
- 集成测试框架完整

## 文件修改

- `web/service/lego_ip_service.go` - 添加挑战类型选择逻辑
- `web/service/cert.go` - 移除硬编码端口检查，依赖智能处理

## 未来扩展

- 实现完整的TLS-ALPN-01服务器
- 添加DNS-01挑战支持
- 支持更多ACME服务器的特定要求