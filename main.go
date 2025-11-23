package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
//	"os/exec"
//	"strings"
	"syscall"
	_ "unsafe"
	// 中文注释: 新增了 time 和 x-ui/job 的导入，这是运行定时任务所必需的包
	"time"

	"x-ui/web/job"
	"x-ui/config"
	"x-ui/database"
	"x-ui/logger"
	"x-ui/sub"
	"x-ui/util/crypto"
	"x-ui/web"
	"x-ui/web/global"
	"x-ui/web/service"
	"x-ui/xray"

	"github.com/joho/godotenv"
	"github.com/op/go-logging"
)

// runWebServer 是【设备限制】项目的主执行函数
func runWebServer() {
	log.Printf("DEBUG: Calling config.GetName() and config.GetVersion()")
	log.Printf("Starting %v %v", config.GetName(), config.GetVersion())

	log.Printf("DEBUG: Calling config.GetLogLevel()")
	switch config.GetLogLevel() {
	case config.Debug:
		logger.InitLogger(logging.DEBUG)
	case config.Info:
		logger.InitLogger(logging.INFO)
	case config.Notice:
		logger.InitLogger(logging.NOTICE)
	case config.Warn:
		logger.InitLogger(logging.WARNING)
	case config.Error:
		logger.InitLogger(logging.ERROR)
	default:
		logger.Warningf("Unknown log level: %v, using default Info level", config.GetLogLevel())
		logger.InitLogger(logging.INFO)
	}

	godotenv.Load()

	err := database.InitDB(config.GetDBPath())
	if err != nil {
		logger.Warningf("Error initializing database: %v, continuing with default settings", err)
	}

	// 〔中文注释〕: 1. 初始化所有需要的服务实例
	xrayService := &service.XrayService{}
	settingService := &service.SettingService{}
	serverService := &service.ServerService{}
	// 还需要 InboundService 等，按需添加
	inboundService := service.InboundService{}
	lastStatus := service.Status{}

	// 创建 Xray API 实例
	xrayApi := xray.XrayAPI{}
	
	// 注入到 XrayService 中 
	xrayService.SetXrayAPI(xrayApi) 
	
	// 注入到 InboundService 中 
	inboundService.SetXrayAPI(xrayApi)

	// 〔中文注释〕: 2. 初始化 TG Bot 服务 (如果已启用)
	tgEnable, err := settingService.GetTgbotEnabled()
	if err != nil {
		logger.Warningf("无法获取 Telegram Bot 设置: %v", err)
	}

	var tgBotService service.TelegramService 
	if tgEnable {
		// 将所有需要的服务作为参数传递进去，确保返回的 tgBotService 是一个完全初始化的、可用的实例。
		tgBot := service.NewTgBot(&inboundService, settingService, serverService, xrayService, &lastStatus)
		tgBotService = tgBot
	}

	// 〔中文注释〕: 3. 【核心步骤】执行依赖注入
	//    将 tgBotService 实例注入到 serverService 中。
	//    这样 serverService 内部的 tgService 字段就不再是 nil 了。
	serverService.SetTelegramService(tgBotService)
	//    同理，也为 InboundService 注入
	inboundService.SetTelegramService(tgBotService)
	
	var server *web.Server
	
	// 〔中文注释〕: 调用我们刚刚改造过的 web.NewServer，把功能完整的 serverService 传进去。
	server = web.NewServer(*serverService)
	   // 将 tgBotService 注入到 web.Server 中，使其在 web.go/Server.Start() 中可用
	   if tgBotService != nil {
		// 〔中文注释〕: 这里的注入是为了让 Web Server 可以在启动时调用 Tgbot.Start()
	       // 同时，也确保了 Web 层的回调处理能使用到这个完整的 Bot 实例
	       server.SetTelegramService(tgBotService)
	   }
	   // 将 xrayService 注入到 web.Server 中，修复 nil 指针问题
	   server.SetXrayService(xrayService)
	
	global.SetWebServer(server)
	
	// 【新增】: 启动前检查并自动配置证书路径
	autoConfigureCertPath(settingService)
	
	err = server.Start()
	if err != nil {
		log.Fatalf("Error starting web server: %v", err)
		return
	}

	var subServer *sub.Server
	subServer = sub.NewServer()
	global.SetSubServer(subServer)
	err = subServer.Start()
	if err != nil {
		log.Fatalf("Error starting sub server: %v", err)
		return
	}

	// 中文注释: 在面板服务启动后，我们在这里启动设备限制的后台任务
	go func() {
		// 中文注释: 等待5秒，确保面板和Xray服务已基本稳定，避免任务启动过早
		time.Sleep(10 * time.Second)

		// 中文注释: 创建一个定时器。这里的 "10 * time.Second" 就是任务执行的间隔时间。
		// 您可以修改 10 为 2 或 1，来实现更短的延迟。
		// 例如: time.NewTicker(2 * time.Second) 就是2秒执行一次。
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		// 〔中文注释〕: 步骤一：在循环外部，只声明一次 tgBotService 变量。
		// 我们将其声明为接口类型，初始值为 nil。
		var tgBotService service.TelegramService

		// 〔中文注释〕: 步骤二：检查 Telegram Bot 是否在面板设置中启用。
		settingService := service.SettingService{}
		tgEnable, err := settingService.GetTgbotEnabled()
		if err != nil {
			logger.Warningf("无法获取 Telegram Bot 设置: %v, 设备限制通知功能可能无法使用", err)
		}

		// 〔中文注释〕: 步骤三：如果 Bot 已启用，则初始化实例并赋值给上面声明的变量。
		// 注意这里使用的是 `=` 而不是 `:=`，因为我们是给已存在的变量赋值。
		if tgEnable {
			tgBotService = new(service.Tgbot)
		}
		
		// 〔中文注释〕：步骤四：创建任务实例时，将 xrayService 和 可能为 nil 的 tgBotService 一同传入。
		// 这样做是安全的，因为 check_client_ip_job.go 内部的 SendMessage 调用前，会先判断服务实例是否可用。
		statsJob := job.NewStatsNotifyJob(xrayService, tgBotService)

		// 启动定时任务
		// statsJob.Run()


		// 中文注释: 使用一个无限循环，每次定时器触发，就执行一次任务的 Run() 函数
		for {
			<-ticker.C
			statsJob.Run()
		}
	}()

	sigCh := make(chan os.Signal, 1)
	// Trap shutdown signals
	signal.Notify(sigCh, syscall.SIGHUP, syscall.SIGTERM)
	for {
		sig := <-sigCh

		switch sig {
		case syscall.SIGHUP:
			logger.Info("Received SIGHUP signal. Restarting servers...")

			err := server.Stop()
			if err != nil {
				logger.Debug("Error stopping web server:", err)
			}
			err = subServer.Stop()
			if err != nil {
				logger.Debug("Error stopping sub server:", err)
			}

			server = web.NewServer(*serverService)
			// 重新注入 tgBotService
			         if tgBotService != nil {
			              server.SetTelegramService(tgBotService)
			         }
			         // 重新注入 xrayService
			         server.SetXrayService(xrayService)
			global.SetWebServer(server)
			err = server.Start()
			if err != nil {
				log.Fatalf("Error restarting web server: %v", err)
				return
			}
			log.Println("Web server restarted successfully.")

			subServer = sub.NewServer()
			global.SetSubServer(subServer)
			err = subServer.Start()
			if err != nil {
				log.Fatalf("Error restarting sub server: %v", err)
				return
			}
			log.Println("Sub server restarted successfully.")

		default:
			server.Stop()
			subServer.Stop()
			log.Println("Shutting down servers.")
			return
		}
	}
}

