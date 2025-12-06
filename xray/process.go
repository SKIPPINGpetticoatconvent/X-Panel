package xray

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"syscall"
	"time"

	"x-ui/config"
	"x-ui/logger"
	"x-ui/util/common"
	"x-ui/util/security"
)

func GetBinaryName() string {
	return fmt.Sprintf("xray-%s-%s", runtime.GOOS, runtime.GOARCH)
}

func GetBinaryPath() string {
	return config.GetBinFolderPath() + "/" + GetBinaryName()
}

func GetConfigPath() string {
	return config.GetBinFolderPath() + "/config.json"
}

func GetGeositePath() string {
	return config.GetBinFolderPath() + "/geosite.dat"
}

func GetGeoipPath() string {
	return config.GetBinFolderPath() + "/geoip.dat"
}

func GetIPLimitLogPath() string {
	return config.GetLogFolder() + "/3xipl.log"
}

func GetIPLimitBannedLogPath() string {
	return config.GetLogFolder() + "/3xipl-banned.log"
}

func GetIPLimitBannedPrevLogPath() string {
	return config.GetLogFolder() + "/3xipl-banned.prev.log"
}

func GetAccessPersistentLogPath() string {
	return config.GetLogFolder() + "/3xipl-ap.log"
}

func GetAccessPersistentPrevLogPath() string {
	return config.GetLogFolder() + "/3xipl-ap.prev.log"
}

func GetAccessLogPath() (string, error) {
	config, err := os.ReadFile(GetConfigPath())
	if err != nil {
		logger.Warningf("Failed to read configuration file: %s", err)
		return "", err
	}

	jsonConfig := map[string]any{}
	err = json.Unmarshal([]byte(config), &jsonConfig)
	if err != nil {
		logger.Warningf("Failed to parse JSON configuration: %s", err)
		return "", err
	}

	if jsonConfig["log"] != nil {
		jsonLog := jsonConfig["log"].(map[string]any)
		if jsonLog["access"] != nil {
			accessLogPath := jsonLog["access"].(string)
			return accessLogPath, nil
		}
	}
	return "", err
}

func stopProcess(p *Process) {
	p.Stop()
}

type Process struct {
	*process
}

func NewProcess(xrayConfig *Config) *Process {
	p := &Process{newProcess(xrayConfig)}
	runtime.SetFinalizer(p, stopProcess)
	return p
}

type process struct {
	cmd *exec.Cmd

	version string
	apiPort int

	onlineClients []string
	mutex         sync.RWMutex

	config    *Config
	logWriter *LogWriter
	exitErr   error
	startTime time.Time
}

func newProcess(config *Config) *process {
	return &process{
		version:   "Unknown",
		config:    config,
		logWriter: NewLogWriter(),
		startTime: time.Now(),
	}
}

func (p *process) IsRunning() bool {
	if p.cmd == nil || p.cmd.Process == nil {
		return false
	}
	if p.cmd.ProcessState == nil {
		return true
	}
	return false
}

func (p *process) GetErr() error {
	return p.exitErr
}

func (p *process) GetResult() string {
	if len(p.logWriter.lastLine) == 0 && p.exitErr != nil {
		return p.exitErr.Error()
	}
	return p.logWriter.lastLine
}

func (p *process) GetVersion() string {
	return p.version
}

func (p *Process) GetAPIPort() int {
	return p.apiPort
}

func (p *Process) GetConfig() *Config {
	return p.config
}

func (p *Process) GetOnlineClients() []string {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	clientsCopy := make([]string, len(p.onlineClients))
	copy(clientsCopy, p.onlineClients)
	return clientsCopy
}

func (p *Process) SetOnlineClients(clients []string) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.onlineClients = clients
}

func (p *Process) GetUptime() uint64 {
	return uint64(time.Since(p.startTime).Seconds())
}

func (p *process) refreshAPIPort() {
	for _, inbound := range p.config.InboundConfigs {
		if inbound.Tag == "api" {
			p.apiPort = inbound.Port
			break
		}
	}
}

func (p *process) refreshVersion() {
	// 获取二进制路径并验证
	binaryPath := GetBinaryPath()
	if err := security.ValidateFilePath(binaryPath); err != nil {
		logger.Warningf("Xray 二进制路径验证失败: %v", err)
		p.version = "Unknown"
		return
	}
	
	// 构建版本查询命令参数
	args := []string{"-version"}
	if err := security.ValidateCommandArgs(args); err != nil {
		logger.Warningf("版本查询命令参数验证失败: %v", err)
		p.version = "Unknown"
		return
	}
	
	cmd := exec.Command(binaryPath, args...)
	data, err := cmd.Output()
	if err != nil {
		p.version = "Unknown"
	} else {
		datas := bytes.Split(data, []byte(" "))
		if len(datas) <= 1 {
			p.version = "Unknown"
		} else {
			p.version = string(datas[1])
		}
	}
}

func (p *process) Start() (err error) {
	if p.IsRunning() {
		return errors.New("xray is already running")
	}

	defer func() {
		if err != nil {
			logger.Error("Failure in running xray-core process: ", err)
			p.exitErr = err
		}
	}()

	data, err := json.MarshalIndent(p.config, "", "  ")
	if err != nil {
		return common.NewErrorf("Failed to generate XRAY configuration files: %v", err)
	}

	err = os.MkdirAll(config.GetLogFolder(), 0o770)
	if err != nil {
		logger.Warningf("Failed to create log folder: %s", err)
	}

	configPath := GetConfigPath()
	
	// 验证配置文件路径安全性
	if err := security.ValidateFilePath(configPath); err != nil {
		return common.NewErrorf("配置文件路径验证失败: %v", err)
	}
	
	err = os.WriteFile(configPath, data, fs.ModePerm)
	if err != nil {
		return common.NewErrorf("Failed to write configuration file: %v", err)
	}

	// 获取二进制路径并验证
	binaryPath := GetBinaryPath()
	if err := security.ValidateFilePath(binaryPath); err != nil {
		return common.NewErrorf("Xray 二进制路径验证失败: %v", err)
	}
	
	// 构建启动命令参数
	args := []string{"-c", configPath}
	if err := security.ValidateCommandArgs(args); err != nil {
		return common.NewErrorf("启动命令参数验证失败: %v", err)
	}
	
	cmd := exec.Command(binaryPath, args...)
	p.cmd = cmd

	cmd.Stdout = p.logWriter
	cmd.Stderr = p.logWriter

	go func() {
		err := cmd.Run()
		if err != nil {
			logger.Error("Failure in running xray-core:", err)
			p.exitErr = err
		}
	}()

	p.refreshVersion()
	p.refreshAPIPort()

	return nil
}

func (p *process) Stop() error {
	if !p.IsRunning() {
		return errors.New("xray is not running")
	}
	
	if runtime.GOOS == "windows" {
		return p.cmd.Process.Kill()
	} else {
		return p.cmd.Process.Signal(syscall.SIGTERM)
	}
}

func writeCrashReport(m []byte) error {
	crashReportPath := config.GetBinFolderPath() + "/core_crash_" + time.Now().Format("20060102_150405") + ".log"
	return os.WriteFile(crashReportPath, m, os.ModePerm)
}
