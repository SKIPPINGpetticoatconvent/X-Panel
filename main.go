package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"syscall"
	_ "unsafe"

	"x-ui/bootstrap"
	"x-ui/config"
	"x-ui/logger"
)

// runWebServer 是主执行函数，使用 bootstrap 模块简化启动流程
func runWebServer() {
	// 初始化应用
	app, err := bootstrap.Initialize()
	if err != nil {
		log.Fatalf("Error initializing application: %v", err)
	}

	// 创建运行时
	runtime := bootstrap.NewRuntime(app)

	// 初始化 Telegram Bot
	if err := runtime.InitTelegramBot(); err != nil {
		logger.Warningf("无法获取 Telegram Bot 设置: %v", err)
	}

	// 执行依赖注入
	app.WireServices(runtime.GetTelegramService())

	// 启动 Web 服务器
	if err := runtime.StartWebServer(); err != nil {
		log.Fatalf("Error starting web server: %v", err)
	}

	// 启动日志转发器
	runtime.StartLogForwarder()

	// 启动订阅服务器
	if err := runtime.StartSubServer(); err != nil {
		log.Fatalf("Error starting sub server: %v", err)
	}

	// 注册并启动后台任务
	runtime.StartJobs()

	// 信号处理循环
	sigCh := make(chan os.Signal, 1)
	setupSignalHandler(sigCh)

	for {
		sig := <-sigCh

		// 处理自定义信号 (如 SIGUSR2)
		if handleCustomSignal(sig, runtime.MonitorJob) {
			continue
		}

		switch sig {
		case syscall.SIGHUP:
			logger.Info("Received SIGHUP signal. Restarting servers...")
			if err := runtime.Restart(); err != nil {
				log.Fatalf("Error restarting: %v", err)
			}

		default:
			runtime.StopAll()
			log.Println("Shutting down servers.")
			return
		}
	}
}

func main() {
	if len(os.Args) < 2 {
		runWebServer()
		return
	}

	var showVersion bool
	flag.BoolVar(&showVersion, "v", false, "show version")

	runCmd := flag.NewFlagSet("run", flag.ExitOnError)

	settingCmd := flag.NewFlagSet("setting", flag.ExitOnError)
	var port int
	var username string
	var password string
	var webBasePath string
	var listenIP string
	var getListen bool
	var webCertFile string
	var webKeyFile string
	var tgbottoken string
	var tgbotchatid string
	var enabletgbot bool
	var tgbotRuntime string
	var reset bool
	var show bool
	var getCert bool
	var resetTwoFactor bool
	settingCmd.BoolVar(&reset, "reset", false, "Reset all settings")
	settingCmd.BoolVar(&show, "show", false, "Display current settings")
	settingCmd.IntVar(&port, "port", 0, "Set panel port number")
	settingCmd.StringVar(&username, "username", "", "Set login username")
	settingCmd.StringVar(&password, "password", "", "Set login password")
	settingCmd.StringVar(&webBasePath, "webBasePath", "", "Set base path for Panel")
	settingCmd.StringVar(&listenIP, "listenIP", "", "set panel listenIP IP")
	settingCmd.BoolVar(&resetTwoFactor, "resetTwoFactor", false, "Reset two-factor authentication settings")
	settingCmd.BoolVar(&getListen, "getListen", false, "Display current panel listenIP IP")
	settingCmd.BoolVar(&getCert, "getCert", false, "Display current certificate settings")
	settingCmd.StringVar(&webCertFile, "webCert", "", "Set path to public key file for panel")
	settingCmd.StringVar(&webKeyFile, "webCertKey", "", "Set path to private key file for panel")
	settingCmd.StringVar(&tgbottoken, "tgbottoken", "", "Set token for Telegram bot")
	settingCmd.StringVar(&tgbotRuntime, "tgbotRuntime", "", "Set cron time for Telegram bot notifications")
	settingCmd.StringVar(&tgbotchatid, "tgbotchatid", "", "Set chat ID for Telegram bot notifications")
	settingCmd.BoolVar(&enabletgbot, "enabletgbot", false, "Enable notifications via Telegram bot")

	oldUsage := flag.Usage
	flag.Usage = func() {
		oldUsage()
		fmt.Println()
		fmt.Println("Commands:")
		fmt.Println("    run            run web panel")
		fmt.Println("    migrate        migrate form other/old x-ui")
		fmt.Println("    setting        set settings")
	}

	flag.Parse()
	if showVersion {
		fmt.Println(config.GetVersion())
		return
	}

	switch os.Args[1] {
	case "run":
		err := runCmd.Parse(os.Args[2:])
		if err != nil {
			fmt.Println(err)
			return
		}
		runWebServer()
	case "migrate":
		migrateDb()
	case "setting":
		err := settingCmd.Parse(os.Args[2:])
		if err != nil {
			fmt.Println(err)
			return
		}
		if reset {
			resetSetting()
		} else {
			updateSetting(port, username, password, webBasePath, listenIP, resetTwoFactor)
		}
		if show {
			showSetting(show)
		}
		if getListen {
			getListenIP(getListen)
		}
		if getCert {
			getCertificate(getCert)
		}
		if (tgbottoken != "") || (tgbotchatid != "") || (tgbotRuntime != "") {
			updateTgbotSetting(tgbottoken, tgbotchatid, tgbotRuntime)
		}
		if enabletgbot {
			updateTgbotEnableSts(enabletgbot)
		}
	case "cert":
		err := settingCmd.Parse(os.Args[2:])
		if err != nil {
			fmt.Println(err)
			return
		}
		if reset {
			updateCert("", "")
		} else {
			updateCert(webCertFile, webKeyFile)
		}
	default:
		fmt.Println("Invalid subcommands ----->>无效命令")
		fmt.Println()
		runCmd.Usage()
		fmt.Println()
		settingCmd.Usage()
	}
}
