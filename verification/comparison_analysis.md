# SNI 和 Target 配置一致性对比分析

## 1. TG Bot 平台配置验证

### SNI 选择逻辑
- **函数**: `buildRealityInbound` 和 `buildXhttpRealityInbound`
- **逻辑**: 统一的SNI域名选择逻辑
- **优先级**:
  1. 优先使用 `t.serverService.GetNewSNI()`
  2. 回退机制：使用 `GetRealityDestinations()` 的随机选择
  3. 最终回退：使用安全的默认域名 "apple.com:443"

### Dest 字段格式
- **格式**: `"域名:443"`
- **示例**: `"apple.com:443"`, `"www.google.com:443"`

### serverNames 数组生成
- **函数**: `generateEnhancedServerNames`
- **逻辑**: 
  - 添加主域名
  - 添加常见的 www 子域名
  - 根据域名类型添加特定的子域名
  - 通用子域名（api., cdn., support.）
  - 去重并限制数量（最多8个）

## 2. 网页端平台配置验证

### SNI 选择逻辑
- **API**: `/server/getNewSNI`
- **实现**: `getNewSNI` 控制器
- **逻辑**: 
  1. 调用 `a.serverService.GetNewSNI()`
  2. 生成增强的 serverNames 列表

### Dest 字段格式
- **格式**: `"域名:443"`
- **实现**: 通过 `GetNewSNI()` 返回的域名 + ":443"

### serverNames 数组生成
- **函数**: `GenerateEnhancedServerNames`
- **逻辑**: 与TG Bot完全相同
- **代码位置**: `web/service/server_sni.go:214-255`

## 3. 一致性对比表

| 功能点 | TG Bot | 网页端 | 一致性 |
|--------|--------|--------|--------|
| SNI 选择逻辑 | 优先使用 serverService.GetNewSNI() | 调用 serverService.GetNewSNI() | ✅ 完全一致 |
| serverNames 生成 | generateEnhancedServerNames | GenerateEnhancedServerNames | ✅ 完全一致 |
| dest 字段格式 | "域名:443" | "域名:443" | ✅ 完全一致 |
| 域名验证 | 标准化域名格式 | 标准化域名格式 | ✅ 完全一致 |
| 错误处理 | 完整的回退机制 | 完整的回退机制 | ✅ 完全一致 |
| 地理位置支持 | 支持 | 支持 | ✅ 完全一致 |

## 4. 关键一致性确认

### 核心逻辑统一
- 两个平台都使用相同的 `ServerService.GetNewSNI()` 方法
- 都使用相同的 `GenerateEnhancedServerNames` 逻辑生成增强域名列表
- 都遵循相同的回退机制和错误处理

### 数据格式统一
- dest 字段都使用 "域名:443" 格式
- serverNames 数组都包含主域名、子域名和通用子域名
- 都实现去重和数量限制

### 地理位置支持统一
- 都支持基于服务器地理位置的SNI域名选择
- 都使用相同的GeoIP服务和域名文件读取逻辑

## 5. 验证结论

✅ **两个平台在SNI和Target配置方面完全一致**

- 核心逻辑统一
- 数据格式统一  
- 错误处理统一
- 功能特性统一

两个平台使用相同的底层服务和逻辑，确保了配置的一致性和可维护性。