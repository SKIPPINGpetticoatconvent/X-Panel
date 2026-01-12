package service

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"x-ui/logger"
)

// RenewalState 续期状态持久化
type RenewalState struct {
	LastCheckTime    time.Time `json:"last_check_time"`
	LastRenewalTime  time.Time `json:"last_renewal_time"`
	ConsecutiveFails int       `json:"consecutive_fails"`
	IsRenewing       bool      `json:"is_renewing"`
}

// RenewalConfig 配置续期参数
type RenewalConfig struct {
	CheckInterval  time.Duration // 检查间隔，默认 6 小时
	RenewThreshold time.Duration // 续期阈值，默认 3 天
	MaxRetries     int           // 最大重试次数，默认 12 次
	RetryInterval  time.Duration // 重试间隔，默认 30 分钟
}

// AggressiveRenewalManager 实现激进续期策略
type AggressiveRenewalManager struct {
	config        RenewalConfig
	certService   *CertService
	portResolver  *PortConflictResolver
	alertFallback *CertAlertFallback
	stopChan      chan struct{}
	retryCount    int
	renewMutex    sync.Mutex
	isRenewing    bool
	stateFile     string
}

// NewAggressiveRenewalManager 创建新的激进续期管理器
func NewAggressiveRenewalManager(config RenewalConfig, certService *CertService, portResolver *PortConflictResolver, alertFallback *CertAlertFallback) *AggressiveRenewalManager {
	return &AggressiveRenewalManager{
		config:        config,
		certService:   certService,
		portResolver:  portResolver,
		alertFallback: alertFallback,
		stopChan:      make(chan struct{}),
		retryCount:    0,
		stateFile:     "config/renewal_state.json",
	}
}

// loadState 加载续期状态
func (m *AggressiveRenewalManager) loadState() (*RenewalState, error) {
	data, err := os.ReadFile(m.stateFile)
	if err != nil {
		if os.IsNotExist(err) {
			// 文件不存在，返回默认状态
			return &RenewalState{}, nil
		}
		return nil, fmt.Errorf("failed to read state file: %w", err)
	}

	var state RenewalState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal state: %w", err)
	}

	return &state, nil
}

// saveState 保存续期状态
func (m *AggressiveRenewalManager) saveState(state *RenewalState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	if err := os.WriteFile(m.stateFile, data, 0o644); err != nil {
		return fmt.Errorf("failed to write state file: %w", err)
	}

	return nil
}

// Start 启动续期调度器
func (m *AggressiveRenewalManager) Start() {
	logger.Info("Starting aggressive renewal manager...")

	// 加载上次状态
	state, err := m.loadState()
	if err != nil {
		logger.Warningf("Failed to load renewal state: %v", err)
		state = &RenewalState{}
	}
	m.isRenewing = state.IsRenewing

	// 计算启动延迟
	now := time.Now()
	initialDelay := 5 * time.Minute // 启动冷却期 5 分钟

	// 如果有上次检查时间，计算剩余时间
	if !state.LastCheckTime.IsZero() {
		elapsed := now.Sub(state.LastCheckTime)
		if elapsed < m.config.CheckInterval {
			remaining := m.config.CheckInterval - elapsed
			if remaining > initialDelay {
				initialDelay = remaining
			}
		}
	}

	logger.Infof("Initial check delay: %v", initialDelay)
	time.Sleep(initialDelay)

	go m.runLoop()
}

// Stop 停止调度器
func (m *AggressiveRenewalManager) Stop() {
	logger.Info("Stopping aggressive renewal manager...")

	// 等待当前续期操作完成
	m.renewMutex.Lock()
	defer m.renewMutex.Unlock()

	// 保存当前状态
	state := &RenewalState{
		LastCheckTime:    time.Now(),                      // 最后检查时间设为现在，因为停止时可能正在检查
		LastRenewalTime:  time.Now().Add(-24 * time.Hour), // 默认上次续期时间为24小时前
		ConsecutiveFails: 0,                               // 重置失败计数
		IsRenewing:       false,
	}
	if err := m.saveState(state); err != nil {
		logger.Warningf("Failed to save state on stop: %v", err)
	}

	close(m.stopChan)
}

// runLoop 运行主循环
func (m *AggressiveRenewalManager) runLoop() {
	ticker := time.NewTicker(m.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := m.CheckAndRenew(); err != nil {
				logger.Warningf("CheckAndRenew failed: %v", err)
			}
		case <-m.stopChan:
			return
		}
	}
}