func resetSetting() {
	err := database.InitDB(config.GetDBPath())
	if err != nil {
		fmt.Println("Failed to initialize database:", err)
		return
	}

	settingService := service.SettingService{}
	err = settingService.ResetSettings()
	if err != nil {
		fmt.Println("Failed to reset settings（重置设置失败）:", err)
	} else {
		fmt.Println("Settings successfully reset ---->>重置设置成功")
	}
}

func showSetting(show bool) {
	// 执行 shell 命令获取 IPv4 地址
   //   cmdIPv4 := exec.Command("sh", "-c", "curl -s4m8 ip.p3terx.com -k | sed -n 1p")
  //    outputIPv4, err := cmdIPv4.Output()
  //    if err != nil {
  //     log.Fatal(err)
  //  }

    // 执行 shell 命令获取 IPv6 地址
   //     cmdIPv6 := exec.Command("sh", "-c", "curl -s6m8 ip.p3terx.com -k | sed -n 1p")
   //     outputIPv6, err := cmdIPv6.Output()
   //     if err != nil {
   //     log.Fatal(err)
  //  }

    // 去除命令输出中的换行符
//    ipv4 := strings.TrimSpace(string(outputIPv4))
//    ipv6 := strings.TrimSpace(string(outputIPv6))
    // 定义转义字符，定义不同颜色的转义字符
	const (
		Reset      = "\033[0m"
		Red        = "\033[31m"
		Green      = "\033[32m"
		Yellow     = "\033[33m"
	)
	if show {
		settingService := service.SettingService{}
		port, err := settingService.GetPort()
		if err != nil {
			fmt.Println("get current port failed, error info（获取当前端口失败，错误信息）:", err)
		}

		webBasePath, err := settingService.GetBasePath()
		if err != nil {
			fmt.Println("get webBasePath failed, error info（获取访问路径失败，错误信息）:", err)
		}

		certFile, err := settingService.GetCertFile()
		if err != nil {
			fmt.Println("get cert file failed, error info:", err)
		}
		keyFile, err := settingService.GetKeyFile()
		if err != nil {
			fmt.Println("get key file failed, error info:", err)
		}

		userService := service.UserService{}
		userModel, err := userService.GetFirstUser()
		if err != nil {
			fmt.Println("get current user info failed, error info（获取当前用户信息失败，错误信息）:", err)
		}

		if userModel.Username == "" || userModel.Password == "" {
			fmt.Println("current username or password is empty --->>当前用户名或密码为空")
		}

		fmt.Println("")
                fmt.Println(Yellow + "----->>>以下为面板重要信息，请自行记录保存<<<-----" + Reset)
		fmt.Println(Green + "Current panel settings as follows (当前面板设置如下):" + Reset)
		fmt.Println("")
		if certFile == "" || keyFile == "" {
                                                   fmt.Println(Red + "------>> 警告：面板未安装证书进行SSL保护" + Reset)
		} else {
                                                   fmt.Println(Green + "------>> 面板已安装证书采用SSL保护" + Reset)
		}
                fmt.Println("")
		hasDefaultCredential := func() bool {
			return userModel.Username == "admin" && crypto.CheckPasswordHash(userModel.Password, "admin")
		}()
                if hasDefaultCredential == true {
                                                   fmt.Println(Red + "------>> 警告：使用了默认的admin账号/密码，容易被扫描" + Reset)
		} else {
                                                   fmt.Println(Green + "------>> 为非默认admin账号/密码，请牢记" + Reset)
		}
		fmt.Println("")
		fmt.Println(Green + fmt.Sprintf("port（端口号）: %d", port) + Reset)
		fmt.Println(Green + fmt.Sprintf("webBasePath（访问路径）: %s", webBasePath) + Reset)
		fmt.Println(Green + "PS：为安全起见，不显示账号和密码" + Reset)
		fmt.Println(Green + "若您已经忘记账号/密码，请用脚本选项〔6〕重新设置" + Reset)

	                 fmt.Println("")
		fmt.Println("--------------------------------------------------")
  // 根据条件打印带颜色的字符串
 //     if ipv4 != "" {
 // 		fmt.Println("")
 // 		formattedIPv4 := fmt.Sprintf("%s %s%s:%d%s" + Reset,
 // 			Green+"面板 IPv4 访问地址------>>",
 // 		  	Yellow+"http://",
 // 			ipv4,
 // 			port,
 // 			Yellow+webBasePath + Reset)
 // 		fmt.Println(formattedIPv4)
 // 		fmt.Println("")
 // 	}

 // 	if ipv6 != "" {
 // 		fmt.Println("")
 // 		formattedIPv6 := fmt.Sprintf("%s %s[%s%s%s]:%d%s%s",
 // 	        	Green+"面板 IPv6 访问地址------>>", // 绿色的提示信息
 // 		        Yellow+"http://",                 // 黄色的 http:// 部分
 // 		        Yellow,                           // 黄色的[ 左方括号
 // 		        ipv6,                             // IPv6 地址
 // 		        Yellow,                           // 黄色的] 右方括号
 // 		        port,                             // 端口号
 // 	        	Yellow+webBasePath,               // 黄色的 Web 基础路径
 // 	         	Reset)                            // 重置颜色
 // 		fmt.Println(formattedIPv6)
 // 		fmt.Println("")
 // 	}
	fmt.Println(Green + ">>>>>>>>注：若您安装了〔证书〕，请使用您的域名用https方式登录" + Reset)
	fmt.Println("")
	fmt.Println("--------------------------------------------------")
	fmt.Println("")
//	fmt.Println("↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑")
	fmt.Println(fmt.Sprintf("%s请确保 %s%d%s 端口已打开放行%s",Green, Red, port, Green, Reset))	
	fmt.Println(Yellow + "请自行确保此端口没有被其他程序占用" + Reset)
//	fmt.Println(Green + "若要登录访问面板，请复制上面的地址到浏览器" + Reset)
	fmt.Println("")
	fmt.Println("--------------------------------------------------")
	fmt.Println("")
            }
}

