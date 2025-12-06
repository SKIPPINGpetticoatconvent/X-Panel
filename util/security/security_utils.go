package security

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// ValidatePort 验证端口号是否在有效范围内
func ValidatePort(portStr string) (int, error) {
	// 检查是否为数字
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return 0, fmt.Errorf("端口号必须是数字: %s", portStr)
	}
	
	// 检查端口范围 (1-65535)
	if port < 1 || port > 65535 {
		return 0, fmt.Errorf("端口号必须在 1-65535 范围内: %d", port)
	}
	
	return port, nil
}

// ValidateLevel 验证日志级别参数
func ValidateLevel(level string) error {
	validLevels := []string{"emerg", "alert", "crit", "error", "warning", "notice", "info", "debug"}
	level = strings.ToLower(strings.TrimSpace(level))
	
	for _, valid := range validLevels {
		if level == valid {
			return nil
		}
	}
	
	return fmt.Errorf("无效的日志级别 '%s'，有效级别: %v", level, validLevels)
}

// ValidateDomain 验证域名格式
func ValidateDomain(domain string) error {
	domain = strings.TrimSpace(domain)
	
	if domain == "" {
		return fmt.Errorf("域名不能为空")
	}
	
	// 基本的域名格式验证
	domainRegex := regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?)*$`)
	if !domainRegex.MatchString(domain) {
		return fmt.Errorf("域名格式无效: %s", domain)
	}
	
	// 检查是否包含危险字符
	dangerousChars := []string{";", "&", "|", "$", "`", "\\", "\"", "'", "\n", "\r"}
	for _, char := range dangerousChars {
		if strings.Contains(domain, char) {
			return fmt.Errorf("域名包含危险字符: %s", char)
		}
	}
	
	return nil
}

// ValidateFilePath 验证文件路径安全性
func ValidateFilePath(filePath string) error {
	filePath = strings.TrimSpace(filePath)
	
	if filePath == "" {
		return fmt.Errorf("文件路径不能为空")
	}
	
	// 检查路径遍历攻击
	if strings.Contains(filePath, "..") {
		return fmt.Errorf("文件路径不能包含 '..' 目录遍历序列")
	}
	
	// 检查绝对路径
	if strings.HasPrefix(filePath, "/") {
		return fmt.Errorf("仅允许相对路径")
	}
	
	// 检查危险字符
	dangerousChars := []string{";", "&", "|", "$", "`", "\\", "\"", "'", "\n", "\r"}
	for _, char := range dangerousChars {
		if strings.Contains(filePath, char) {
			return fmt.Errorf("文件路径包含危险字符: %s", char)
		}
	}
	
	return nil
}

// ValidateScriptPath 验证脚本路径安全性
func ValidateScriptPath(scriptPath string) error {
	scriptPath = strings.TrimSpace(scriptPath)
	
	if scriptPath == "" {
		return fmt.Errorf("脚本路径不能为空")
	}
	
	// 仅允许绝对路径且在特定目录下
	if !strings.HasPrefix(scriptPath, "/usr/bin/") && !strings.HasPrefix(scriptPath, "/usr/local/bin/") {
		return fmt.Errorf("脚本路径必须在 /usr/bin/ 或 /usr/local/bin/ 目录下")
	}
	
	// 检查路径遍历攻击
	if strings.Contains(scriptPath, "..") {
		return fmt.Errorf("脚本路径不能包含 '..' 目录遍历序列")
	}
	
	// 检查危险字符
	dangerousChars := []string{";", "&", "|", "$", "`", "\\", "\"", "'", "\n", "\r"}
	for _, char := range dangerousChars {
		if strings.Contains(scriptPath, char) {
			return fmt.Errorf("脚本路径包含危险字符: %s", char)
		}
	}
	
	return nil
}

// ValidateCommandArgs 验证命令参数安全性
func ValidateCommandArgs(args []string) error {
	for i, arg := range args {
		arg = strings.TrimSpace(arg)
		
		if arg == "" {
			return fmt.Errorf("第 %d 个参数不能为空", i+1)
		}
		
		// 检查危险字符
		dangerousChars := []string{";", "&", "|", "$", "`", "\\", "\"", "'", "\n", "\r", "`"}
		for _, char := range dangerousChars {
			if strings.Contains(arg, char) {
				return fmt.Errorf("第 %d 个参数包含危险字符 '%s': %s", i+1, char, arg)
			}
		}
	}
	
	return nil
}

// SanitizeFilename 清理文件名
func SanitizeFilename(filename string) string {
	// 移除危险字符
	dangerousChars := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|"}
	for _, char := range dangerousChars {
		filename = strings.ReplaceAll(filename, char, "_")
	}
	
	// 限制长度
	if len(filename) > 255 {
		filename = filename[:255]
	}
	
	return filename
}