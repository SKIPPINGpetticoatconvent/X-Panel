package bootstrap

import (
	"log"

	"x-ui/config"
	"x-ui/database"
	"x-ui/database/repository"
	"x-ui/logger"
	"x-ui/web/service"
	"x-ui/xray"

	"github.com/joho/godotenv"
)

// App 封装应用运行时所需的所有服务实例
type App struct {
	SettingService  *service.SettingService
	ServerService   *service.ServerService
	XrayService     *service.XrayService
	InboundService  *service.InboundService
	OutboundService *service.OutboundService
	UserService     *service.UserService
	LastStatus      *service.Status
	XrayAPI         *xray.XrayAPI
	TgBot           *service.Tgbot

	// Repositories
	InboundRepo  repository.InboundRepository
	OutboundRepo repository.OutboundRepository
	SettingRepo  repository.SettingRepository
	UserRepo     repository.UserRepository
}

// NewApp 创建并初始化应用实例
func NewApp(
	settingService *service.SettingService,
	userService *service.UserService,
	outboundService *service.OutboundService,
	inboundService *service.InboundService,
	xrayService *service.XrayService,
	serverService *service.ServerService,
	tgBotService *service.Tgbot,
	status *service.Status,
	xrayAPI *xray.XrayAPI,
	inboundRepo repository.InboundRepository,
	outboundRepo repository.OutboundRepository,
	settingRepo repository.SettingRepository,
	userRepo repository.UserRepository,
) *App {
	// 补丁：手动解决循环依赖 (Wire 无法自动处理的闭环)
	inboundService.SetXrayService(xrayService)
	inboundService.SetTelegramService(tgBotService)
	xrayService.SetInboundService(inboundService)
	serverService.SetInboundService(inboundService)
	serverService.SetXrayService(xrayService)
	serverService.SetTelegramService(tgBotService)
	tgBotService.SetInboundService(inboundService)
	tgBotService.SetServerService(serverService)

	return &App{
		SettingService:  settingService,
		ServerService:   serverService,
		XrayService:     xrayService,
		InboundService:  inboundService,
		OutboundService: outboundService,
		UserService:     userService,
		LastStatus:      status,
		XrayAPI:         xrayAPI,
		TgBot:           tgBotService,

		InboundRepo:  inboundRepo,
		OutboundRepo: outboundRepo,
		SettingRepo:  settingRepo,
		UserRepo:     userRepo,
	}
}

// InitDatabase 初始化数据库连接
func InitDatabase() error {
	return database.InitDB(config.GetDBPath())
}

// InitLogger 根据配置初始化日志系统
func InitLogger(settingService *service.SettingService) {
	localLogEnabled, err := settingService.GetLocalLogEnabled()
	if err != nil {
		logger.Warningf("无法获取本地日志配置，使用默认设置: %v", err)
		localLogEnabled = false
	}

	var level logger.Level
	switch config.GetLogLevel() {
	case config.Debug:
		level = logger.DEBUG
	case config.Info:
		level = logger.INFO
	case config.Notice:
		level = logger.NOTICE
	case config.Warning:
		level = logger.WARNING
	case config.Error:
		level = logger.ERROR
	default:
		log.Fatalf("Unknown log level: %v", config.GetLogLevel())
	}

	logger.InitLogger(level, localLogEnabled)
}

// LoadEnv 加载环境变量
func LoadEnv() {
	_ = godotenv.Load()
}

// Initialize 执行完整的应用初始化流程
func Initialize() (*App, error) {
	log.Printf("Starting %v %v", config.GetName(), config.GetVersion())

	LoadEnv()

	if err := InitDatabase(); err != nil {
		return nil, err
	}

	app, err := InitializeApp()
	if err != nil {
		return nil, err
	}
	InitLogger(app.SettingService)

	return app, nil
}
