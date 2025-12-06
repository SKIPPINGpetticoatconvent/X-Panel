package core

import (
	"context"
	"crypto/rand"
	"embed"
	"errors"
	"fmt"
	"math/big"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"x-ui/logger"
	"x-ui/web/global"

	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttpproxy"
)

// Bot 生命周期管理接口
type Bot interface {
	// 生命周期管理
	Start(i18nFS embed.FS, settingService interface{}) error
	Stop()
	Restart() error
	IsRunning() bool

	// 消息发送
	SendMessage(chatId int64, message string, replyMarkup ...telego.ReplyMarkup) error
	SendMessageToAdmins(message string, replyMarkup ...telego.ReplyMarkup) error

	// 获取 Bot 信息
	GetBot() *telego.Bot
	GetContext() *Context
	GetRouter() *Router

	// 工具方法
	RandomLowerAndNum(length int) string
	CheckAdmin(tgId int64) bool
}

// TgBot Telegram Bot 的具体实现
type TgBot struct {
	// 核心组件
	ctx    *Context
	router *Router

	// Bot 实例
	bot        *telego.Bot
	botHandler *th.BotHandler

	// 配置
	adminIds       []int64
	isRunning      bool
	hostname       string
	hashStorage    *global.HashStorage
	tgBotToken     string
	tgBotID        string
	tgBotProxy     string
	tgBotAPIServer string
	settingService interface{}
}

// NewBot 创建新的 Bot 实例
func NewBot() Bot {
	ctx := NewContext()
	router := NewRouter(ctx)

	bot := &TgBot{
		ctx:         ctx,
		router:      router,
		hashStorage: global.NewHashStorage(20 * time.Minute),
	}

	// 设置路由器的上下文
	router.SetContext(ctx)

	return bot
}

// Start 启动 Bot
func (b *TgBot) Start(i18nFS embed.FS, settingService interface{}) error {
	if b.isRunning {
		return errors.New("bot is already running")
	}

	b.settingService = settingService

	// 初始化本地化器
	// 注意：i18nFS 和 settingService 需要在实际使用时正确传入
	// 暂时跳过本地化初始化，简化集成
	logger.Info("Bot starting without i18n initialization (simplified)")

	// 初始化哈希存储
	b.hashStorage = global.NewHashStorage(20 * time.Minute)

	// 设置主机名
	b.SetHostname()

	// 获取配置
	if err := b.loadConfiguration(); err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// 创建 Bot 实例
	if err := b.createBotInstance(); err != nil {
		return fmt.Errorf("failed to create bot instance: %w", err)
	}

	// 验证 Bot 连通性
	if err := b.verifyBotConnectivity(); err != nil {
		return fmt.Errorf("failed to verify bot connectivity: %w", err)
	}

	// 设置 Bot 命令
	if err := b.setupBotCommands(); err != nil {
		logger.Warning("Failed to set bot commands:", err)
	}

	// 启动消息接收
	b.startMessageReceiving()

	return nil
}

// Stop 停止 Bot
func (b *TgBot) Stop() {
	if b.botHandler != nil {
		b.botHandler.Stop()
	}
	logger.Info("Stopping Telegram receiver...")
	b.isRunning = false
	b.adminIds = nil
}

// Restart 重启 Bot
func (b *TgBot) Restart() error {
	b.Stop()
	time.Sleep(2 * time.Second)
	return b.Start(embed.FS{}, b.settingService)
}

// IsRunning 检查 Bot 是否正在运行
func (b *TgBot) IsRunning() bool {
	return b.isRunning
}

// SendMessage 发送消息
func (b *TgBot) SendMessage(chatId int64, message string, replyMarkup ...telego.ReplyMarkup) error {
	return b.ctx.SendMsgToTgbot(chatId, message, replyMarkup...)
}

// SendMessageToAdmins 发送消息给所有管理员
func (b *TgBot) SendMessageToAdmins(message string, replyMarkup ...telego.ReplyMarkup) error {
	return b.ctx.SendMsgToTgbotAdmins(message, replyMarkup...)
}

