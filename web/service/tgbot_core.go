package service

import (
	"context"
	"embed"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"x-ui/database/model"
	"x-ui/logger"
	"x-ui/web/global"
	"x-ui/web/locale"

	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttpproxy"
)

// 新增 TelegramService 接口，用于解耦 Job 和 Telegram Bot 的直接依赖。
// 任何实现了 SendMessage(msg string) error 方法的结构体，都可以被认为是 TelegramService。
type TelegramService interface {
	SendMessage(msg string) error
	IsRunning() bool
	// 您可以根据 server.go 的需要，在这里继续扩展接口
	// 将 SendOneClickConfig 方法添加到接口中，这样其他服务可以通过接口来调用它，
	// 实现了与具体实现 Tgbot 的解耦。
	SendOneClickConfig(inbound *model.Inbound, inFromPanel bool, chatId int64) error
	// 新增 GetDomain 方法签名，以满足 server.go 的调用需求
	GetDomain() (string, error)
}

// 全局状态实例（向后兼容，逐步迁移到 Tgbot.state）
var (
	bot         *telego.Bot
	botHandler  *th.BotHandler
	adminIds    []int64
	isRunning   bool
	hostname    string
	hashStorage *global.HashStorage

	// clients data to adding new client
	receiver_inbound_ID int
	client_Id           string
	client_Flow         string
	client_Email        string
	client_LimitIP      int
	client_TotalGB      int64
	client_ExpiryTime   int64
	client_Enable       bool
	client_TgID         string
	client_SubID        string
	client_Comment      string
	client_Reset        int
	client_Security     string
	client_ShPassword   string
	client_TrPassword   string
	client_Method       string
)

var (
	userStates   = make(map[int64]string)
	userStatesMu sync.RWMutex
)

type LoginStatus byte

const (
	LoginSuccess        LoginStatus = 1
	LoginFail           LoginStatus = 0
	EmptyTelegramUserID             = int64(0)
)

type Tgbot struct {
	inboundService *InboundService
	settingService *SettingService
	serverService  *ServerService
	xrayService    *XrayService
	lastStatus     *Status

	// state 封装了 Bot 的运行时状态（新架构）
	// 目前保持向后兼容，逐步将全局变量迁移到此处
	state *BotState

	// 缓存解析后的域名/IP
	cachedDomain    string
	domainCacheTime time.Time
}

// GetRealityDestinations 方法 - 提供智能的 SNI 域名列表
func (t *Tgbot) GetRealityDestinations() []string {
	// 检查服务器地理位置，并使用对应的SNI域名列表
	if t.serverService != nil {
		country, err := t.serverService.GetServerLocation()
		if err == nil && country != "Unknown" {
			// 获取对应国家的SNI域名列表（包含去重机制）
			countryDomains := t.serverService.GetCountrySNIDomains(country)
			if len(countryDomains) > 0 {
				logger.Infof("检测到服务器IP位于%s，使用%s-SNI域名列表（%d个域名，已去重）", country, country, len(countryDomains))
				return countryDomains
			}
		}
		logger.Infof("服务器地理位置: %s，使用默认Reality域名列表", country)
	}

	// 默认 Reality 域名列表（国际通用）
	return []string{
		"tesla.com:443",
		"sega.com:443",
		"apple.com:443",
		"icloud.com:443",
		"lovelive-anime.jp:443",
		"meta.com:443",
	}
}

// 用于从外部注入 ServerService 实例
func (t *Tgbot) SetServerService(s *ServerService) {
	t.serverService = s
}

// 配合目前 main.go 代码结构实践。
func (t *Tgbot) SetInboundService(s *InboundService) {
	t.inboundService = s
}

// 在这里添加新的构造函数
// NewTgBot 创建并返回一个完全初始化的 Tgbot 实例。
// 这个函数确保了所有服务依赖项都被正确注入，避免了空指针问题。
func NewTgBot(
	inboundService *InboundService,
	settingService *SettingService,
	serverService *ServerService,
	xrayService *XrayService,
	lastStatus *Status,
) *Tgbot {
	t := &Tgbot{
		inboundService: inboundService,
		settingService: settingService,
		serverService:  serverService,
		xrayService:    xrayService,
		lastStatus:     lastStatus,
		state:          NewBotState(),
	}

	return t
}

// GetState 返回 Bot 的状态实例
func (t *Tgbot) GetState() *BotState {
	return t.state
}

/*
func (t *Tgbot) NewTgbot() *Tgbot {
	return new(Tgbot)
}
*/

func (t *Tgbot) I18nBot(name string, params ...string) string {
	return locale.I18n(locale.Bot, name, params...)
}

func (t *Tgbot) GetHashStorage() *global.HashStorage {
	return hashStorage
}

