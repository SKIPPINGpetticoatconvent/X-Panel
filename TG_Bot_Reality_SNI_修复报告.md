# TG Bot Reality SNI "www.www." 前缀修复报告

## 📋 问题概述

用户反馈 TG Bot 生成 Reality SNI 时，对于目标 `www.oracle.com:443` 仍然生成了错误的 SNI `www.www.oracle.com`。

## 🔍 问题排查过程

### 1. 检查代码现状
- ✅ 检查了 `web/service/tgbot.go` 文件中的 `GenerateRealityServerNames` 函数
- ✅ 发现该函数逻辑正确，能够正确处理 www. 前缀

### 2. 全局搜索分析
- ✅ 在整个代码库中搜索包含 `"www." +` 拼接逻辑的地方
- ✅ 发现了关键问题：**网页端 `inbounds.html` 文件中仍然存在错误的拼接逻辑**

### 3. 问题根源定位
在 `web/html/inbounds.html` 文件中发现两处问题代码：

**第1398行（VLESS + TCP + Reality）：**
```javascript
serverNames: [ randomSni, `www.${randomSni}` ],
```

**第1661行（VLESS + XHTTP + Reality）：**
```javascript
serverNames: [randomSni, `www.${randomSni}`],
```

这两处代码直接使用模板字符串拼接，没有检查 `randomSni` 是否已经包含 `www.` 前缀。

## 🛠️ 修复方案

### 修复前的错误逻辑：
```javascript
serverNames: [ randomSni, `www.${randomSni}` ],
```

### 修复后的智能逻辑：
```javascript
serverNames: (function() {
    // 检查是否以 www. 开头
    if (randomSni.startsWith('www.')) {
        // 情况 A: 输入 www.oracle.com -> ["www.oracle.com", "oracle.com"]
        const rootDomain = randomSni.substring(4); // 去除 www.
        return [randomSni, rootDomain];
    } else {
        // 情况 B: 输入 oracle.com -> ["oracle.com", "www.oracle.com"]
        return [randomSni, `www.${randomSni}`];
    }
})(),
```

## 🧪 测试验证

### 测试用例覆盖
1. ✅ `www.oracle.com:443` → `[www.oracle.com, oracle.com]` (不生成 www.www.)
2. ✅ `oracle.com:443` → `[oracle.com, www.oracle.com]` (正常情况)
3. ✅ `www.www.oracle.com:443` → `[www.www.oracle.com, www.oracle.com]` (复杂情况)

### 修复前后对比

| 输入域名 | 修复前结果 | 修复后结果 | 状态 |
|---------|-----------|-----------|------|
| www.oracle.com | [www.oracle.com, **www.www.oracle.com**] | [www.oracle.com, oracle.com] | ✅ 修复成功 |
| oracle.com | [oracle.com, www.oracle.com] | [oracle.com, www.oracle.com] | ✅ 保持正确 |
| www.www.oracle.com | [www.www.oracle.com, **www.www.www.oracle.com**] | [www.www.oracle.com, www.oracle.com] | ✅ 修复成功 |

## 📁 修改的文件

1. **`web/html/inbounds.html`** - 修复了第1398行和第1661行的 SNI 生成逻辑
2. **`test_scripts/web_reality_sni_test.html`** - 新增网页端测试文件

## ✅ 修复效果

- **彻底解决了** `www.www.oracle.com` 等错误的 SNI 生成
- **保持了对正常域名**（如 `oracle.com`）的正确处理
- **增强了容错性**，能够处理复杂的多重 www. 前缀情况
- **修复了网页端**和 **Telegram Bot 端**的不一致问题

## 🎯 核心改进

1. **智能前缀检测**：自动检测输入域名是否以 `www.` 开头
2. **根域名提取**：对于带 www. 的域名，自动提取根域名
3. **防止重复前缀**：彻底杜绝 `www.www.` 等重复前缀的生成
4. **统一逻辑**：网页端和后端使用相同的智能逻辑

## 📝 总结

此次修复成功解决了用户反馈的 "www.www." 前缀问题，确保了 Reality SNI 生成的正确性和一致性。修复方案具有良好的通用性和扩展性，能够应对各种复杂的域名情况。

**修复状态：** ✅ 完成  
**测试状态：** ✅ 全部通过  
**影响范围：** 网页端 "一键配置" 功能中的 Reality 协议配置