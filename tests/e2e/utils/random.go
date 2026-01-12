package utils

import (
	"fmt"
	"math/rand"
	"time"
)

// GenerateRandomPort 生成随机端口号
func GenerateRandomPort() int {
	rand.Seed(time.Now().UnixNano())
	// 使用 20000-30000 范围，避免冲突
	return 20000 + rand.Intn(10000)
}

// GenerateRandomRemark 生成随机备注
func GenerateRandomRemark(prefix string) string {
	rand.Seed(time.Now().UnixNano())
	suffix := fmt.Sprintf("%06d", rand.Intn(1000000))
	return fmt.Sprintf("%s-%s", prefix, suffix)
}

// GenerateRandomUUID 生成随机UUID
func GenerateRandomUUID() string {
	rand.Seed(time.Now().UnixNano())
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		rand.Uint32(),
		rand.Uint32()&0xffff,
		rand.Uint32()&0xffff,
		rand.Uint32()&0xffff,
		rand.Uint32())
}

// GenerateRandomEmail 生成随机邮箱
func GenerateRandomEmail() string {
	rand.Seed(time.Now().UnixNano())
	return fmt.Sprintf("test-%06d@example.com", rand.Intn(1000000))
}

// GenerateVMessInboundData 生成VMess入站测试数据
func GenerateVMessInboundData() map[string]interface{} {
	return map[string]interface{}{
		"enable":         true,
		"remark":         GenerateRandomRemark("e2e-test-vmess"),
		"listen":         "",
		"port":           GenerateRandomPort(),
		"protocol":       "vmess",
		"up":             0,
		"down":           0,
		"total":          0,
		"settings":       fmt.Sprintf(`{"clients": [{"id": "%s", "alterId": 0}], "disableInsecureEncryption": false}`, GenerateRandomUUID()),
		"streamSettings": `{"network": "tcp", "security": "none", "tcpSettings": {}}`,
		"sniffing":       "{}",
	}
}

// GenerateVLESSInboundData 生成VLESS入站测试数据
func GenerateVLESSInboundData() map[string]interface{} {
	return map[string]interface{}{
		"enable":         true,
		"remark":         GenerateRandomRemark("e2e-test-vless"),
		"listen":         "",
		"port":           GenerateRandomPort(),
		"protocol":       "vless",
		"up":             0,
		"down":           0,
		"total":          0,
		"settings":       fmt.Sprintf(`{"clients": [{"id": "%s"}], "decryption": "none"}`, GenerateRandomUUID()),
		"streamSettings": `{"network": "tcp", "security": "none", "tcpSettings": {}}`,
		"sniffing":       "{}",
	}
}

// GenerateClientData 生成客户端测试数据
func GenerateClientData(inboundID int) map[string]interface{} {
	return map[string]interface{}{
		"id": inboundID,
		"settings": fmt.Sprintf(`{
			"clients": [
				{
					"id": "%s",
					"alterId": 0,
					"email": "%s"
				}
			]
		}`, GenerateRandomUUID(), GenerateRandomEmail()),
	}
}