package config

import (
	"os"
	"runtime"
	"strings"

	"github.com/spf13/viper"
)

// initStaticConfig 初始化 Viper 静态配置管理
func initStaticConfig() {
	viper.SetConfigName("config")
	viper.SetConfigType("toml")
	viper.AddConfigPath("/etc/x-ui")
	viper.AddConfigPath(".")
	viper.AddConfigPath(getBaseDir())

	// 环境变量设置
	viper.SetEnvPrefix("XUI")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// 设置默认值
	setStaticDefaults()

	// 读取配置文件（配置文件是可选的，不存在时静默使用默认值）
	_ = viper.ReadInConfig()
}

// RefreshEnvConfig 刷新环境变量配置（用于测试）
func RefreshEnvConfig() {
	viper.SetEnvPrefix("XUI")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	// 强制重新读取所有环境变量
	viper.Set("app.debug", os.Getenv("XUI_DEBUG") == "true")
	viper.Set("app.log_level", os.Getenv("XUI_LOG_LEVEL"))
	viper.Set("paths.bin_folder", os.Getenv("XUI_BIN_FOLDER"))
	viper.Set("paths.db_folder", os.Getenv("XUI_DB_FOLDER"))
	viper.Set("paths.log_folder", os.Getenv("XUI_LOG_FOLDER"))
	viper.Set("paths.sni_folder", os.Getenv("XUI_SNI_FOLDER"))
}

// setStaticDefaults 设置静态配置的默认值
func setStaticDefaults() {
	// 应用默认值
	viper.SetDefault("app.name", "x-ui")
	viper.SetDefault("app.debug", false)
	viper.SetDefault("app.log_level", "info")

	// 路径默认值
	viper.SetDefault("paths.bin_folder", "bin")
	viper.SetDefault("paths.sni_folder", "sni")

	// 平台特定默认值
	if runtime.GOOS == "windows" {
		viper.SetDefault("paths.db_folder", getBaseDir())
		viper.SetDefault("paths.log_folder", "./log")
	} else {
		viper.SetDefault("paths.db_folder", "/etc/x-ui")
		viper.SetDefault("paths.log_folder", "/var/log")
	}
}
