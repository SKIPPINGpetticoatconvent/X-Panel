---
name: xpanel-gorm-database
description: X-Panel Gorm 数据库操作模式。在创建模型、查询数据、处理事务或优化 SQLite 时使用。
---

# X-Panel Gorm 数据库模式

## 数据库配置

位置: `database/db.go`

### 初始化

```go
func InitDB(dbPath string) error {
    db, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{
        Logger: logger.Default,
    })
    
    // 连接池配置
    sqlDB, _ := db.DB()
    sqlDB.SetMaxOpenConns(25)
    sqlDB.SetMaxIdleConns(5)
    sqlDB.SetConnMaxLifetime(5 * time.Minute)
    
    // SQLite WAL 模式优化
    db.Exec("PRAGMA journal_mode=WAL;")
    db.Exec("PRAGMA synchronous=NORMAL;")
    
    return initModels()
}
```

## 模型定义

位置: `database/model/`

```go
package model

type Inbound struct {
    Id          int    `json:"id" gorm:"primaryKey;autoIncrement"`
    UserId      int    `json:"userId"`
    Up          int64  `json:"up"`
    Down        int64  `json:"down"`
    Total       int64  `json:"total"`
    Remark      string `json:"remark"`
    Enable      bool   `json:"enable"`
    ExpiryTime  int64  `json:"expiryTime"`
    Listen      string `json:"listen"`
    Port        int    `json:"port" gorm:"unique"`
    Protocol    string `json:"protocol"`
    Settings    string `json:"settings"`
    StreamSettings string `json:"streamSettings"`
    Tag         string `json:"tag" gorm:"unique"`
    Sniffing    string `json:"sniffing"`
    Allocate    string `json:"allocate"`
}

func (i *Inbound) TableName() string {
    return "inbounds"
}
```

## 常用查询模式

### 获取数据库实例

```go
import "x-ui/database"

db := database.GetDB()
```

### 查询单条

```go
var inbound model.Inbound
err := db.First(&inbound, id).Error
if database.IsNotFound(err) {
    // 未找到记录
}
```

### 查询多条

```go
var inbounds []*model.Inbound
err := db.Where("enable = ?", true).Find(&inbounds).Error
```

### 条件查询

```go
// 链式条件
db.Where("port = ?", port).
   Where("protocol = ?", "vmess").
   Find(&inbounds)

// 多条件 OR
db.Where("port = ? OR protocol = ?", port, protocol).Find(&inbounds)
```

### 创建记录

```go
inbound := &model.Inbound{
    Port:     12345,
    Protocol: "vmess",
    Enable:   true,
}
err := db.Create(inbound).Error
```

### 更新记录

```go
// 更新单个字段
db.Model(&inbound).Update("enable", false)

// 更新多个字段
db.Model(&inbound).Updates(map[string]interface{}{
    "enable": false,
    "remark": "disabled",
})

// 保存整个结构体
db.Save(&inbound)
```

### 删除记录

```go
db.Delete(&model.Inbound{}, id)
```

## 事务处理

```go
err := db.Transaction(func(tx *gorm.DB) error {
    if err := tx.Create(&inbound).Error; err != nil {
        return err  // 回滚
    }
    if err := tx.Create(&client).Error; err != nil {
        return err  // 回滚
    }
    return nil  // 提交
})
```

## 自动迁移

```go
func initModels() error {
    models := []any{
        &model.User{},
        &model.Inbound{},
        &model.Setting{},
    }
    for _, model := range models {
        if err := db.AutoMigrate(model); err != nil {
            return err
        }
    }
    return nil
}
```

## SQLite 特定优化

```go
// WAL 模式 (Write-Ahead Logging)
db.Exec("PRAGMA journal_mode=WAL;")

// 同步模式
db.Exec("PRAGMA synchronous=NORMAL;")

// 检查点
db.Exec("PRAGMA wal_checkpoint;")

// 完整性检查
var res string
db.Raw("PRAGMA integrity_check;").Scan(&res)
```

## 最佳实践

1. **始终检查错误**: 使用 `database.IsNotFound()` 区分未找到和其他错误
2. **使用参数化查询**: 避免 SQL 注入
3. **事务包裹**: 多个写操作使用事务
4. **避免 N+1**: 使用 `Preload()` 预加载关联
5. **连接池**: 使用项目配置的连接池设置
