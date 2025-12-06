# 数据库规范化设计文档：Client 表独立化

## 1. 背景与目标

当前 X-Panel 的数据库模型中，`Client`（客户端/用户）信息存储在 `Inbound` 表的 `settings` 字段中，格式为 JSON 字符串。这种设计导致了以下问题：
*   **查询效率低**：无法直接通过 SQL 查询特定客户端，必须加载并解析整个 JSON。
*   **数据完整性差**：缺乏数据库级别的约束（如外键、唯一性约束），容易产生脏数据。
*   **并发冲突**：更新单个客户端信息需要锁住整个 Inbound 记录，容易覆盖其他并发修改。
*   **扩展性受限**：难以对客户端添加新的属性或关联关系。

本设计方案的目标是将 `Client` 从 JSON 中独立出来，建立独立的 `Client` 表，并与 `Inbound` 表建立一对多关系。

## 2. 数据库 Schema 设计

### 2.1 新增 `Client` 表

我们将创建一个新的 `Client` 结构体映射到数据库表 `clients`。

```go
package model

// Client 独立后的客户端表结构
type Client struct {
    Id          int    `json:"id" form:"id" gorm:"primaryKey;autoIncrement"`
    InboundId   int    `json:"inboundId" form:"inboundId" gorm:"index"` // 外键，关联 Inbound 表
    
    // 核心认证与配置
    // 对于 VMess/VLESS，这是 UUID；对于 Trojan/Shadowsocks，这可能是密码或留空(视具体实现而定)
    // 建议统一字段名，或者保留原有的 ID/Password 区分
    Key         string `json:"id" form:"id"` // 对应原 JSON 中的 "id" (UUID)
    Password    string `json:"password" form:"password"` // 对应原 JSON 中的 "password"
    Security    string `json:"security" form:"security"`
    Flow        string `json:"flow" form:"flow"`
    Email       string `json:"email" form:"email" gorm:"uniqueIndex"` // 邮箱全局唯一，用于关联流量统计
    
    // 限制策略
    LimitIp     int    `json:"limitIp" form:"limitIp" gorm:"default:0"`
    TotalGB     int64  `json:"totalGB" form:"totalGB" gorm:"default:0"`
    ExpiryTime  int64  `json:"expiryTime" form:"expiryTime" gorm:"default:0"`
    SpeedLimit  int    `json:"speedLimit" form:"speedLimit" gorm:"default:0"` // KB/s
    
    // 状态与元数据
    Enable      bool   `json:"enable" form:"enable" gorm:"default:true"`
    TgID        int64  `json:"tgId" form:"tgId" gorm:"default:0"`
    SubID       string `json:"subId" form:"subId"`
    Reset       int    `json:"reset" form:"reset" gorm:"default:0"`
    Comment     string `json:"comment" form:"comment"`
    
    CreatedAt   int64  `json:"createdAt" gorm:"autoCreateTime:milli"` // 毫秒级时间戳
    UpdatedAt   int64  `json:"updatedAt" gorm:"autoUpdateTime:milli"` // 毫秒级时间戳
}
```

**索引设计：**
*   `InboundId`: 普通索引，用于快速查找某个入站下的所有用户。
*   `Email`: 唯一索引 (`uniqueIndex`)，保证邮箱全局唯一，这与现有的 `ClientTraffic` 表逻辑一致。

### 2.2 `Inbound` 表变更

`Inbound` 表结构本身不需要删除字段，但其 `Settings` 字段的内容将发生变化。

*   **字段保留**：`Settings` 字段保留，用于存储协议相关的非客户端配置（如 `fallbacks`, `network`, `security` 等）。
*   **内容变更**：`Settings` JSON 中的 `clients` 数组将被移除。

### 2.3 `ClientTraffic` 表关联

目前的 `ClientTraffic` 表结构如下：
```go
type ClientTraffic struct {
    Id         int    `json:"id" ...`
    InboundId  int    `json:"inboundId" ...`
    Email      string `json:"email" ... gorm:"unique"`
    // ... 其他统计字段
}
```
*   **关联方式**：继续使用 `Email` 作为关联键。虽然使用 `ClientId` 外键更规范，但考虑到 `ClientTraffic` 记录了历史流量，且 Xray 核心通常以 Email 标识用户统计，保持 `Email` 关联可以减少迁移的复杂性。
*   **外键约束**：可以在 `ClientTraffic` 上添加 `Email` 指向 `Client(Email)` 的外键约束（可选，视数据库引擎支持情况，SQLite 支持但默认可能未开启）。

## 3. 数据迁移策略

迁移过程需要保证数据不丢失且服务尽可能少中断。

### 3.1 迁移步骤

