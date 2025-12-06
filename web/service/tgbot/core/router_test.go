package core

import (
	"testing"

	"github.com/mymmrac/telego"
	"github.com/stretchr/testify/assert"
)

// TestRouter_NewRouter 测试路由器的创建
func TestRouter_NewRouter(t *testing.T) {
	ctx := NewContext()
	router := NewRouter(ctx)

	assert.NotNil(t, router)
	assert.Equal(t, 0, len(router.commandHandlers))
	assert.Equal(t, 0, len(router.callbackHandlers))
	assert.Equal(t, ctx, router.ctx)
}

// TestRouter_RegisterCommand 测试命令注册
func TestRouter_RegisterCommand(t *testing.T) {
	ctx := NewContext()
	router := NewRouter(ctx)

	handler := func(ctx *Context, message telego.Message, isAdmin bool) error {
		return nil
	}

	router.RegisterCommand("test", handler)

	assert.Equal(t, 1, len(router.commandHandlers))
	assert.NotNil(t, router.commandHandlers["test"])
}

// TestRouter_RegisterCallback 测试回调注册
func TestRouter_RegisterCallback(t *testing.T) {
	ctx := NewContext()
	router := NewRouter(ctx)

	handler := func(ctx *Context, query telego.CallbackQuery, isAdmin bool) error {
		return nil
	}

	router.RegisterCallback("test_callback", handler)

	assert.Equal(t, 1, len(router.callbackHandlers))
	assert.NotNil(t, router.callbackHandlers["test_callback"])
}

// TestRouter_SetContext 测试设置上下文
func TestRouter_SetContext(t *testing.T) {
	ctx := NewContext()
	router := NewRouter(ctx)

	newCtx := NewContext()
	router.SetContext(newCtx)

	assert.Equal(t, newCtx, router.ctx)
}

// TestRouter_RegisterCommandForExternal 测试外部命令注册
func TestRouter_RegisterCommandForExternal(t *testing.T) {
	ctx := NewContext()
	router := NewRouter(ctx)

	// 模拟外部处理器
	externalHandler := func(ctx ContextInterface, message telego.Message, isAdmin bool) error {
		return nil
	}

	// 注册外部命令
	router.RegisterCommandForExternal("external_test", externalHandler)

	// 验证命令已注册
	assert.NotNil(t, router.commandHandlers["external_test"])
}

// TestRouter_RegisterCommandExt 测试外部命令注册接口
func TestRouter_RegisterCommandExt(t *testing.T) {
	ctx := NewContext()
	router := NewRouter(ctx)

	externalHandler := func(ctx ContextInterface, message telego.Message, isAdmin bool) error {
		return nil
	}

	// 测试 RegisterCommandExt 方法
	router.RegisterCommandExt("ext_cmd", externalHandler)

	// 验证命令已注册
	assert.NotNil(t, router.commandHandlers["ext_cmd"])
}

// TestRouter_CommandHandlerRegistration 测试命令处理器注册逻辑
func TestRouter_CommandHandlerRegistration(t *testing.T) {
	ctx := NewContext()
	router := NewRouter(ctx)

	// 测试多个命令注册
	cmdHandler := func(ctx *Context, message telego.Message, isAdmin bool) error { 
		return nil 
	}

	router.RegisterCommand("cmd1", cmdHandler)
	router.RegisterCommand("cmd2", cmdHandler)
	router.RegisterCommand("cmd3", cmdHandler)

	// 验证所有命令都已注册
	assert.Equal(t, 3, len(router.commandHandlers))
	assert.NotNil(t, router.commandHandlers["cmd1"])
	assert.NotNil(t, router.commandHandlers["cmd2"])
	assert.NotNil(t, router.commandHandlers["cmd3"])
}

