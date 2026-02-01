package service

import (
	"sync"
	"testing"
)

func TestSNISelector_Next_CyclesThroughAllDomains(t *testing.T) {
	domains := []string{"a.com", "b.com", "c.com"}
	selector := NewSNISelector(domains)

	// 调用 Next() 3 次应覆盖所有域名（顺序可能不同因为初始化时会 shuffle）
	seen := make(map[string]bool)
	for i := 0; i < 3; i++ {
		d := selector.Next()
		if d == "" {
			t.Fatal("Next() returned empty string")
		}
		seen[d] = true
	}

	if len(seen) != 3 {
		t.Errorf("Expected 3 unique domains in first round, got %d", len(seen))
	}

	// 验证所有原始域名都被使用
	for _, d := range domains {
		if !seen[d] {
			t.Errorf("Domain %q was not returned in first round", d)
		}
	}
}

func TestSNISelector_Next_ReshufflesAfterFullCycle(t *testing.T) {
	domains := []string{"a.com", "b.com", "c.com"}
	selector := NewSNISelector(domains)

	// 完成第一轮
	for i := 0; i < 3; i++ {
		selector.Next()
	}

	// 第二轮应仍然覆盖所有域名
	seen := make(map[string]bool)
	for i := 0; i < 3; i++ {
		d := selector.Next()
		seen[d] = true
	}

	if len(seen) != 3 {
		t.Errorf("Expected 3 unique domains in second round, got %d", len(seen))
	}
}

func TestSNISelector_UpdateDomains(t *testing.T) {
	selector := NewSNISelector([]string{"old.com"})

	newDomains := []string{"new1.com", "new2.com"}
	selector.UpdateDomains(newDomains)

	got := selector.GetDomains()
	if len(got) != 2 {
		t.Fatalf("Expected 2 domains after update, got %d", len(got))
	}

	// 验证内容正确（顺序可能不同因为 shuffle）
	domainSet := make(map[string]bool)
	for _, d := range got {
		domainSet[d] = true
	}
	for _, d := range newDomains {
		if !domainSet[d] {
			t.Errorf("Domain %q not found after UpdateDomains", d)
		}
	}
}

func TestSNISelector_UpdateDomains_EmptyIgnored(t *testing.T) {
	selector := NewSNISelector([]string{"keep.com"})
	selector.UpdateDomains([]string{})

	got := selector.GetDomains()
	if len(got) != 1 || got[0] != "keep.com" {
		t.Errorf("Empty update should be ignored, got %v", got)
	}
}

func TestSNISelector_GetDomains_ReturnsCopy(t *testing.T) {
	selector := NewSNISelector([]string{"a.com", "b.com"})

	copy1 := selector.GetDomains()
	copy1[0] = "modified.com"

	copy2 := selector.GetDomains()
	for _, d := range copy2 {
		if d == "modified.com" {
			t.Error("GetDomains should return a copy, not a reference to internal state")
		}
	}
}

func TestSNISelector_GetCurrentDomain(t *testing.T) {
	selector := NewSNISelector([]string{"a.com", "b.com"})

	d := selector.GetCurrentDomain()
	if d == "" {
		t.Error("GetCurrentDomain should not return empty string")
	}
}

func TestSNISelector_GetStats(t *testing.T) {
	domains := []string{"a.com", "b.com", "c.com"}
	selector := NewSNISelector(domains)

	total, index := selector.GetStats()
	if total != 3 {
		t.Errorf("Expected total 3, got %d", total)
	}
	if index != 0 {
		t.Errorf("Expected initial index 0, got %d", index)
	}

	selector.Next()
	_, index = selector.GetStats()
	if index != 1 {
		t.Errorf("Expected index 1 after one Next(), got %d", index)
	}
}

func TestSNISelector_EmptyListDefault(t *testing.T) {
	selector := NewSNISelector([]string{})

	got := selector.GetDomains()
	if len(got) != 2 {
		t.Fatalf("Empty init should use defaults, got %d domains", len(got))
	}

	// 默认值是 www.google.com 和 www.amazon.com
	domainSet := make(map[string]bool)
	for _, d := range got {
		domainSet[d] = true
	}
	if !domainSet["www.google.com"] || !domainSet["www.amazon.com"] {
		t.Errorf("Expected default domains, got %v", got)
	}
}

func TestSNISelector_NilListDefault(t *testing.T) {
	selector := NewSNISelector(nil)

	got := selector.GetDomains()
	if len(got) != 2 {
		t.Fatalf("Nil init should use defaults, got %d domains", len(got))
	}
}

func TestSNISelector_ConcurrentSafety(t *testing.T) {
	selector := NewSNISelector([]string{"a.com", "b.com", "c.com"})

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			selector.Next()
		}()
	}
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			selector.GetDomains()
		}()
	}
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			selector.UpdateDomains([]string{"x.com", "y.com"})
		}()
	}
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			selector.GetCurrentDomain()
			selector.GetStats()
		}()
	}

	wg.Wait()
	// 只要不 panic 或 data race 即测试通过
}

func TestSNISelector_GetGeoIPInfo_NilService(t *testing.T) {
	selector := NewSNISelector([]string{"a.com"})
	info := selector.GetGeoIPInfo()
	if info != "GeoIP 服务未初始化" {
		t.Errorf("Expected GeoIP not initialized message, got %q", info)
	}
}
