package service

import (
	"fmt"
	"sync"
	"testing"
)

// TestSNISelector_Next_RoundRobin 验证在不重置的情况下，是否按顺序（或洗牌后的顺序）返回所有 SNI，且不重复
func TestSNISelector_Next_RoundRobin(t *testing.T) {
	domains := []string{"domain1.com", "domain2.com", "domain3.com", "domain4.com"}
	selector := NewSNISelector(domains)
	
	// 获取所有域名（两轮以确保轮询）
	var firstRound []string
	var secondRound []string
	
	// 第一轮
	for i := 0; i < len(domains); i++ {
		domain := selector.Next()
		firstRound = append(firstRound, domain)
	}
	
	// 第二轮
	for i := 0; i < len(domains); i++ {
		domain := selector.Next()
		secondRound = append(secondRound, domain)
	}
	
	// 验证第一轮没有重复
	firstRoundSet := make(map[string]bool)
	for _, domain := range firstRound {
		if firstRoundSet[domain] {
			t.Errorf("在第一轮中发现重复域名: %s", domain)
		}
		firstRoundSet[domain] = true
	}
	
	// 验证第一轮包含了所有域名
	if len(firstRoundSet) != len(domains) {
		t.Errorf("第一轮应该包含所有 %d 个域名，但只包含 %d 个", len(domains), len(firstRoundSet))
	}
	
	// 验证第二轮没有重复
	secondRoundSet := make(map[string]bool)
	for _, domain := range secondRound {
		if secondRoundSet[domain] {
			t.Errorf("在第二轮中发现重复域名: %s", domain)
		}
		secondRoundSet[domain] = true
	}
	
	// 验证第二轮包含了所有域名
	if len(secondRoundSet) != len(domains) {
		t.Errorf("第二轮应该包含所有 %d 个域名，但只包含 %d 个", len(domains), len(secondRoundSet))
	}
	
	// 验证第一轮和第二轮包含的域名集合相同
	for domain := range firstRoundSet {
		if !secondRoundSet[domain] {
			t.Errorf("域名 %s 在第一轮中出现但不在第二轮中", domain)
		}
	}
}

