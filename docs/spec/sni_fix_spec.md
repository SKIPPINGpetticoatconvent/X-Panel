# 修复 Reality SNI 生成逻辑规格说明书

## 1. 背景与问题
用户报告在使用 TG Bot 一键配置 Reality 协议（VLESS + TCP + Reality 或 VLESS + XHTTP + Reality）时，如果选定的 Target（目标域名）已经包含 `www.` 前缀（例如 `www.walmart.com:443`），生成的配置中 `serverNames` 列表会出现错误的 `www.www.` 前缀（例如 `www.www.walmart.com`）。

## 2. 代码分析
经过检查 `web/service/tgbot.go`，问题出在 `buildRealityInbound` 和 `buildXhttpRealityInbound` 方法中。

当前逻辑如下：
```go
randomSni := strings.Split(randomDest, ":")[0]
// ...
"serverNames": []string{randomSni, "www." + randomSni},
```

当 `randomDest` 为 `www.walmart.com:443` 时：
1. `randomSni` 变为 `www.walmart.com`。
2. 列表第一个元素为 `www.walmart.com`。
3. 列表第二个元素为 `"www." + "www.walmart.com"` 即 `www.www.walmart.com`。

## 3. 解决方案
我们需要引入一个辅助函数 `GenerateRealityServerNames` 来智能处理 SNI 列表的生成。

### 逻辑规则
1. **去除端口**：首先从输入中去除端口号（如果有）。
2. **前缀检查**：
   - 如果域名以 `www.` 开头，则列表应包含 `[原域名, 去除www.的根域名]`。
   - 如果域名不以 `www.` 开头，则列表应包含 `[原域名, www. + 原域名]`。
3. **去重**：确保列表中没有重复项（虽然上述逻辑基本不会产生重复，除非域名本身就是 `www.`）。

## 4. 伪代码 (Pseudocode)

### 4.1 新增辅助函数 `GenerateRealityServerNames`

```go
// web/service/tgbot.go 或 web/service/util.go

// GenerateRealityServerNames 根据输入的 host 生成合理的 SNI 列表
// 输入: host (例如 "www.walmart.com:443", "google.com")
// 输出: []string (例如 ["www.walmart.com", "walmart.com"], ["google.com", "www.google.com"])
FUNCTION GenerateRealityServerNames(host STRING) RETURNS LIST OF STRING
    // 1. 去除端口
    VAR domain = Split(host, ":")[0]
    
    // 2. 初始化结果列表
    VAR serverNames = []
    
    // 3. 判断是否以 www. 开头
    IF HasPrefix(domain, "www.") THEN
        // 情况 A: 输入 www.walmart.com
        // 添加原始域名: www.walmart.com
        Append(serverNames, domain)
        
        // 添加根域名: walmart.com
        VAR rootDomain = TrimPrefix(domain, "www.")
        IF rootDomain != "" THEN
            Append(serverNames, rootDomain)
        END IF
    ELSE
        // 情况 B: 输入 walmart.com
        // 添加原始域名: walmart.com
        Append(serverNames, domain)
        
        // 添加 www 域名: www.walmart.com
        // 注意：对于多级子域名 (api.walmart.com)，这里也会生成 www.api.walmart.com，
        // 虽然不一定常用，但在 Reality 配置中通常是安全的或者是为了伪装。
        // 核心目标是避免 www.www.
        Append(serverNames, "www." + domain)
    END IF
    
    RETURN serverNames
END FUNCTION
```

### 4.2 修改 `buildRealityInbound` 和 `buildXhttpRealityInbound`

```go
// web/service/tgbot.go

METHOD buildRealityInbound(targetDest ...string)
    // ... (获取密钥、UUID 等逻辑保持不变) ...

    // 获取 randomDest (Target)
    // ... (获取 randomDest 逻辑保持不变) ...
    
    // --- 修改开始 ---
    // 旧代码:
    // randomSni := strings.Split(randomDest, ":")[0]
    // ...
    // "serverNames": []string{randomSni, "www." + randomSni},
    
    // 新代码:
    VAR serverNamesList = t.GenerateRealityServerNames(randomDest)
    VAR randomSni = serverNamesList[0] // 使用列表第一个作为主 SNI
    
    // ...
    
    // 在 settings JSON 中使用 serverNamesList
    "realitySettings": map[string]any{
        // ...
        "serverNames": serverNamesList,
        // ...
    }
    // --- 修改结束 ---
    
    // ... (其余逻辑保持不变) ...
END METHOD

METHOD buildXhttpRealityInbound(targetDest ...string)
    // ... (类似修改) ...
    VAR serverNamesList = t.GenerateRealityServerNames(randomDest)
    VAR randomSni = serverNamesList[0]
    
    // ...
    
    "realitySettings": map[string]any{
        // ...
        "serverNames": serverNamesList,
        // ...
    }
    // ...
END METHOD
```

## 5. TDD 锚点 (Test Driven Development)

在实现代码之前，必须先编写测试用例。建议创建 `web/service/tgbot_sni_test.go`。

### 测试用例规格

```go
// web/service/tgbot_sni_test.go

TEST GenerateRealityServerNames
    CASE "Standard Domain":
        INPUT: "google.com"
        EXPECTED: ["google.com", "www.google.com"]
        
    CASE "Domain with Port":
        INPUT: "google.com:443"
        EXPECTED: ["google.com", "www.google.com"]
        
    CASE "WWW Domain":
        INPUT: "www.walmart.com"
        EXPECTED: ["www.walmart.com", "walmart.com"]
        
    CASE "WWW Domain with Port":
        INPUT: "www.walmart.com:443"
        EXPECTED: ["www.walmart.com", "walmart.com"]
        
    CASE "Subdomain":
        INPUT: "api.walmart.com"
        EXPECTED: ["api.walmart.com", "www.api.walmart.com"]
        
    CASE "Mixed Case (Should handle or assume lowercase input? Usually input is lowercase from selection)":
        INPUT: "Google.com"
        EXPECTED: ["Google.com", "www.Google.com"] (保持原样或转小写，视具体需求，通常保持原样即可)
END TEST
```

## 6. 约束条件
- **不硬编码环境变量**：所有配置应来自函数参数或服务方法调用。
- **模块化**：SNI 生成逻辑应封装在独立函数中，便于复用和测试。
- **安全性**：不涉及密钥变更，仅涉及 SNI 列表生成。