// TestRouter_CallbackHandlerRegistration 测试回调处理器注册逻辑
func TestRouter_CallbackHandlerRegistration(t *testing.T) {
	ctx := NewContext()
	router := NewRouter(ctx)

	// 测试多个回调注册
	callbackHandler := func(ctx *Context, query telego.CallbackQuery, isAdmin bool) error { 
		return nil 
	}

	router.RegisterCallback("callback1", callbackHandler)
	router.RegisterCallback("callback2", callbackHandler)
	router.RegisterCallback("callback3", callbackHandler)

	// 验证所有回调都已注册
	assert.Equal(t, 3, len(router.callbackHandlers))
	assert.NotNil(t, router.callbackHandlers["callback1"])
	assert.NotNil(t, router.callbackHandlers["callback2"])
	assert.NotNil(t, router.callbackHandlers["callback3"])
}

// TestRouter_CommandOverride 测试命令覆盖
func TestRouter_CommandOverride(t *testing.T) {
	ctx := NewContext()
	router := NewRouter(ctx)

	cmdHandler := func(ctx *Context, message telego.Message, isAdmin bool) error { 
		return nil 
	}

	// 注册第一个处理器
	router.RegisterCommand("test_cmd", cmdHandler)

	// 验证第一个处理器已注册
	assert.NotNil(t, router.commandHandlers["test_cmd"])

	// 注册第二个处理器（覆盖第一个）
	router.RegisterCommand("test_cmd", cmdHandler)

	// 验证第二个处理器覆盖了第一个
	assert.NotNil(t, router.commandHandlers["test_cmd"])
	// 这里我们不能直接比较函数，但可以验证数量没有增加
	assert.Equal(t, 1, len(router.commandHandlers))
}

// TestRouter_EmptyRouterBehavior 测试空路由器行为
func TestRouter_EmptyRouterBehavior(t *testing.T) {
	ctx := NewContext()
	router := NewRouter(ctx)

	// 验证初始状态
	assert.Equal(t, 0, len(router.commandHandlers))
	assert.Equal(t, 0, len(router.callbackHandlers))
	assert.NotNil(t, router.ctx)
}

// TestRouter_ContextSetting 测试上下文设置
func TestRouter_ContextSetting(t *testing.T) {
	ctx1 := NewContext()
	ctx2 := NewContext()
	
	router := NewRouter(ctx1)
	
	// 验证初始上下文
	assert.Equal(t, ctx1, router.ctx)
	
	// 设置新上下文
	router.SetContext(ctx2)
	
	// 验证上下文已更新
	assert.Equal(t, ctx2, router.ctx)
}

// TestRouter_ExternalCommandRegistryInterface 测试外部命令注册接口
func TestRouter_ExternalCommandRegistryInterface(t *testing.T) {
	ctx := NewContext()
	router := NewRouter(ctx)

	// 验证路由器实现了 ExternalCommandRegistry 接口
	var _ ExternalCommandRegistry = router

	// 测试 RegisterCommandExt 方法
	externalHandler := func(ctx ContextInterface, message telego.Message, isAdmin bool) error {
		return nil
	}

	router.RegisterCommandExt("external_cmd", externalHandler)
	assert.NotNil(t, router.commandHandlers["external_cmd"])
}

// TestRouter_RegisterCommonCommandHandlers 测试注册通用命令处理器
func TestRouter_RegisterCommonCommandHandlers(t *testing.T) {
	ctx := NewContext()
	router := NewRouter(ctx)

	// 调用注册通用命令处理器方法
	router.RegisterCommonCommandHandlers()

	// 验证方法执行成功（没有panic）
	assert.NotNil(t, router)
}

// TestRouter_SetupCommonHandlers 测试设置通用处理器
func TestRouter_SetupCommonHandlers(t *testing.T) {
	ctx := NewContext()
	router := NewRouter(ctx)

	// 调用设置通用处理器方法
	router.SetupCommonHandlers()

	// 验证方法执行成功（没有panic）
	assert.NotNil(t, router)
}