1.  **备份数据库**：在执行任何操作前，备份当前的 SQLite 数据库文件。
2.  **创建新表**：执行 `AutoMigrate(&model.Client{})` 创建 `clients` 表。
3.  **数据迁移脚本**：
    *   读取所有 `Inbound` 记录。
    *   遍历每条 `Inbound` 记录，解析 `Settings` JSON。
    *   提取 `clients` 数组中的每个对象。
    *   将提取出的客户端信息转换为 `model.Client` 实体，并设置 `InboundId`。
    *   插入到 `clients` 表中。
    *   **关键点**：如果发现重复 Email，记录错误或跳过（理论上现有逻辑已校验 Email 唯一性）。
4.  **清洗旧数据**：
    *   再次遍历 `Inbound` 记录。
    *   从 `Settings` JSON 中删除 `clients` 字段。
    *   更新 `Inbound` 记录。
5.  **验证**：
    *   比对迁移前后的客户端总数。
    *   随机抽取样本比对字段准确性。

### 3.2 数据一致性保障

*   **事务支持**：整个迁移过程（步骤 3 和 4）应在一个数据库事务中完成。如果任何一步失败，回滚所有更改。
*   **停机迁移**：建议在迁移期间停止面板的写操作（如添加/修改用户），或者在面板启动时的初始化阶段执行迁移。

## 4. 代码重构指导

### 4.1 Service 层重构 (`web/service/inbound.go`)

需要修改的方法主要集中在 `InboundService` 中：

*   **`GetInbound(id)` / `GetInbounds`**:
    *   需要使用 GORM 的 `Preload("Clients")` (需要在 `Inbound` struct 中定义 `Clients []Client` 关联) 或者手动查询 `clients` 表填充数据，以便前端获取完整的 Inbound 信息（包含 clients）。
    *   或者，改变 API 语义，`GetInbound` 只返回配置，客户端列表通过单独的 API 获取（推荐）。

*   **`AddInbound`**:
    *   先保存 `Inbound` 基本信息。
    *   再保存 `Clients` 到 `clients` 表。
    *   不再将 clients 序列化到 `Settings`。

*   **`UpdateInbound`**:
    *   更新 `Inbound` 基本信息。
    *   处理 `Settings` 更新（不含 clients）。
    *   **注意**：如果前端仍一次性提交包含 clients 的大 JSON，后端需要拆分处理；或者前端改为单独管理 clients。

*   **`AddInboundClient` / `DelInboundClient` / `UpdateInboundClient`**:
    *   **彻底重写**。
    *   不再读取-解析-修改-保存 JSON。
    *   改为直接执行 SQL `INSERT`, `DELETE`, `UPDATE` 操作 `clients` 表。
    *   这将极大简化逻辑并提高性能。

*   **`GetClients`**:
    *   改为 `db.Where("inbound_id = ?", inboundId).Find(&clients)`。

*   **`checkEmailExistForInbound` / `checkEmailsExistForClients`**:
    *   改为直接查询数据库 `db.Where("email = ?", email).Count(&count)`，效率极大提升。

### 4.2 前端 API 的潜在影响

*   **方案 A（最小改动）**：后端 API 保持输入输出格式不变。
    *   `GET /inbound/list`：后端在返回前，将 `clients` 表数据读出并组装回 `settings.clients` JSON 结构中（或者作为独立字段 `streamSettings` 同级的字段返回，前端需适配）。
    *   `POST /inbound/add`：后端接收包含 clients 的 JSON，自行拆分存储。
    *   **优点**：前端改动小。
    *   **缺点**：后端逻辑依然复杂，只是存储变了。

*   **方案 B（推荐，彻底重构）**：API 分离。
    *   `Inbound` 对象不再包含全量 `clients` 列表（或只包含摘要）。
    *   新增 `GET /inbound/:id/clients` 获取客户端列表。
    *   新增 `POST /client/add`, `POST /client/update`, `POST /client/del` 专门管理客户端。
    *   **优点**：符合 RESTful 设计，性能好，适合分页显示客户端（解决大量用户时的卡顿问题）。
    *   **缺点**：前端页面逻辑需要较大调整（从“编辑Inbound弹窗中管理用户”变为“独立的用户管理列表”）。

**建议**：第一阶段采用 **方案 A** 的变体。
*   后端 `Inbound` 结构体中增加 `Clients []Client` 字段（GORM 关联）。
*   JSON 序列化时，`clients` 作为 `Inbound` 的一级字段返回，而不是藏在 `settings` 字符串里。
*   前端需要调整：从 `inbound.settings.clients` 读取改为从 `inbound.clients` 读取。
*   对于增删改 Client 的 API，后端内部改为操作数据库，但对外接口参数保持兼容或微调。

## 5. 总结

此次重构将显著提升 X-Panel 的数据管理能力和性能。虽然涉及数据迁移和代码逻辑的较大变动，但对于项目的长期维护和扩展是必要的。建议优先完成数据库 Schema 变更和 Service 层的数据读写逻辑重写。