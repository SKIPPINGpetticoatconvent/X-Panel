package service

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

// PublicIPDetector 公网 IP 检测服务
type PublicIPDetector struct {
	services []string
	timeout  time.Duration
}

// NewPublicIPDetector 创建新的公网 IP 检测器
func NewPublicIPDetector() *PublicIPDetector {
	return &PublicIPDetector{
		services: []string{
			"https://api.ipify.org",
			"https://ifconfig.me/ip",
			"https://icanhazip.com",
			"https://ip.sb",
			"https://api.ip.sb/ip",
		},
		timeout: 5 * time.Second,
	}
}

// GetPublicIP 获取机器的公网 IP
func (d *PublicIPDetector) GetPublicIP() (string, error) {
	client := &http.Client{
		Timeout: d.timeout,
	}

	// 使用通道来接收第一个成功的响应
	type result struct {
		ip  string
		err error
	}
	resultChan := make(chan result, len(d.services))

	// 并行请求所有服务
	for _, service := range d.services {
		go func(url string) {
			ip, err := d.fetchIPFromService(client, url)
			resultChan <- result{ip: ip, err: err}
		}(service)
	}

	// 等待第一个成功的结果
	for i := 0; i < len(d.services); i++ {
		res := <-resultChan
		if res.err == nil && res.ip != "" && d.ValidateIP(res.ip) {
			return res.ip, nil
		}
	}

	return "", fmt.Errorf("无法获取公网 IP，所有服务都不可用")
}

// fetchIPFromService 从单个服务获取 IP
func (d *PublicIPDetector) fetchIPFromService(client *http.Client, url string) (string, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("User-Agent", "Xray-UI-Panel/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	ip := strings.TrimSpace(string(body))
	return ip, nil
}

// ValidateIP 验证 IP 是否有效
func (d *PublicIPDetector) ValidateIP(ip string) bool {
	if ip == "" || ip == "N/A" {
		return false
	}

	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return false
	}

	// 排除私有 IP 地址
	return !d.isPrivateIP(parsedIP)
}

// isPrivateIP 检查是否为私有 IP
func (d *PublicIPDetector) isPrivateIP(ip net.IP) bool {
	privateCIDRs := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"127.0.0.0/8",
		"169.254.0.0/16", // Link-local
		"::1/128",        // IPv6 loopback
		"fe80::/10",      // IPv6 link-local
		"fc00::/7",       // IPv6 unique local
	}

	for _, cidr := range privateCIDRs {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if network.Contains(ip) {
			return true
		}
	}

	return false
}

// IsIPv4 检查是否为 IPv4
func (d *PublicIPDetector) IsIPv4(ip string) bool {
	parsedIP := net.ParseIP(ip)
	return parsedIP != nil && parsedIP.To4() != nil
}

// IsIPv6 检查是否为 IPv6
func (d *PublicIPDetector) IsIPv6(ip string) bool {
	parsedIP := net.ParseIP(ip)
	return parsedIP != nil && parsedIP.To4() == nil && parsedIP.To16() != nil
}