func updateTgbotEnableSts(status bool) {
	settingService := service.SettingService{}
	currentTgSts, err := settingService.GetTgbotEnabled()
	if err != nil {
		fmt.Println(err)
		return
	}
	logger.Infof("current enabletgbot status[%v],need update to status[%v]", currentTgSts, status)
	if currentTgSts != status {
		err := settingService.SetTgbotEnabled(status)
		if err != nil {
			fmt.Println(err)
			return
		} else {
			logger.Infof("SetTgbotEnabled[%v] success", status)
		}
	}
}

func updateTgbotSetting(tgBotToken string, tgBotChatid string, tgBotRuntime string) {
	err := database.InitDB(config.GetDBPath())
	if err != nil {
		fmt.Println("Error initializing database（初始化数据库出错）:", err)
		return
	}

	settingService := service.SettingService{}

	if tgBotToken != "" {
		err := settingService.SetTgBotToken(tgBotToken)
		if err != nil {
			fmt.Printf("Error setting Telegram bot token（设置TG电报机器人令牌出错）: %v\n", err)
			return
		}
		logger.Info("Successfully updated Telegram bot token ----->>已成功更新TG电报机器人令牌")
	}

	if tgBotRuntime != "" {
		err := settingService.SetTgbotRuntime(tgBotRuntime)
		if err != nil {
			fmt.Printf("Error setting Telegram bot runtime（设置TG电报机器人通知周期出错）: %v\n", err)
			return
		}
		logger.Infof("Successfully updated Telegram bot runtime to （已成功将TG电报机器人通知周期设置为） [%s].", tgBotRuntime)
	}

	if tgBotChatid != "" {
		err := settingService.SetTgBotChatId(tgBotChatid)
		if err != nil {
			fmt.Printf("Error setting Telegram bot chat ID（设置TG电报机器人管理者聊天ID出错）: %v\n", err)
			return
		}
		logger.Info("Successfully updated Telegram bot chat ID ----->>已成功更新TG电报机器人管理者聊天ID")
	}
}

