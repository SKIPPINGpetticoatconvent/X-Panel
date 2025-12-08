# SNI 域名不重复选择规格说明 (Specification)

## 1. 背景与目标
目前 TG Bot 的“一键配置”功能在生成 Reality/TLS 节点时，采用完全随机的方式从列表中选择 SNI 域名。这导致用户在连续创建多个节点时，可能会遇到重复的 SNI，降低了配置的多样性和抗封锁能力。

本规格说明旨在设计一个 **SNI 轮询选择器 (SNI Round-Robin Selector)**，确保在给定的 SNI 列表范围内，连续生成的节点尽可能使用不同的域名。

## 2. 核心需求
*   **全局轮询**：在 Bot 运行期间，维护一个全局的 SNI 使用状态。
*   **自动重置**：当所有可用 SNI 都被使用过一次后，自动重新开始新的一轮（可选择重新洗牌）。
*   **线程安全**：支持并发请求（虽然 TG Bot 主要是串行处理消息，但作为服务应保证并发安全）。
*   **回退机制**：如果列表为空，应有合理的默认行为。

## 3. 架构设计

### 3.1 数据结构
引入 `SNISelector` 结构体，负责管理 SNI 列表和选择逻辑。

```go
type SNISelector struct {
    domains []string // 当前可用的域名列表
    index   int      // 当前读取到的索引
    mu      sync.Mutex // 互斥锁，保证并发安全
}
```

### 3.2 接口定义
在 `Tgbot` 或相关服务中集成 `SNISelector`。

*   `NewSNISelector(domains []string) *SNISelector`: 初始化选择器。
*   `Next() string`: 获取下一个不重复的 SNI。

### 3.3 交互流程
1.  Bot 启动或加载 SNI 列表时，初始化 `SNISelector`。
2.  当用户请求 `/oneclick` 生成节点时：
    *   调用 `SNISelector.Next()` 获取 SNI。
    *   使用获取到的 SNI 构建 `Inbound` 配置。
3.  `SNISelector` 内部：
    *   检查 `index` 是否越界。
    *   如果越界（`index >= len(domains)`），重置 `index = 0` （可选：重新洗牌 `domains` 以增加随机性）。
    *   返回 `domains[index]`，并执行 `index++`。

## 4. 约束条件
*   **无持久化要求**：重启后重置为初始状态即可，无需数据库存储状态。
*   **性能**：操作应在纳秒/微秒级完成，不阻塞主线程。
*   **配置源**：SNI 列表来源保持不变（`serverService` 或硬编码列表），本模块只负责“选择”逻辑。

---

# 伪代码 (Pseudocode)

## 1. SNI Selector 模块

```go
// web/service/sni_selector.go (拟定)

package service

import (
    "sync"
    "x-ui/util/common" // 假设有随机工具
)

// TDD Anchor: TestSNISelector_Next_RoundRobin
// 验证 Next() 方法是否按顺序返回域名，并在耗尽后循环
type SNISelector struct {
    domains []string
    index   int
    mu      sync.Mutex
}

// NewSNISelector 初始化选择器
// TDD Anchor: TestNewSNISelector_Shuffle
// 验证初始化时是否可以对列表进行洗牌（可选）
func NewSNISelector(initialDomains []string) *SNISelector {
    // 复制切片以防外部修改
    // 如果列表为空，提供默认值防止 panic
    if len(initialDomains) == 0 {
        initialDomains = []string{"www.google.com", "www.amazon.com"} 
    }
    
    s := &SNISelector{
        domains: make([]string, len(initialDomains)),
        index:   0,
    }
    copy(s.domains, initialDomains)
    
    // 初始化时洗牌，避免每次启动顺序都完全一样
    s.shuffle()
    
    return s
}

// Next 返回下一个 SNI
func (s *SNISelector) Next() string {
    s.mu.Lock()
    defer s.mu.Unlock()

    if len(s.domains) == 0 {
        return "" // Should not happen due to init check
    }

    // 检查是否需要重置
    if s.index >= len(s.domains) {
        s.index = 0
        // TDD Anchor: TestSNISelector_Reshuffle_On_Reset
        // 验证一轮结束后是否重新洗牌
        s.shuffle()
    }

    domain := s.domains[s.index]
    s.index++
    
    return domain
}

// shuffle 内部方法：打乱列表顺序
func (s *SNISelector) shuffle() {
    // 使用 Fisher-Yates 洗牌算法或类似逻辑
    // common.Shuffle(s.domains) 
}

// UpdateDomains 允许运行时更新列表
func (s *SNISelector) UpdateDomains(newDomains []string) {
    s.mu.Lock()
    defer s.mu.Unlock()
    
    if len(newDomains) == 0 {
        return
    }
    
    s.domains = make([]string, len(newDomains))
    copy(s.domains, newDomains)
    s.index = 0
    s.shuffle()
}
```

## 2. 集成到 Tgbot

```go
// web/service/tgbot.go

type Tgbot struct {
    // ... existing fields
    sniSelector *SNISelector // 新增字段
}

// 初始化
func NewTgBot(...) *Tgbot {
    t := &Tgbot{...}
    
    // 获取初始域名列表
    initialDomains := t.GetRealityDestinations() 
    t.sniSelector = NewSNISelector(initialDomains)
    
    return t
}

// 修改 GetRealityDestinations (可选，或者直接在 build 方法中使用 selector)
// 建议保持 GetRealityDestinations 原样（作为数据源），
// 而在 buildRealityInbound 中使用 selector。

// 修改 buildRealityInbound
func (t *Tgbot) buildRealityInbound(targetDest ...string) (...) {
    // ...
    
    var randomDest string
    if len(targetDest) > 0 && targetDest[0] != "" {
        randomDest = targetDest[0]
    } else {
        // OLD: randomDest = realityDests[common.RandomInt(len(realityDests))]
        
        // NEW: 使用选择器
        // 确保 selector 已初始化（防止空指针）
        if t.sniSelector == nil {
             // Fallback logic
             dests := t.GetRealityDestinations()
             t.sniSelector = NewSNISelector(dests)
        }
        
        randomDest = t.sniSelector.Next()
    }
    
    // ...
}

// 同理修改 buildXhttpRealityInbound
```

## 3. 测试计划 (TDD Anchors)

1.  **`TestSNISelector_Basic`**:
    *   输入: `["a.com", "b.com"]`
    *   操作: 连续调用 `Next()` 3次。
    *   期望: 输出应覆盖 "a.com", "b.com"，第三次调用应返回 "a.com" 或 "b.com" (取决于洗牌后的顺序，但前两次必须不同)。

2.  **`TestSNISelector_EmptyInit`**:
    *   输入: `[]`
    *   期望: `Next()` 返回默认安全值，不 Panic。

3.  **`TestSNISelector_Concurrency`**:
    *   操作: 启动 10 个 goroutine 同时调用 `Next()`。
    *   期望: 无数据竞争 (Data Race)，所有调用成功返回。

4.  **`TestTgbot_Integration`**:
    *   操作: 模拟调用 `remoteCreateOneClickInbound` 多次。
    *   期望: 生成的 `Inbound` 配置中，SNI 字段在短时间内不重复。