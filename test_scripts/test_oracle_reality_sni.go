package main

import (
	"fmt"
	"reflect"
	"strings"
)

func GenerateRealityServerNames(host string) []string {
	// 1. 去除端口
	domain := host
	if strings.Contains(host, ":") {
		domain = strings.Split(host, ":")[0]
	}

	// 2. 初始化结果列表
	serverNames := make([]string, 0, 2)

	// 3. 判断是否以 www. 开头
	if strings.HasPrefix(domain, "www.") {
		// 情况 A: 输入 www.walmart.com
		// 添加原始域名: www.walmart.com
		serverNames = append(serverNames, domain)

		// 添加根域名: walmart.com
		rootDomain := strings.TrimPrefix(domain, "www.")
		if rootDomain != "" {
			serverNames = append(serverNames, rootDomain)
		}
	} else {
		// 情况 B: 输入 walmart.com
		// 添加原始域名: walmart.com
		serverNames = append(serverNames, domain)

		// 添加 www 域名: www.walmart.com
		// 注意：对于多级子域名 (api.walmart.com)，这里也会生成 www.api.walmart.com，
		// 虽然不一定常用，但在 Reality 配置中通常是安全的或者是为了伪装。
		// 核心目标是避免 www.www.
		serverNames = append(serverNames, "www."+domain)
	}

	return serverNames
}

func main() {
	fmt.Println("🧪 开始测试 Oracle Reality SNI 修复...")

	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "Test www.oracle.com:443 - should NOT generate www.www.oracle.com",
			input:    "www.oracle.com:443",
			expected: []string{"www.oracle.com", "oracle.com"},
		},
		{
			name:     "Test oracle.com:443 - should generate both with and without www",
			input:    "oracle.com:443",
			expected: []string{"oracle.com", "www.oracle.com"},
		},
		{
			name:     "Test www.www.oracle.com - edge case with double www",
			input:    "www.www.oracle.com:443",
			expected: []string{"www.www.oracle.com", "www.oracle.com"},
		},
	}

	allPassed := true

	for _, tt := range tests {
		fmt.Printf("\n📝 测试: %s\n", tt.name)
		result := GenerateRealityServerNames(tt.input)

		fmt.Printf("输入: %s\n", tt.input)
		fmt.Printf("期望输出: %v\n", tt.expected)
		fmt.Printf("实际输出: %v\n", result)

		if !reflect.DeepEqual(result, tt.expected) {
			fmt.Printf("❌ 测试失败\n")
			allPassed = false
		} else {
			fmt.Printf("✅ 测试通过\n")
		}
	}

	fmt.Println("\n" + strings.Repeat("=", 50))
	if allPassed {
		fmt.Println("🎉 所有测试通过！Oracle Reality SNI 修复验证成功")
	} else {
		fmt.Println("💥 存在测试失败，需要进一步调试")
	}
}