// GetBot 获取 telego Bot 实例
func (b *TgBot) GetBot() *telego.Bot {
	return b.bot
}

// GetContext 获取 Context 实例
func (b *TgBot) GetContext() *Context {
	return b.ctx
}

// GetRouter 获取 Router 实例
func (b *TgBot) GetRouter() *Router {
	return b.router
}

// RandomLowerAndNum 生成随机小写字母和数字
func (b *TgBot) RandomLowerAndNum(length int) string {
	charset := "abcdefghijklmnopqrstuvwxyz0123456789"
	bytes := make([]byte, length)
	for i := range bytes {
		randomIndex, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		bytes[i] = charset[randomIndex.Int64()]
	}
	return string(bytes)
}

// CheckAdmin 检查用户是否为管理员
func (b *TgBot) CheckAdmin(tgId int64) bool {
	return b.ctx.CheckAdmin(tgId)
}

// loadConfiguration 加载配置
func (b *TgBot) loadConfiguration() error {
	// 获取 Telegram Bot Token
	var err error
	b.tgBotToken, err = b.getSettingString("GetTgBotToken")
	if err != nil || b.tgBotToken == "" {
		return fmt.Errorf("Telegram bot token is missing or invalid: %v", err)
	}

	// 验证 Bot Token 格式
	if len(b.tgBotToken) < 10 || !strings.Contains(b.tgBotToken, ":") {
		return fmt.Errorf("invalid Telegram bot token format. Token should be in format '123456789:ABCdefGHIjklMNOpqrsTUVwxyz'")
	}

	// 获取 Telegram Bot Chat ID
	b.tgBotID, err = b.getSettingString("GetTgBotChatId")
	if err != nil {
		return fmt.Errorf("failed to get Telegram bot chat ID: %v", err)
	}

	// 解析管理员 ID
	if err := b.parseAdminIds(); err != nil {
		return fmt.Errorf("failed to parse admin IDs: %w", err)
	}

	// 获取代理和 API 服务器配置
	b.tgBotProxy, _ = b.getSettingString("GetTgBotProxy")
	b.tgBotAPIServer, _ = b.getSettingString("GetTgBotAPIServer")

	return nil
}

// getSettingString 获取设置字符串值
func (b *TgBot) getSettingString(methodName string) (string, error) {
	// 这里使用反射来调用方法，简化依赖
	// 在实际使用中，可以通过类型断言来获得具体的服务实例
	if b.settingService == nil {
		return "", errors.New("setting service not initialized")
	}

	// TODO: 在实际集成时需要实现具体逻辑
	return "", errors.New("configuration loading not implemented - this is a stub for the refactoring")
}

// createBotInstance 创建 Bot 实例
func (b *TgBot) createBotInstance() error {
	var err error
	b.bot, err = b.createBot(b.tgBotToken, b.tgBotProxy, b.tgBotAPIServer)
	if err != nil {
		return fmt.Errorf("failed to create bot instance: %w", err)
	}

	// 设置 Bot 到 Context
	b.ctx.SetBot(b.bot)
	b.ctx.SetAdminIds(b.adminIds)
	b.ctx.SetRunning(true)

	return nil
}

// verifyBotConnectivity 验证 Bot 连通性
func (b *TgBot) verifyBotConnectivity() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	botInfo, err := b.bot.GetMe(ctx)
	if err != nil {
		return fmt.Errorf("failed to verify bot token with Telegram API: %w", err)
	}

	logger.Infof("Successfully connected to Telegram bot: @%s (ID: %d)", botInfo.Username, botInfo.ID)
	return nil
}

