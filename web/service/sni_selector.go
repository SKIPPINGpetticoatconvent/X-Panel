package service

import (
	"math/rand"
	"sync"
	"time"

	"x-ui/logger"
)

// SNISelector 负责管理 SNI 域名的轮询选择，确保不重复使用
type SNISelector struct {
	domains []string // 当前可用的域名列表
	index   int      // 当前读取到的索引
	mu      sync.Mutex
	rng     *rand.Rand
}

// NewSNISelector 创建并初始化一个 SNI 选择器
func NewSNISelector(initialDomains []string) *SNISelector {
	// 复制切片以防外部修改
	if len(initialDomains) == 0 {
		// 如果列表为空，提供默认值防止 panic
		initialDomains = []string{"www.google.com", "www.amazon.com"}
	}

	s := &SNISelector{
		domains: make([]string, len(initialDomains)),
		index:   0,
		rng:     rand.New(rand.NewSource(time.Now().UnixNano())),
	}
	copy(s.domains, initialDomains)

	// 初始化时洗牌，避免每次启动顺序都完全一样
	s.shuffle()

	logger.Infof("SNI selector initialized with %d domains", len(s.domains))
	return s
}

// Next 返回下一个不重复的 SNI 域名
func (s *SNISelector) Next() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.domains) == 0 {
		return ""
	}

	// 检查是否需要重置，开始新的一轮
	if s.index >= len(s.domains) {
		s.index = 0
		s.shuffle()
		logger.Infof("SNI selector reshuffled, starting new round with %d domains", len(s.domains))
	}

	domain := s.domains[s.index]
	s.index++

	logger.Debugf("SNI selector selected domain: %s (index: %d)", domain, s.index-1)
	return domain
}

// shuffle 打乱域名列表顺序
func (s *SNISelector) shuffle() {
	// 使用 Fisher-Yates 洗牌算法
	for i := len(s.domains) - 1; i > 0; i-- {
		// 使用内部的随机数生成器
		j := s.rng.Intn(i + 1)
		s.domains[i], s.domains[j] = s.domains[j], s.domains[i]
	}
}

// UpdateDomains 允许运行时更新域名列表
func (s *SNISelector) UpdateDomains(newDomains []string) {
	if len(newDomains) == 0 {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.domains = make([]string, len(newDomains))
	copy(s.domains, newDomains)
	s.index = 0
	s.shuffle()

	logger.Infof("SNI selector updated with %d domains", len(s.domains))
}

// GetCurrentDomain 获取当前域名（不移动索引）
func (s *SNISelector) GetCurrentDomain() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.domains) == 0 {
		return ""
	}

	currentIndex := s.index
	if currentIndex >= len(s.domains) {
		currentIndex = 0
	}

	return s.domains[currentIndex]
}

// GetDomains 获取当前域名列表的副本
func (s *SNISelector) GetDomains() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	domains := make([]string, len(s.domains))
	copy(domains, s.domains)
	return domains
}

// GetStats 获取选择器统计信息
func (s *SNISelector) GetStats() (totalDomains, currentIndex int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.domains), s.index
}
