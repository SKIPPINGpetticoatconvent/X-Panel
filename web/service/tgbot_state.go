package service

import (
	"sync"
	"time"

	"x-ui/web/global"

	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
)

// BotState 封装 Telegram Bot 的所有运行时状态
// 将原有的全局变量收敛到此结构体中，提升可测试性和线程安全性
type BotState struct {
	bot         *telego.Bot
	botHandler  *th.BotHandler
	adminIds    []int64
	isRunning   bool
	hostname    string
	hashStorage *global.HashStorage

	// 用户状态管理（用于多步交互）
	userStates map[int64]string
	stateMu    sync.RWMutex

	// 客户端添加时的临时数据
	clientData *ClientFormData
	clientMu   sync.Mutex
}

// ClientFormData 封装添加客户端时的表单数据
type ClientFormData struct {
	ReceiverInboundID int
	ID                string
	Flow              string
	Email             string
	LimitIP           int
	TotalGB           int64
	ExpiryTime        int64
	Enable            bool
	TgID              string
	SubID             string
	Comment           string
	Reset             int
	Security          string
	ShPassword        string
	TrPassword        string
	Method            string
}

// NewBotState 创建新的 BotState 实例
func NewBotState() *BotState {
	return &BotState{
		userStates: make(map[int64]string),
		clientData: &ClientFormData{},
	}
}

// InitHashStorage 初始化哈希存储
func (s *BotState) InitHashStorage(ttl time.Duration) {
	s.hashStorage = global.NewHashStorage(ttl)
}

// GetBot 获取 Bot 实例
func (s *BotState) GetBot() *telego.Bot {
	return s.bot
}

// SetBot 设置 Bot 实例
func (s *BotState) SetBot(b *telego.Bot) {
	s.bot = b
}

// GetBotHandler 获取 BotHandler 实例
func (s *BotState) GetBotHandler() *th.BotHandler {
	return s.botHandler
}

// SetBotHandler 设置 BotHandler 实例
func (s *BotState) SetBotHandler(h *th.BotHandler) {
	s.botHandler = h
}

// GetAdminIds 获取管理员 ID 列表
func (s *BotState) GetAdminIds() []int64 {
	return s.adminIds
}

// SetAdminIds 设置管理员 ID 列表
func (s *BotState) SetAdminIds(ids []int64) {
	s.adminIds = ids
}

// AddAdminId 添加管理员 ID
func (s *BotState) AddAdminId(id int64) {
	s.adminIds = append(s.adminIds, id)
}

// ClearAdminIds 清空管理员 ID 列表
func (s *BotState) ClearAdminIds() {
	s.adminIds = []int64{}
}

// IsRunning 检查 Bot 是否正在运行
func (s *BotState) IsRunning() bool {
	return s.isRunning
}

// SetRunning 设置运行状态
func (s *BotState) SetRunning(running bool) {
	s.isRunning = running
}

// GetHostname 获取主机名
func (s *BotState) GetHostname() string {
	return s.hostname
}

// SetHostname 设置主机名
func (s *BotState) SetHostname(h string) {
	s.hostname = h
}

// GetHashStorage 获取哈希存储
func (s *BotState) GetHashStorage() *global.HashStorage {
	return s.hashStorage
}

// GetUserState 线程安全地获取用户状态
func (s *BotState) GetUserState(userId int64) (string, bool) {
	s.stateMu.RLock()
	defer s.stateMu.RUnlock()
	state, exists := s.userStates[userId]
	return state, exists
}

// SetUserState 线程安全地设置用户状态
func (s *BotState) SetUserState(userId int64, state string) {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	s.userStates[userId] = state
}

// DeleteUserState 线程安全地删除用户状态
func (s *BotState) DeleteUserState(userId int64) {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	delete(s.userStates, userId)
}

// GetClientData 获取客户端表单数据
func (s *BotState) GetClientData() *ClientFormData {
	s.clientMu.Lock()
	defer s.clientMu.Unlock()
	return s.clientData
}

// SetClientData 设置客户端表单数据
func (s *BotState) SetClientData(data *ClientFormData) {
	s.clientMu.Lock()
	defer s.clientMu.Unlock()
	s.clientData = data
}

// ResetClientData 重置客户端表单数据
func (s *BotState) ResetClientData() {
	s.clientMu.Lock()
	defer s.clientMu.Unlock()
	s.clientData = &ClientFormData{}
}

// CheckAdmin 检查用户是否为管理员
func (s *BotState) CheckAdmin(userId int64) bool {
	for _, adminId := range s.adminIds {
		if visitorId := userId; visitorId == adminId {
			return true
		}
	}
	return false
}

// Stop 停止 Bot 并清理状态
func (s *BotState) Stop() {
	if s.botHandler != nil {
		_ = s.botHandler.Stop()
	}
	s.isRunning = false
	s.adminIds = nil
}