// CheckAndRenew 检查并执行续期
func (m *AggressiveRenewalManager) CheckAndRenew() error {
	// 检查并发锁
	m.renewMutex.Lock()
	if m.isRenewing {
		m.renewMutex.Unlock()
		logger.Info("Renewal already in progress, skipping check")
		return nil
	}
	m.renewMutex.Unlock()

	// 更新检查时间
	state, err := m.loadState()
	if err != nil {
		logger.Warningf("Failed to load state for check: %v", err)
		state = &RenewalState{}
	}
	state.LastCheckTime = time.Now()
	if saveErr := m.saveState(state); saveErr != nil {
		logger.Warningf("Failed to save check time: %v", saveErr)
	}

	// 检查是否启用 IP 证书
	enabled, err := m.certService.settingService.GetIpCertEnable()
	if err != nil {
		return fmt.Errorf("failed to get IP cert enable status: %w", err)
	}
	if !enabled {
		return nil // 未启用，跳过
	}

	// 获取证书信息
	certInfo, err := m.getCertInfo()
	if err != nil {
		logger.Warningf("Failed to get certificate info: %v, attempting renewal", err)
		return m.attemptRenewWithRetry()
	}

	// 检查是否需要续期
	remaining := time.Until(certInfo.Expiry)
	if remaining < m.config.RenewThreshold {
		logger.Infof("Certificate expires in %v, triggering aggressive renewal", remaining)
		return m.attemptRenewWithRetry()
	}

	logger.Infof("Certificate check completed, expires in %v", remaining)
	return nil
}

// getCertInfo 获取证书信息
func (m *AggressiveRenewalManager) getCertInfo() (*CertInfo, error) {
	// 获取 IP 地址
	ip, err := m.certService.settingService.GetIpCertTarget()
	if err != nil {
		return nil, fmt.Errorf("failed to get IP cert target: %w", err)
	}
	if ip == "" {
		return nil, errors.New("IP cert target is empty")
	}

	// 使用 LegoIPService 获取证书信息
	legoCertInfo, err := m.certService.legoIPService.GetCertInfo(ip)
	if err != nil {
		logger.Warningf("Failed to get cert info from Lego service, falling back to file parsing: %v", err)

		// 回退到文件解析方式
		certPath, err := m.certService.settingService.GetIpCertPath()
		if err != nil {
			return nil, fmt.Errorf("failed to get IP cert path: %w", err)
		}
		if certPath == "" {
			return nil, errors.New("IP cert path is empty")
		}

		certFile := certPath + ".crt"
		keyFile := certPath + ".key"

		// 检查证书文件是否存在
		if _, err := os.Stat(certFile); os.IsNotExist(err) {
			return nil, errors.New("certificate file does not exist")
		}

		// 加载证书
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load certificate: %w", err)
		}

		// 解析证书获取过期时间
		if len(cert.Certificate) == 0 {
			return nil, errors.New("no certificate data found")
		}

		x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
		if err != nil {
			return nil, fmt.Errorf("failed to parse X.509 certificate: %w", err)
		}

		return &CertInfo{
			Path:   fmt.Sprintf("/etc/ssl/certs/ip_%s.crt", strings.ReplaceAll(ip, ".", "_")),
			Expiry: x509Cert.NotAfter,
		}, nil
	}

	return &CertInfo{
		Path:   fmt.Sprintf("/etc/ssl/certs/ip_%s.crt", strings.ReplaceAll(ip, ".", "_")),
		Expiry: legoCertInfo.Expiry,
	}, nil
}

// CertInfo 证书信息
type CertInfo struct {
	Path   string
	Expiry time.Time
}

// attemptRenewWithRetry 尝试续期并重试
func (m *AggressiveRenewalManager) attemptRenewWithRetry() error {
	m.renewMutex.Lock()
	if m.isRenewing {
		m.renewMutex.Unlock()
		logger.Info("Renewal already in progress, skipping attempt")
		return nil
	}
	m.isRenewing = true
	m.renewMutex.Unlock()

	// 更新状态
	state, err := m.loadState()
	if err != nil {
		logger.Warningf("Failed to load state for renewal: %v", err)
		state = &RenewalState{}
	}
	state.IsRenewing = true
	if saveErr := m.saveState(state); saveErr != nil {
		logger.Warningf("Failed to save renewing state: %v", saveErr)
	}

	m.retryCount = 0
	err = m.scheduleRetry(0)

	// 重置状态
	m.renewMutex.Lock()
	m.isRenewing = false
	m.renewMutex.Unlock()

	state.IsRenewing = false
	if !state.LastRenewalTime.IsZero() && err == nil {
		state.LastRenewalTime = time.Now()
	}
	if saveErr := m.saveState(state); saveErr != nil {
		logger.Warningf("Failed to save state after renewal: %v", saveErr)
	}

	return err
}