func updateSetting(port int, username string, password string, webBasePath string, listenIP string, resetTwoFactor bool) {
	err := database.InitDB(config.GetDBPath())
	if err != nil {
		fmt.Println("Database initialization failed（初始化数据库失败）:", err)
		return
	}

	settingService := service.SettingService{}
	userService := service.UserService{}

	if port > 0 {
		err := settingService.SetPort(port)
		if err != nil {
			fmt.Println("Failed to set port（设置端口失败）:", err)
		} else {
			fmt.Printf("Port set successfully（端口设置成功）: %v\n", port)
		}
	}

	if username != "" || password != "" {
		err := userService.UpdateFirstUser(username, password)
		if err != nil {
			fmt.Println("Failed to update username and password（更新用户名和密码失败）:", err)
		} else {
			fmt.Println("Username and password updated successfully ------>>用户名和密码更新成功")
		}
	}

	if webBasePath != "" {
		err := settingService.SetBasePath(webBasePath)
		if err != nil {
			fmt.Println("Failed to set base URI path（设置访问路径失败）:", err)
		} else {
			fmt.Println("Base URI path set successfully ------>>设置访问路径成功")
		}
	}

	if resetTwoFactor {
		err := settingService.SetTwoFactorEnable(false)

		if err != nil {
			fmt.Println("Failed to reset two-factor authentication（设置两步验证失败）:", err)
		} else {
			settingService.SetTwoFactorToken("")
			fmt.Println("Two-factor authentication reset successfully --------->>设置两步验证成功")
		}
	}

	if listenIP != "" {
		err := settingService.SetListen(listenIP)
		if err != nil {
			fmt.Println("Failed to set listen IP（设置监听IP失败）:", err)
		} else {
			fmt.Printf("listen %v set successfully --------->>设置监听IP成功", listenIP)
		}
	}
}