func (t *Tgbot) Start(i18nFS embed.FS) error {
	// Initialize localizer
	err := locale.InitLocalizer(i18nFS, t.settingService)
	if err != nil {
		return err
	}

	// Initialize hash storage to store callback queries
	hashStorage = global.NewHashStorage(20 * time.Minute)

	t.SetHostname()

	// Get Telegram bot token
	tgBotToken, err := t.settingService.GetTgBotToken()
	if err != nil || tgBotToken == "" {
		logger.Warning("Failed to get Telegram bot token:", err)
		return fmt.Errorf("telegram bot token is missing or invalid: %v", err)
	}

	// Validate bot token format
	if len(tgBotToken) < 10 || !strings.Contains(tgBotToken, ":") {
		logger.Warning("Invalid Telegram bot token format:", tgBotToken)
		return fmt.Errorf("invalid Telegram bot token format. Token should be in format '123456789:ABCdefGHIjklMNOpqrsTUVwxyz'")
	}

	// Get Telegram bot chat ID(s)
	tgBotID, err := t.settingService.GetTgBotChatId()
	if err != nil {
		logger.Warning("Failed to get Telegram bot chat ID:", err)
		return fmt.Errorf("failed to get Telegram bot chat ID: %v", err)
	}

	// Parse admin IDs from comma-separated string with enhanced validation
	if tgBotID != "" {
		trimmedID := strings.TrimSpace(tgBotID)
		if trimmedID == "" {
			logger.Warning("Telegram bot chat ID is empty after trimming")
			return fmt.Errorf("telegram bot chat ID cannot be empty")
		}

		// Reset adminIds to avoid duplicates when Start is called multiple times
		adminIds = []int64{}

		for _, adminID := range strings.Split(trimmedID, ",") {
			cleanedID := strings.TrimSpace(adminID)
			if cleanedID == "" {
				logger.Warning("Empty admin ID found in chat ID list, skipping")
				continue
			}

			id, err := strconv.Atoi(cleanedID)
			if err != nil {
				logger.Warningf("Failed to parse admin ID '%s' from Telegram bot chat ID: %v", cleanedID, err)
				return fmt.Errorf("invalid admin ID format '%s': %v. Chat IDs should be numeric (e.g., '123456789')", cleanedID, err)
			}

			if id <= 0 {
				logger.Warningf("Invalid admin ID '%d': Chat ID must be positive", id)
				return fmt.Errorf("invalid admin ID '%d': Chat ID must be a positive number", id)
			}

			adminIds = append(adminIds, int64(id))
			logger.Infof("Added admin ID: %d", id)
		}

		if len(adminIds) == 0 {
			logger.Warningf("No valid admin IDs were parsed from chat ID string: %s", tgBotID)
			return fmt.Errorf("no valid admin IDs found in chat ID configuration")
		}
	} else {
		logger.Warning("Telegram bot chat ID is not configured")
		return fmt.Errorf("telegram bot chat ID must be configured")
	}

	// Get Telegram bot proxy URL
	tgBotProxy, err := t.settingService.GetTgBotProxy()
	if err != nil {
		logger.Warning("Failed to get Telegram bot proxy URL:", err)
	}

	// Get Telegram bot API server URL
	tgBotAPIServer, err := t.settingService.GetTgBotAPIServer()
	if err != nil {
		logger.Warning("Failed to get Telegram bot API server URL:", err)
	}

	// Create new Telegram bot instance with enhanced validation
	bot, err = t.NewBot(tgBotToken, tgBotProxy, tgBotAPIServer)
	if err != nil {
		logger.Error("Failed to initialize Telegram bot API:", err)
		return fmt.Errorf("failed to initialize Telegram bot: %v. Please check your bot token and network settings", err)
	}

	// Test bot connectivity by getting bot info
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	botInfo, err := bot.GetMe(ctx)
	if err != nil {
		logger.Error("Failed to get bot information:", err)
		return fmt.Errorf("failed to verify bot token with Telegram API: %v. Please ensure the token is valid and has not been revoked", err)
	}

	logger.Infof("Successfully connected to Telegram bot: @%s (ID: %d)", botInfo.Username, botInfo.ID)

	// After bot initialization, set up bot commands with localized descriptions
	err = bot.SetMyCommands(context.Background(), &telego.SetMyCommandsParams{
		Commands: []telego.BotCommand{
			{Command: "start", Description: t.I18nBot("tgbot.commands.startDesc")},
			{Command: "help", Description: t.I18nBot("tgbot.commands.helpDesc")},
			{Command: "status", Description: t.I18nBot("tgbot.commands.statusDesc")},
			{Command: "id", Description: t.I18nBot("tgbot.commands.idDesc")},
			{Command: "oneclick", Description: "一键配置节点"},

			{Command: "restartx", Description: "重启X-Panel面板"},
			{Command: "xrayversion", Description: "管理Xray版本"},
		},
	})
	if err != nil {
		logger.Warning("Failed to set bot commands:", err)
	}

	// Start receiving Telegram bot messages
	if !isRunning {
		logger.Info("Telegram bot receiver started")
		go t.OnReceive()
		isRunning = true
	}

	// 确保 LogForwarder 被激活
	if t.settingService != nil {
		logForwarder := NewLogForwarder(t.settingService, t)
		go func() { _ = logForwarder.Start() }()
	}

	return nil
}

func (t *Tgbot) NewBot(token string, proxyUrl string, apiServerUrl string) (*telego.Bot, error) {
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

func (t *Tgbot) IsRunning() bool {
	return isRunning
}

func deleteUserState(userId int64) {
	userStatesMu.Lock()
	defer userStatesMu.Unlock()
	delete(userStates, userId)
}
