# SNISelector 安全性审查报告

## 审查概述

本报告对 `SNISelector` 及其在 `Tgbot` 中的集成进行全面的安全审查，重点关注并发安全、数据竞争、输入验证和资源耗尽等关键安全领域。

## 1. 并发安全性分析 ✅

### 1.1 锁机制实现

- **状态**: ✅ 安全
- **分析**:
  - 所有方法都正确使用 `sync.Mutex` 保护共享状态
  - 使用 `defer s.mu.Unlock()` 确保锁总是被释放
  - 锁粒度合适，每个操作都是原子的
  - 没有嵌套锁调用，消除死锁风险

### 1.2 共享状态保护

- **状态**: ✅ 安全
- **分析**:
  - `domains`、`index`、`rng` 等字段都在锁保护下访问
  - 切片复制机制防止外部修改内部状态
  - 随机数生成器的访问也是线程安全的

## 2. 数据竞争分析 ✅

### 2.1 Tgbot 集成点分析

- **状态**: ✅ 安全
- **分析**:
  - `buildRealityInbound()` 和 `buildXhttpRealityInbound()` 都调用 `t.sniSelector.Next()`
  - `remoteCreateOneClickInbound()` 可能被并发调用
  - 由于 `Next()` 方法是线程安全的，不存在数据竞争

### 2.2 访问模式分析

- **状态**: ✅ 安全
- **分析**:
  - 每次调用 `Next()` 都是独立的操作
  - SNI 选择器的内部状态由互斥锁保护
  - 不存在读写分离的竞态条件

## 3. 输入验证分析 ⚠️

### 3.1 构造函数验证

- **状态**: ✅ 安全
- **分析**:
  ```go
  if len(initialDomains) == 0 {
      // 如果列表为空，提供默认值防止 panic
      initialDomains = []string{"www.google.com", "www.amazon.com"}
  }
  ```
  - 空列表检查防止 panic
  - 提供合理的默认值

### 3.2 UpdateDomains 验证

- **状态**: ✅ 安全
- **分析**:
  ```go
  if len(newDomains) == 0 {
      return
  }
  ```
  - 拒绝空域名列表
  - 防止无效状态转换

### 3.3 域名文件读取安全 🔍

- **状态**: ⚠️ 存在安全风险
- **分析**:
  - 在 `server.go:1745` 中直接使用 `os.ReadFile(filePath)`
  - 没有路径验证，可能存在目录遍历风险
  - 没有文件权限检查
  - 文件内容解析缺少充分验证

### 3.4 域名格式验证缺失 ⚠️

- **状态**: ⚠️ 需要加强
- **分析**:
  - 缺少域名格式正则验证
  - 可能接受无效的域名格式
  - 没有检查恶意域名或注入风险

## 4. 资源耗尽分析 ✅

### 4.1 内存管理

- **状态**: ✅ 安全
- **分析**:
  - 域名列表长度固定，不会无限增长
  - `UpdateDomains` 正确释放旧切片
  - 使用值拷贝而非引用，避免内存泄漏

### 4.2 CPU 使用

- **状态**: ✅ 安全
- **分析**:
  - 洗牌算法 O(n) 复杂度，可接受
  - 随机数生成器高效

## 5. 具体安全问题

### 5.1 文件读取安全漏洞 ⚠️

- **问题**: `GetCountrySNIDomains()` 函数存在路径遍历风险
- **代码位置**: `server.go:1742-1745`
- **风险**: 可能读取任意文件
- **修复建议**:
  ```go
  // 添加路径验证
  func (s *ServerService) GetCountrySNIDomains(countryCode string) []string {
      // 标准化和验证国家代码
      countryCode = strings.ToUpper(countryCode)
      if !isValidCountryCode(countryCode) {
          logger.Warningf("无效的国家代码: %s", countryCode)
          return s.getDefaultSNIDomains("DEFAULT")
      }
      
      // 构建安全的文件路径
      filePath := filepath.Join("sni", countryCode, "sni_domains.txt")
      
      // 验证文件路径在预期目录内
      absPath, err := filepath.Abs(filePath)
      if err != nil {
          logger.Warningf("无法获取文件绝对路径: %v", err)
          return s.getDefaultSNIDomains(countryCode)
      }
      
      // 检查文件是否在允许的目录内
      allowedDir := filepath.Join(s.getBaseDir(), "sni")
      if !strings.HasPrefix(absPath, allowedDir) {
          logger.Warningf("文件路径不在允许的目录内: %s", absPath)
          return s.getDefaultSNIDomains(countryCode)
      }
      
      // ... 其余逻辑
  }
  ```