func updateCert(publicKey string, privateKey string) {
	err := database.InitDB(config.GetDBPath())
	if err != nil {
		fmt.Println(err)
		return
	}

	if (privateKey != "" && publicKey != "") || (privateKey == "" && publicKey == "") {
		settingService := service.SettingService{}
		err = settingService.SetCertFile(publicKey)
		if err != nil {
			fmt.Println("set certificate public key failed（设置证书公钥失败）:", err)
		} else {
			fmt.Println("set certificate public key success --------->>设置证书公钥成功")
		}

		err = settingService.SetKeyFile(privateKey)
		if err != nil {
			fmt.Println("set certificate private key failed（设置证书私钥失败）:", err)
		} else {
			fmt.Println("set certificate private key success --------->>设置证书私钥成功")
		}
	} else {
		fmt.Println("both public and private key should be entered ------>>必须同时输入证书公钥和私钥")
	}
}

func GetCertificate(getCert bool) {
	if getCert {
		settingService := service.SettingService{}
		certFile, err := settingService.GetCertFile()
		if err != nil {
			fmt.Println("get cert file failed, error info:", err)
		}
		keyFile, err := settingService.GetKeyFile()
		if err != nil {
			fmt.Println("get key file failed, error info:", err)
		}

		fmt.Println("cert:", certFile)
		fmt.Println("key:", keyFile)
	}
}

func GetListenIP(getListen bool) {
	if getListen {

		settingService := service.SettingService{}
		ListenIP, err := settingService.GetListen()
		if err != nil {
			log.Printf("Failed to retrieve listen IP: %v", err)
			return
		}

		fmt.Println("listenIP:", ListenIP)
	}
}

func migrateDb() {
	inboundService := service.InboundService{}

	err := database.InitDB(config.GetDBPath())
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Start migrating database... ---->>开始迁移数据库...")
	inboundService.MigrateDB()
	fmt.Println("Migration done! ------------>>迁移完成！")
}