// setupBotCommands 设置 Bot 命令
func (b *TgBot) setupBotCommands() error {
	commands := []telego.BotCommand{
		{Command: "start", Description: b.ctx.I18n("tgbot.commands.startDesc")},
		{Command: "help", Description: b.ctx.I18n("tgbot.commands.helpDesc")},
		{Command: "status", Description: b.ctx.I18n("tgbot.commands.statusDesc")},
		{Command: "id", Description: b.ctx.I18n("tgbot.commands.idDesc")},
		{Command: "oneclick", Description: "一键配置节点"},
		{Command: "subconverter", Description: "检测或安装订阅转换"},
		{Command: "restartx", Description: "重启X-Panel面板"},
		{Command: "xrayversion", Description: "管理Xray版本"},
	}

	return b.bot.SetMyCommands(context.Background(), &telego.SetMyCommandsParams{
		Commands: commands,
	})
}

// startMessageReceiving 开始接收消息
func (b *TgBot) startMessageReceiving() {
	if !b.isRunning {
		logger.Info("Telegram bot receiver started")
		go b.onReceive()
		b.isRunning = true
	}
}

// onReceive 消息接收处理
func (b *TgBot) onReceive() {
	params := telego.GetUpdatesParams{
		Timeout: 10,
	}

	updates, _ := b.bot.UpdatesViaLongPolling(context.Background(), &params)

	b.botHandler, _ = th.NewBotHandler(b.bot, updates)

	// 设置处理器
	b.router.SetupHandlers(b.botHandler)

	b.botHandler.Start()
}

// createBot 创建 Bot 实例
func (b *TgBot) createBot(token string, proxyUrl string, apiServerUrl string) (*telego.Bot, error) {
	if proxyUrl == "" && apiServerUrl == "" {
		return telego.NewBot(token)
	}

	if proxyUrl != "" {
		if !strings.HasPrefix(proxyUrl, "socks5://") {
			logger.Warning("Invalid socks5 URL, using default")
			return telego.NewBot(token)
		}

		_, err := url.Parse(proxyUrl)
		if err != nil {
			logger.Warningf("Can't parse proxy URL, using default instance for tgbot: %v", err)
			return telego.NewBot(token)
		}

		return telego.NewBot(token, telego.WithFastHTTPClient(&fasthttp.Client{
			Dial: fasthttpproxy.FasthttpSocksDialer(proxyUrl),
		}))
	}

	if !strings.HasPrefix(apiServerUrl, "http") {
		logger.Warning("Invalid http(s) URL, using default")
		return telego.NewBot(token)
	}

	_, err := url.Parse(apiServerUrl)
	if err != nil {
		logger.Warningf("Can't parse API server URL, using default instance for tgbot: %v", err)
		return telego.NewBot(token)
	}

	return telego.NewBot(token, telego.WithAPIServer(apiServerUrl))
}

// parseAdminIds 解析管理员 ID
func (b *TgBot) parseAdminIds() error {
	if b.tgBotID == "" {
		return errors.New("Telegram bot chat ID is not configured")
	}

	trimmedID := strings.TrimSpace(b.tgBotID)
	if trimmedID == "" {
		return errors.New("Telegram bot chat ID cannot be empty")
	}

	for _, adminID := range strings.Split(trimmedID, ",") {
		cleanedID := strings.TrimSpace(adminID)
		if cleanedID == "" {
			continue
		}

		id, err := strconv.Atoi(cleanedID)
		if err != nil {
			return fmt.Errorf("invalid admin ID format '%s': %v", cleanedID, err)
		}

		if id <= 0 {
			return fmt.Errorf("invalid admin ID '%d': Chat ID must be a positive number", id)
		}

		b.adminIds = append(b.adminIds, int64(id))
		logger.Infof("Added admin ID: %d", id)
	}

	if len(b.adminIds) == 0 {
		return errors.New("no valid admin IDs found in chat ID configuration")
	}

	return nil
}

// SetHostname 设置主机名
func (b *TgBot) SetHostname() {
	host, err := os.Hostname()
	if err != nil {
		logger.Error("get hostname error:", err)
		b.hostname = ""
		return
	}
	b.hostname = host
	b.ctx.SetHostname()
}