// TestRouter_CommandHandlerMapIntegrity 测试命令处理器映射完整性
func TestRouter_CommandHandlerMapIntegrity(t *testing.T) {
	ctx := NewContext()
	router := NewRouter(ctx)

	// 验证初始映射是空的
	assert.NotNil(t, router.commandHandlers)
	assert.NotNil(t, router.callbackHandlers)
	assert.Equal(t, 0, len(router.commandHandlers))
	assert.Equal(t, 0, len(router.callbackHandlers))

	// 注册一些命令和回调
	cmdHandler := func(ctx *Context, message telego.Message, isAdmin bool) error { return nil }
	callbackHandler := func(ctx *Context, query telego.CallbackQuery, isAdmin bool) error { return nil }

	router.RegisterCommand("test_cmd", cmdHandler)
	router.RegisterCallback("test_callback", callbackHandler)

	// 验证映射完整性
	assert.Equal(t, 1, len(router.commandHandlers))
	assert.Equal(t, 1, len(router.callbackHandlers))
	
	// 验证映射不为nil
	assert.NotNil(t, router.commandHandlers["test_cmd"])
	assert.NotNil(t, router.callbackHandlers["test_callback"])
}

// TestRouter_DefaultCommandsRegistration 测试默认命令注册功能
func TestRouter_DefaultCommandsRegistration(t *testing.T) {
	ctx := NewContext()
	router := NewRouter(ctx)

	// 测试注册默认命令而不调用具体服务方法
	// 这里我们只验证方法可以调用而不panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("RegisterDefaultCommands panicked: %v", r)
		}
	}()

	// 由于无法创建真实的 service 实例，我们只测试方法签名
	// 这个测试主要验证方法不会panic
	assert.NotPanics(t, func() {
		// 空指针调用不会panic，只是会在实际使用时panic
		// 这里我们只测试方法存在且可调用
		router.RegisterCommonCommandHandlers()
	})
}

// TestRouter_DefaultCallbacksRegistration 测试默认回调注册功能
func TestRouter_DefaultCallbacksRegistration(t *testing.T) {
	ctx := NewContext()
	router := NewRouter(ctx)

	// 测试注册默认回调而不调用具体服务方法
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("RegisterDefaultCallbacks panicked: %v", r)
		}
	}()

	// 这个测试主要验证方法不会panic
	assert.NotPanics(t, func() {
		router.SetupCommonHandlers()
	})
}

// TestRouter_CommandHandlerType 测试命令处理器类型定义
func TestRouter_CommandHandlerType(t *testing.T) {
	// 验证 CommandHandler 类型定义正确
	var handler CommandHandler = func(ctx *Context, message telego.Message, isAdmin bool) error {
		return nil
	}
	
	assert.NotNil(t, handler)
	
	// 验证 CallbackHandler 类型定义正确
	var callbackHandler CallbackHandler = func(ctx *Context, query telego.CallbackQuery, isAdmin bool) error {
		return nil
	}
	
	assert.NotNil(t, callbackHandler)
	
	// 验证 CommandHandlerFunc 类型定义正确
	var externalHandler CommandHandlerFunc = func(ctx ContextInterface, message telego.Message, isAdmin bool) error {
		return nil
	}
	
	assert.NotNil(t, externalHandler)
}

// TestRouter_RouterStruct 路由器结构体测试
func TestRouter_RouterStruct(t *testing.T) {
	ctx := NewContext()
	router := NewRouter(ctx)

	// 验证路由器结构体字段
	assert.NotNil(t, router.commandHandlers)
	assert.NotNil(t, router.callbackHandlers)
	assert.NotNil(t, router.ctx)
	
	// 验证映射类型
	assert.IsType(t, make(map[string]CommandHandler), router.commandHandlers)
	assert.IsType(t, make(map[string]CallbackHandler), router.callbackHandlers)
	assert.IsType(t, (*Context)(nil), router.ctx)
}