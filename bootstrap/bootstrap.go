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
	"github.com/op/go-logging"
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
}

// NewApp 创建并初始化应用实例
func NewApp() *App {
	// 创建 Repository 实例
	settingRepo := repository.NewSettingRepository()
	userRepo := repository.NewUserRepository()
	inboundRepo := repository.NewInboundRepository()
	clientTrafficRepo := repository.NewClientTrafficRepository()
	clientIPRepo := repository.NewClientIPRepository()
	outboundRepo := repository.NewOutboundRepository()

	// 使用构造函数创建 Service，注入 Repository 依赖
	settingService := service.NewSettingService(settingRepo)
	userService := service.NewUserService(userRepo, settingService)
	inboundService := service.NewInboundService(inboundRepo, clientTrafficRepo, clientIPRepo)
	outboundService := service.NewOutboundService(outboundRepo)

	return &App{
		SettingService:  settingService,
		ServerService:   &service.ServerService{},
		XrayService:     &service.XrayService{},
		InboundService:  inboundService,
		OutboundService: outboundService,
		UserService:     userService,
		LastStatus:      &service.Status{},
		XrayAPI:         &xray.XrayAPI{},
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

	var level logging.Level
	switch config.GetLogLevel() {
	case config.Debug:
		level = logging.DEBUG
	case config.Info:
		level = logging.INFO
	case config.Notice:
		level = logging.NOTICE
	case config.Warning:
		level = logging.WARNING
	case config.Error:
		level = logging.ERROR
	default:
		log.Fatalf("Unknown log level: %v", config.GetLogLevel())
	}

	logger.InitLogger(level, localLogEnabled)
}

// LoadEnv 加载环境变量
func LoadEnv() {
	_ = godotenv.Load()
}

// WireServices 执行服务间的依赖注入
func (app *App) WireServices(tgBotService service.TelegramService) {
	// 注入 XrayAPI
	app.XrayService.SetXrayAPI(*app.XrayAPI)
	app.InboundService.SetXrayAPI(*app.XrayAPI)

	// 注入服务间依赖
	app.ServerService.SetXrayService(app.XrayService)
	app.ServerService.SetInboundService(app.InboundService)
	app.XrayService.SetInboundService(app.InboundService)
	app.InboundService.SetXrayService(app.XrayService)

	// 注入 Telegram 服务
	app.ServerService.SetTelegramService(tgBotService)
	app.InboundService.SetTelegramService(tgBotService)
}

// Initialize 执行完整的应用初始化流程
func Initialize() (*App, error) {
	log.Printf("Starting %v %v", config.GetName(), config.GetVersion())

	LoadEnv()

	if err := InitDatabase(); err != nil {
		return nil, err
	}

	app := NewApp()
	InitLogger(app.SettingService)

	return app, nil
}