### 5.2 域名格式验证缺失 ⚠️

- **问题**: 域名解析缺少格式验证
- **风险**: 可能接受恶意域名或无效格式
- **修复建议**:
  ```go
  func isValidDomain(domain string) bool {
      // 域名长度检查
      if len(domain) == 0 || len(domain) > 253 {
          return false
      }
      
      // 使用正则表达式验证域名格式
      pattern := `^[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?)*$`
      if !regexp.MustCompile(pattern).MatchString(domain) {
          return false
      }
      
      // 检查不允许的字符
      forbiddenChars := []string{"<", ">", "\"", "'", "&", "|", ";", "$", "`", "\\", "(", ")", "{", "}", "[", "]", "!", "~", "*", "?"}
      for _, char := range forbiddenChars {
          if strings.Contains(domain, char) {
              return false
          }
      }
      
      return true
  }
  ```

### 5.3 默认域名策略优化 🔍

- **问题**: 依赖硬编码的默认域名
- **风险**: 默认域名可能被屏蔽或失效
- **建议**: 考虑动态更新默认域名列表

## 6. 测试覆盖率分析 ✅

### 6.1 单元测试

- **状态**: ✅ 良好
- **分析**:
  - 覆盖了并发安全测试
  - 包含边界条件测试
  - 验证了轮询机制
  - 测试了空输入处理

### 6.2 集成测试

- **状态**: ✅ 完整
- **分析**:
  - 测试了与 Tgbot 的集成
  - 验证了文件读取和域名解析
  - 包含了去重机制测试

## 7. 修复建议

### 7.1 高优先级修复

1. **文件读取安全**:
   - 添加路径验证机制
   - 实现文件权限检查
   - 增加目录遍历防护

2. **域名格式验证**:
   - 实现域名格式正则验证
   - 添加恶意字符检查
   - 限制域名长度

### 7.2 中优先级建议

1. **增强输入验证**:
   - 验证文件内容格式
   - 检查域名可访问性
   - 实现域名黑名单机制

2. **监控和日志**:
   - 增加安全事件日志
   - 实现异常监控
   - 添加性能指标

### 7.3 低优先级优化

1. **性能优化**:
   - 实现域名缓存机制
   - 优化文件读取效率
   - 减少内存分配

2. **功能增强**:
   - 支持动态域名更新
   - 实现域名健康检查
   - 添加使用统计

## 8. 总体评估

### 安全性评分: 7.5/10

### 优点

- ✅ 并发安全实现优秀
- ✅ 没有明显的数据竞争
- ✅ 单元测试覆盖率良好
- ✅ 资源管理合理
- ✅ 代码结构清晰

### 需要改进的地方

- ⚠️ 文件读取存在安全漏洞
- ⚠️ 域名格式验证不足
- 🔍 默认域名策略可以优化
- 🔍 缺少安全监控机制

### 风险评估

- **数据竞争风险**: 无
- **并发安全风险**: 无
- **输入验证风险**: 中等
- **文件访问风险**: 中等
- **资源耗尽风险**: 无

## 9. 结论

`SNISelector` 的整体安全实现在并发和资源管理方面表现优秀，但存在文件读取安全和输入验证方面的不足。主要的安全风险集中在文件路径验证和域名格式检查上。建议优先修复文件读取安全问题，并加强域名格式验证。

**风险等级**: 中等
**建议优先级**: 高（文件安全）、中（输入验证）
**审查状态**: 基本安全，需要关键修复
**下次审查**: 修复完成后1个月内
