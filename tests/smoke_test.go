package tests

import (
	"os"
	"testing"

	"x-ui/bootstrap"

	"github.com/stretchr/testify/assert"
)

func TestSmokeLogin(t *testing.T) {
	// 1. 环境准备：重定向数据库和日志到临时目录，避免污染开发环境
	os.Setenv("XUI_DB_FOLDER", t.TempDir())
	os.Setenv("XUI_LOG_FOLDER", t.TempDir())
	os.Setenv("XUI_LOG_LEVEL", "debug")

	// 2. 初始化应用 (含 DB 迁移和 Wire 注入)
	app, err := bootstrap.Initialize()
	assert.NoError(t, err, "应用初始化不应有错误")
	assert.NotNil(t, app, "App 实例不应为 nil")

	// 3. 验证 DI 注入：检查核心 Service 和 Repo 是否已由 Wire 注入
	assert.NotNil(t, app.UserService, "UserService 应该已被 Wire 注入")
	assert.NotNil(t, app.UserRepo, "UserRepo 应该已被 Wire 注入")

	// 4. 执行冒烟测试：尝试使用默认凭证登录
	// 数据库初始化时会自动创建 admin/admin 用户
	user := app.UserService.CheckUser("admin", "admin", "")
	assert.NotNil(t, user, "使用默认 admin/admin 登录应该成功，证明 UserService -> UserRepository -> DB 链路通畅")
	assert.Equal(t, "admin", user.Username, "返回的用户名称应为 admin")

	t.Log("✅ 冒烟测试通过：Wire 依赖注入链路验证成功！")
}
