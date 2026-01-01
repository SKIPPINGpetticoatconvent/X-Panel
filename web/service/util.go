package service

import (
	"strings"
)

// GenerateRealityServerNames 根据输入的 host 生成合理的 SNI 列表
// 输入: host (例如 "www.walmart.com:443", "google.com")
// 输出: []string (例如 ["www.walmart.com", "walmart.com"], ["google.com", "www.google.com"])
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
		serverNames = append(serverNames, "www."+domain)
	}

	return serverNames
}
