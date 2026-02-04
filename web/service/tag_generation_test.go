package service

import (
	"testing"

	"x-ui/util/random"
)

// TestTagGeneration 测试 tag 生成逻辑
func TestTagGeneration(t *testing.T) {
	// 模拟网页端 tag 生成

	// 模拟原来的简单生成
	oldTag := "inbound-1"

	// 模拟新的增强生成
	randomSuffix := random.SeqWithCharset(4, "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	newTag := "inbound-1-" + randomSuffix

	t.Logf("旧格式: %s", oldTag)
	t.Logf("新格式: %s", newTag)

	// 验证新格式包含必要元素
	if len(newTag) < len("inbound-1-ABCD") {
		t.Error("新 tag 格式太短")
	}

	// 验证随机性
	generatedTags := make(map[string]bool)
	for i := 0; i < 100; i++ {
		randomSuffix := random.SeqWithCharset(4, "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
		tag := "inbound-1-" + randomSuffix
		generatedTags[tag] = true
	}

	// 验证生成了不同的 tag
	if len(generatedTags) < 90 { // 允许少量重复
		t.Errorf("随机性不足，生成了 %d 个相同的 tag", 100-len(generatedTags))
	}

	t.Logf("成功生成了 %d 个不同的 tag", len(generatedTags))
}

// TestConcurrentTagGeneration 测试并发 tag 生成
func TestConcurrentTagGeneration(t *testing.T) {
	generatedTags := make(chan string, 100)

	// 并发生成 100 个 tag
	for i := 0; i < 100; i++ {
		go func(id int) {
			randomSuffix := random.SeqWithCharset(4, "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
			tag := "inbound-1-" + randomSuffix
			generatedTags <- tag
		}(i)
	}

	// 收集所有 tag
	uniqueTags := make(map[string]bool)
	for i := 0; i < 100; i++ {
		tag := <-generatedTags
		uniqueTags[tag] = true
	}

	// 验证唯一性
	if len(uniqueTags) < 95 { // 允许少量重复
		t.Errorf("并发生成了 %d 个 tag，但只有 %d 个是唯一的", 100, len(uniqueTags))
	}

	t.Logf("并发生成了 %d 个 tag，其中 %d 个是唯一的", 100, len(uniqueTags))
}
