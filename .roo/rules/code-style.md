# 代码风格规范 (Code Style Guide)

## Go 编码规范

### 命名约定

| 类型 | 规范 | 示例 |
|------|------|------|
| **包名** | 小写单词，简短有意义 | `service`, `controller`, `util` |
| **私有变量/函数** | camelCase | `userCount`, `getConfig` |
| **公开变量/函数** | PascalCase | `NewServer`, `UserService` |
| **常量** | PascalCase 或全大写 | `MaxRetries`, `DEFAULT_PORT` |
| **接口** | 通常以 `er` 结尾 | `Reader`, `Writer`, `Handler` |
| **结构体** | PascalCase，名词 | `User`, `InboundConfig` |

### 错误处理

**基本原则**: 错误必须立即检查，禁止忽略

```go
// ✅ 正确: 立即检查并包装错误上下文
result, err := someOperation()
if err != nil {
    return fmt.Errorf("operation failed: %w", err)
}

// ❌ 错误: 忽略错误
result, _ := someOperation()
```

**自定义错误类型**: 用于领域特定错误

```go
// 定义自定义错误
type ValidationError struct {
    Field   string
    Message string
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("validation failed for %s: %s", e.Field, e.Message)
}

// 使用自定义错误
if input == "" {
    return nil, &ValidationError{Field: "username", Message: "cannot be empty"}
}
```

### 并发编程

**Context 使用**: 所有长时操作必须接受 Context 参数

```go
// ✅ 正确: 使用 context 控制超时和取消
func (s *Service) FetchData(ctx context.Context, id string) (*Data, error) {
    select {
    case <-ctx.Done():
        return nil, ctx.Err()
    case result := <-s.fetchAsync(id):
        return result, nil
    }
}
```

**Channel 通信**: 通过通信共享内存，而非通过共享内存通信

```go
// ✅ 正确: 使用 channel 传递数据
results := make(chan Result)
go func() {
    results <- processData(data)
}()
result := <-results

// ❌ 避免: 直接共享内存（除非有明确的同步机制）
```

### 代码格式化

- **工具**: 使用 `dprint fmt` 或 `gofmt` 格式化代码
- **导入分组**: 标准库、第三方库、项目内部包分开，用空行隔开

```go
import (
    // 标准库
    "context"
    "fmt"
    "net/http"

    // 第三方库
    "github.com/gin-gonic/gin"
    "gorm.io/gorm"

    // 项目内部包
    "x-ui/database"
    "x-ui/web/service"
)
```

## Web/前端规范

### JavaScript 规范

- **格式化**: 遵循 `biome.json` 配置
- **文件组织**: 按功能模块划分，放置于 `web/assets/js/`
- **命名**: 使用 camelCase

```javascript
// ✅ 正确
const fetchUserData = async (userId) => {
    const response = await fetch(`/api/user/${userId}`);
    return response.json();
};

// ❌ 避免
function fetch_user_data(user_id) { ... }
```

### HTML 模板规范

- **文件位置**: `web/html/` 目录下按页面/组件组织
- **命名**: 使用小写字母和连字符 (`user-list.html`)
- **结构**: 保持语义化 HTML，合理使用标签

```html
<!-- ✅ 正确: 语义化结构 -->
<section class="user-panel">
    <header class="panel-header">
        <h2>用户管理</h2>
    </header>
    <main class="panel-content">
        <!-- 内容 -->
    </main>
</section>
```

### CSS 规范

- **命名**: 使用 BEM 命名法或简洁的类名
- **组织**: 按组件/页面分离样式文件

## 测试规范

### 单元测试

- **文件命名**: `*_test.go`
- **位置**: 与被测试代码同目录
- **覆盖要求**: 新业务逻辑必须编写测试

```go
// service/user_test.go
func TestUserService_Create(t *testing.T) {
    // Arrange
    svc := NewUserService(mockDB)
    
    // Act
    user, err := svc.Create("testuser", "password123")
    
    // Assert
    assert.NoError(t, err)
    assert.NotEmpty(t, user.ID)
}
```

### E2E 测试

- **位置**: `tests/e2e/` 目录
- **覆盖**: 关键用户路径（登录、节点添加、流量查看）

## 提交规范

### Commit Message 格式

遵循 [Conventional Commits](https://www.conventionalcommits.org/) 规范：

```
<type>(<scope>): <description>

[optional body]

[optional footer]
```

**类型 (type)**:
| 类型 | 描述 |
|------|------|
| `feat` | 新功能 |
| `fix` | Bug 修复 |
| `docs` | 文档变更 |
| `style` | 代码格式（不影响逻辑） |
| `refactor` | 重构（非功能/修复） |
| `test` | 测试相关 |
| `chore` | 构建/工具变更 |

**示例**:
```
feat(inbound): add support for REALITY protocol

fix(auth): resolve session timeout issue

docs(readme): update installation instructions
```
