package main

import (
	"fmt"
	"log"

	"x-ui/config"
	"x-ui/database"
	"x-ui/logger"
	"x-ui/util/crypto"
	"x-ui/web/service"
)

// CLI 颜色常量
const (
	Reset  = "\033[0m"
	Red    = "\033[31m"
	Green  = "\033[32m"
	Yellow = "\033[33m"
)

// initDBForCLI 初始化数据库用于 CLI 命令
func initDBForCLI() error {
	return database.InitDB(config.GetDBPath())
}

func resetSetting() {
	if err := initDBForCLI(); err != nil {
		fmt.Println("Failed to initialize database:", err)
		return
	}

	settingService := service.SettingService{}
	err := settingService.ResetSettings()
	if err != nil {
		fmt.Println("Failed to reset settings（重置设置失败）:", err)
	} else {
		fmt.Println("Settings successfully reset ---->>重置设置成功")
	}
}

func showSetting(show bool) {
	if !show {
		return
	}

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
		return
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
	hasDefaultCredential := userModel.Username == "admin" && crypto.CheckPasswordHash(userModel.Password, "admin")
	if hasDefaultCredential {
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
	fmt.Println(Green + ">>>>>>>>注：若您安装了〔证书〕，请使用您的域名用https方式登录" + Reset)
	fmt.Println("")
	fmt.Println("--------------------------------------------------")
	fmt.Println("")
	fmt.Printf("%s请确保 %s%d%s 端口已打开放行%s\n", Green, Red, port, Green, Reset)
	fmt.Println(Yellow + "请自行确保此端口没有被其他程序占用" + Reset)
	fmt.Println("")
	fmt.Println("--------------------------------------------------")
	fmt.Println("")
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
		}
		logger.Infof("SetTgbotEnabled[%v] success", status)
	}
}

func updateTgbotSetting(tgBotToken string, tgBotChatid string, tgBotRuntime string) {
	if err := initDBForCLI(); err != nil {
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
	if err := initDBForCLI(); err != nil {
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
			_ = settingService.SetTwoFactorToken("")
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
	if err := initDBForCLI(); err != nil {
		fmt.Println(err)
		return
	}

	if (privateKey != "" && publicKey != "") || (privateKey == "" && publicKey == "") {
		settingService := service.SettingService{}
		err := settingService.SetCertFile(publicKey)
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

func getCertificate(getCert bool) {
	if !getCert {
		return
	}

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

func getListenIP(getListen bool) {
	if !getListen {
		return
	}

	settingService := service.SettingService{}
	listenIP, err := settingService.GetListen()
	if err != nil {
		log.Printf("Failed to retrieve listen IP: %v", err)
		return
	}

	fmt.Println("listenIP:", listenIP)
}

func migrateDb() {
	inboundService := service.InboundService{}

	if err := initDBForCLI(); err != nil {
		log.Fatal(err)
	}
	fmt.Println("Start migrating database... ---->>开始迁移数据库...")
	inboundService.MigrateDB()
	fmt.Println("Migration done! ------------>>迁移完成！")
}