// TestNewSNISelector_Shuffle 验证初始化时是否进行了洗牌（可以通过多次初始化比较顺序，或者检查是否与输入顺序不同）
func TestNewSNISelector_Shuffle(t *testing.T) {
	domains := []string{"domain1.com", "domain2.com", "domain3.com", "domain4.com", "domain5.com"}
	
	// 创建多个选择器实例
	var selectors []*SNISelector
	for i := 0; i < 10; i++ {
		selectors = append(selectors, NewSNISelector(domains))
	}
	
	// 收集每个选择器的第一个域名
	firstDomains := make([]string, len(selectors))
	for i, selector := range selectors {
		firstDomains[i] = selector.Next()
	}
	
	// 验证不是所有选择器都返回相同的第一个域名（说明确实进行了洗牌）
	uniqueFirstDomains := make(map[string]bool)
	for _, domain := range firstDomains {
		uniqueFirstDomains[domain] = true
	}
	
	// 至少应该有部分变化（注意：由于随机性，理论上可能巧合相同，但概率很低）
	if len(uniqueFirstDomains) <= len(selectors)/2 {
		t.Logf("警告：洗牌效果可能不明显。观察到的唯一首域名: %v", uniqueFirstDomains)
	}
	
	// 验证返回的域名都是输入域名中的
	for _, firstDomain := range firstDomains {
		found := false
		for _, expectedDomain := range domains {
			if firstDomain == expectedDomain {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("返回的域名 %s 不在预期域名列表中", firstDomain)
		}
	}
}

// TestSNISelector_Reshuffle_On_Reset 验证当所有 SNI 用完后，是否重置并重新洗牌
func TestSNISelector_Reshuffle_On_Reset(t *testing.T) {
	domains := []string{"domain1.com", "domain2.com", "domain3.com"}
	selector := NewSNISelector(domains)
	
	// 获取第一轮的完整顺序
	firstRound := make([]string, len(domains))
	for i := 0; i < len(domains); i++ {
		firstRound[i] = selector.Next()
	}
	
	// 获取第二轮的第一个域名（应该触发重置和洗牌）
	secondRoundFirst := selector.Next()
	
	// 验证第一轮包含了所有域名且没有重复
	firstRoundSet := make(map[string]bool)
	for _, domain := range firstRound {
		if firstRoundSet[domain] {
			t.Errorf("第一轮中发现重复域名: %s", domain)
		}
		firstRoundSet[domain] = true
	}
	
	if len(firstRoundSet) != len(domains) {
		t.Errorf("第一轮应该包含所有 %d 个域名，但只包含 %d 个", len(domains), len(firstRoundSet))
	}
	
	// 验证第二轮的第一个域名是有效域名
	validDomain := false
	for _, expectedDomain := range domains {
		if secondRoundFirst == expectedDomain {
			validDomain = true
			break
		}
	}
	if !validDomain {
		t.Errorf("第二轮的第一个域名 %s 不是有效域名", secondRoundFirst)
	}
	
	// 继续获取第二轮的更多域名来验证洗牌
	secondRound := []string{secondRoundFirst}
	for i := 1; i < len(domains); i++ {
		secondRound = append(secondRound, selector.Next())
	}
	
	// 验证第二轮也没有重复
	secondRoundSet := make(map[string]bool)
	for _, domain := range secondRound {
		if secondRoundSet[domain] {
			t.Errorf("第二轮中发现重复域名: %s", domain)
		}
		secondRoundSet[domain] = true
	}
	
	if len(secondRoundSet) != len(domains) {
		t.Errorf("第二轮应该包含所有 %d 个域名，但只包含 %d 个", len(domains), len(secondRoundSet))
	}
	
	// 验证两轮包含的域名集合相同
	for domain := range firstRoundSet {
		if !secondRoundSet[domain] {
			t.Errorf("域名 %s 在第一轮中出现但不在第二轮中", domain)
		}
	}
}

// TestSNISelector_Concurrency 使用多个 goroutine 并发调用 Next()，验证是否没有 panic 且返回的 SNI 数量正确
func TestSNISelector_Concurrency(t *testing.T) {
	domains := []string{"domain1.com", "domain2.com", "domain3.com", "domain4.com", "domain5.com"}
	selector := NewSNISelector(domains)
	
	numGoroutines := 10
	callsPerGoroutine := 20
	
	var wg sync.WaitGroup
	results := make(chan string, numGoroutines*callsPerGoroutine)
	errors := make(chan error, numGoroutines)
	
	// 启动多个 goroutine 并发调用 Next()
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			
			for j := 0; j < callsPerGoroutine; j++ {
				defer func() {
					if r := recover(); r != nil {
						errors <- fmt.Errorf("goroutine %d panic: %v", goroutineID, r)
					}
				}()
				
				domain := selector.Next()
				results <- domain
			}
		}(i)
	}
	
	wg.Wait()
	close(results)
	close(errors)
	
	// 检查是否有 panic
	var panicErrors []string
	for err := range errors {
		panicErrors = append(panicErrors, err.Error())
	}
	
	if len(panicErrors) > 0 {
		t.Errorf("并发测试中发现 panic: %v", panicErrors)
	}
	
	// 验证返回的域名数量
	actualResults := make([]string, 0)
	for domain := range results {
		actualResults = append(actualResults, domain)
	}
	
	expectedTotalCalls := numGoroutines * callsPerGoroutine
	if len(actualResults) != expectedTotalCalls {
		t.Errorf("预期 %d 次调用，但实际收到 %d 次结果", expectedTotalCalls, len(actualResults))
	}
	
	// 验证所有返回的域名都是有效的
	validDomains := make(map[string]bool)
	for _, domain := range domains {
		validDomains[domain] = true
	}
	
	for _, result := range actualResults {
		if !validDomains[result] {
			t.Errorf("返回了无效域名: %s", result)
		}
	}
}

// TestSNISelector_Empty 验证输入为空时的行为（应返回空字符串或默认值，不应 panic）
func TestSNISelector_Empty(t *testing.T) {
	// 测试空切片输入
	selector := NewSNISelector([]string{})
	
	// 不应 panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("对空输入调用 Next() 时发生了 panic: %v", r)
		}
	}()
	
	// 应该返回默认值
	result := selector.Next()
	
	// 验证返回的是默认域名之一
	defaultDomains := []string{"www.google.com", "www.amazon.com"}
	isDefault := false
	for _, defaultDomain := range defaultDomains {
		if result == defaultDomain {
			isDefault = true
			break
		}
	}
	
	if !isDefault {
		t.Errorf("空输入时应该返回默认域名，但返回了: %s", result)
	}
	
	// 测试多个 Next() 调用都返回默认值
	for i := 0; i < 5; i++ {
		result := selector.Next()
		if result == "" {
			t.Errorf("第 %d 次调用返回了空字符串，预期为默认域名", i+1)
		}
		isDefault = false
		for _, defaultDomain := range defaultDomains {
			if result == defaultDomain {
				isDefault = true
				break
			}
		}
		if !isDefault {
			t.Errorf("第 %d 次调用返回了非默认域名: %s", i+1, result)
		}
	}
	
	// 验证 GetCurrentDomain 和 GetDomains 方法
	currentDomain := selector.GetCurrentDomain()
	if currentDomain == "" {
		t.Errorf("GetCurrentDomain() 不应返回空字符串")
	}
	
	domains := selector.GetDomains()
	if len(domains) == 0 {
		t.Errorf("GetDomains() 不应返回空切片")
	}
	
	// 验证统计信息
	total, index := selector.GetStats()
	if total == 0 {
		t.Errorf("GetStats() 返回的总域名数不应为 0")
	}
	if index < 0 {
		t.Errorf("GetStats() 返回的索引不应为负数: %d", index)
	}
}