// 【新增函数】: 自动检查并配置证书路径
func autoConfigureCertPath(settingService *service.SettingService) {
	// 检查是否已经配置了证书路径
	certFile, err := settingService.GetCertFile()
	if err != nil {
		logger.Warningf("无法获取当前证书文件设置: %v", err)
	}
	
	keyFile, err := settingService.GetKeyFile()
	if err != nil {
		logger.Warningf("无法获取当前密钥文件设置: %v", err)
	}
	
	// 如果已经有证书配置，先检查文件是否存在
	if certFile != "" && keyFile != "" {
		if _, err := os.Stat(certFile); os.IsNotExist(err) {
			logger.Warningf("配置的证书文件不存在: %s", certFile)
			certFile = "" // 清除无效路径
		}
		if _, err := os.Stat(keyFile); os.IsNotExist(err) {
			logger.Warningf("配置的密钥文件不存在: %s", keyFile)
			keyFile = "" // 清除无效路径
		}
		
		// 如果两个文件都有效，跳过自动配置
		if certFile != "" && keyFile != "" {
			logger.Info("面板证书路径已正确配置")
			return
		}
	}
	
	// 自动检测 /root/cert/ 目录下的证书
	certDir := "/root/cert"
	if _, err := os.Stat(certDir); os.IsNotExist(err) {
		logger.Info("未找到证书目录 /root/cert/")
		return
	}
	
	// 获取域名列表（目录名）
	domains, err := os.ReadDir(certDir)
	if err != nil {
		logger.Warningf("无法读取证书目录: %v", err)
		return
	}
	
	// 遍历目录查找有效的证书文件
	var foundCert, foundKey, foundDomain string
	for _, domain := range domains {
		if domain.IsDir() {
			domainName := domain.Name()
			certPath := filepath.Join(certDir, domainName, "fullchain.pem")
			keyPath := filepath.Join(certDir, domainName, "privkey.pem")
			
			// 检查证书文件是否存在
			if _, err := os.Stat(certPath); err == nil {
				if _, err := os.Stat(keyPath); err == nil {
					foundCert = certPath
					foundKey = keyPath
					foundDomain = domainName
					break // 找到第一个有效的证书就停止
				}
			}
		}
	}
	
	// 如果找到了有效的证书，自动配置路径
	if foundCert != "" && foundKey != "" {
		logger.Infof("自动检测到证书文件，正在配置面板证书路径...")
		logger.Infof("域名: %s", foundDomain)
		logger.Infof("证书文件: %s", foundCert)
		logger.Infof("密钥文件: %s", foundKey)
		
		// 设置证书路径
		if err := settingService.SetCertFile(foundCert); err != nil {
			logger.Warningf("设置证书文件路径失败: %v", err)
		} else {
			logger.Info("✅ 证书文件路径设置成功")
		}
		
		if err := settingService.SetKeyFile(foundKey); err != nil {
			logger.Warningf("设置密钥文件路径失败: %v", err)
		} else {
			logger.Info("✅ 密钥文件路径设置成功")
		}
		
		logger.Infof("🎉 面板证书路径自动配置完成！")
		logger.Infof("建议重启面板以使证书配置生效：%s", "systemctl restart x-ui")
	} else {
		logger.Info("未在 /root/cert/ 目录下找到有效的证书文件")
	}
}

func main() {
	logger.Info(fmt.Sprintf("Starting main function with arguments: %v", os.Args))
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

	// Add debug logging before flag.Parse() to diagnose exit code 2
	logger.Info("About to parse command line flags...")
	flag.Parse()
	logger.Info("Command line flags parsed successfully")
	if showVersion {
		fmt.Println(config.GetVersion())
		return
	}

	switch os.Args[1] {
	case "run":
		logger.Info("Parsing 'run' subcommand flags...")
		err := runCmd.Parse(os.Args[2:])
		if err != nil {
			logger.Errorf("Error parsing 'run' subcommand flags: %v", err)
			fmt.Println(err)
			return
		}
		logger.Info("'run' subcommand flags parsed successfully")
		runWebServer()
	case "migrate":
		logger.Info("Executing 'migrate' command")
		migrateDb()
	case "setting":
		logger.Info("Parsing 'setting' subcommand flags...")
		err := settingCmd.Parse(os.Args[2:])
		if err != nil {
			logger.Errorf("Error parsing 'setting' subcommand flags: %v", err)
			fmt.Println(err)
			return
		}
		logger.Info("'setting' subcommand flags parsed successfully")
		if reset {
			resetSetting()
		} else {
			updateSetting(port, username, password, webBasePath, listenIP, resetTwoFactor)
		}
		if show {
			showSetting(show)
		}
		if getListen {
			GetListenIP(getListen)
		}
		if getCert {
			GetCertificate(getCert)
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