// scheduleRetry 安排重试
func (m *AggressiveRenewalManager) scheduleRetry(attempt int) error {
	if attempt >= m.config.MaxRetries {
		logger.Errorf("Renewal failed after %d attempts", m.config.MaxRetries)

		// 所有重试失败，触发回退机制
		if m.alertFallback != nil {
			if err := m.alertFallback.TriggerFallback(); err != nil {
				logger.Errorf("Failed to trigger fallback: %v", err)
			}
		}

		// 更新连续失败次数
		state, loadErr := m.loadState()
		if loadErr != nil {
			logger.Warningf("Failed to load state for failure count: %v", loadErr)
			state = &RenewalState{}
		}
		state.ConsecutiveFails++
		if saveErr := m.saveState(state); saveErr != nil {
			logger.Warningf("Failed to save failure count: %v", saveErr)
		}

		return WrapError(ErrCodeRenewalFailed, fmt.Errorf("renewal failed after max retries"))
	}

	m.retryCount = attempt + 1
	backoff := m.calculateBackoff(attempt)

	logger.Infof("Scheduling renewal attempt %d/%d in %v", m.retryCount, m.config.MaxRetries, backoff)

	time.Sleep(backoff)

	// 执行续期
	if err := m.performRenewal(); err != nil {
		logger.Warningf("Renewal attempt %d failed: %v", m.retryCount, err)
		// 如果是证书错误，使用标准化错误码
		if IsCertError(err) {
			return m.scheduleRetry(attempt + 1)
		}
		// 其他错误包装为续期失败
		wrappedErr := WrapError(ErrCodeRenewalFailed, err)
		return wrappedErr
	}

	logger.Infof("Renewal attempt %d succeeded", m.retryCount)
	m.retryCount = 0 // 重置重试计数
	return nil
}

// performRenewal 执行续期逻辑
func (m *AggressiveRenewalManager) performRenewal() error {
	// 获取必要参数
	ip, err := m.certService.settingService.GetIpCertTarget()
	if err != nil {
		return fmt.Errorf("failed to get IP cert target: %w", err)
	}
	if ip == "" {
		return errors.New("IP cert target is empty")
	}

	email, err := m.certService.settingService.GetIpCertEmail()
	if err != nil {
		return fmt.Errorf("failed to get IP cert email: %w", err)
	}
	if email == "" {
		return errors.New("IP cert email is empty")
	}

	ctx := context.Background()

	// 使用端口冲突解决器
	if err := m.portResolver.AcquirePort80(ctx); err != nil {
		return fmt.Errorf("failed to acquire port 80: %w", err)
	}
	defer func() {
		if releaseErr := m.portResolver.ReleasePort80(); releaseErr != nil {
			logger.Warningf("Failed to release port 80: %v", releaseErr)
		}
	}()

	// 执行证书获取
	if err := m.certService.ObtainIPCert(ip, email); err != nil {
		// 续期失败，通知告警模块
		if m.alertFallback != nil {
			if alertErr := m.alertFallback.OnRenewalFailed(err, m.retryCount); alertErr != nil {
				logger.Warningf("Failed to handle renewal failure: %v", alertErr)
			}
		}
		return fmt.Errorf("failed to obtain certificate: %w", err)
	}

	// 续期成功，重置告警状态
	if m.alertFallback != nil {
		m.alertFallback.consecutiveFailures = 0
		m.alertFallback.lastSuccessTime = time.Now()
		m.alertFallback.inFallbackMode = false
	}

	// 更新续期时间
	state, err := m.loadState()
	if err != nil {
		logger.Warningf("Failed to load state after successful renewal: %v", err)
		state = &RenewalState{}
	}
	state.LastRenewalTime = time.Now()
	state.ConsecutiveFails = 0
	if saveErr := m.saveState(state); saveErr != nil {
		logger.Warningf("Failed to save renewal time: %v", saveErr)
	}

	return nil
}

// calculateBackoff 计算退避时间
func (m *AggressiveRenewalManager) calculateBackoff(attempt int) time.Duration {
	if attempt == 0 {
		return 0 // 第一次尝试立即执行
	}

	// 指数退避：RetryInterval * 2^(attempt-1)
	backoff := time.Duration(attempt) * m.config.RetryInterval

	// 设置上限，避免过长等待
	maxBackoff := 1 * time.Hour
	if backoff > maxBackoff {
		backoff = maxBackoff
	}

	return backoff
}
