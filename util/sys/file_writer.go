package sys

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// BackupFile 创建文件的备份，备份文件名格式为 filePath.bak.timestamp
func BackupFile(filePath string) error {
	// 检查源文件是否存在
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return fmt.Errorf("源文件不存在: %s", filePath)
	}

	// 生成备份文件名
	timestamp := time.Now().Unix()
	backupPath := fmt.Sprintf("%s.bak.%d", filePath, timestamp)

	// 复制文件
	//nolint:gosec
	srcFile, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("打开源文件失败: %v", err)
	}
	defer func() { _ = srcFile.Close() }()
	//nolint:gosec
	dstFile, err := os.Create(backupPath)
	if err != nil {
		return fmt.Errorf("创建备份文件失败: %v", err)
	}
	defer func() { _ = dstFile.Close() }()

	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return fmt.Errorf("复制文件内容失败: %v", err)
	}

	// 确保内容写入磁盘
	err = dstFile.Sync()
	if err != nil {
		return fmt.Errorf("同步备份文件失败: %v", err)
	}

	return nil
}

// AtomicWriteFile 使用临时文件和原子重命名的方式安全地写入文件
func AtomicWriteFile(filePath string, content []byte, perm os.FileMode) error {
	// 确保目标目录存在
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("创建目录失败: %v", err)
	}

	// 创建临时文件
	tempFile, err := os.CreateTemp(dir, "atomic_write_*")
	if err != nil {
		return fmt.Errorf("创建临时文件失败: %v", err)
	}
	tempPath := tempFile.Name()
	defer func() { _ = os.Remove(tempPath) }() // 无论成功失败都清理临时文件

	// 写入内容
	if _, err := tempFile.Write(content); err != nil {
		_ = tempFile.Close()
		return fmt.Errorf("写入临时文件失败: %v", err)
	}

	// 确保内容写入磁盘
	if err := tempFile.Sync(); err != nil {
		_ = tempFile.Close()
		return fmt.Errorf("同步临时文件失败: %v", err)
	}

	// 设置正确的权限
	if err := tempFile.Chmod(perm); err != nil {
		_ = tempFile.Close()
		return fmt.Errorf("设置文件权限失败: %v", err)
	}

	// 关闭临时文件
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("关闭临时文件失败: %v", err)
	}

	// 原子重命名
	if err := os.Rename(tempPath, filePath); err != nil {
		return fmt.Errorf("原子重命名失败: %v", err)
	}

	return nil
}

// WriteConfigBlock 写入配置块到文件
// 如果文件中存在以 startMarker 开始和 endMarker 结束的块，则替换该块的内容
// 如果不存在，则在文件末尾追加该块
func WriteConfigBlock(filePath, startMarker, endMarker, content string) error {
	// 读取现有文件内容
	var existingContent string
	//nolint:gosec
	if file, err := os.Open(filePath); err == nil {
		data, readErr := io.ReadAll(file)
		_ = file.Close()
		if readErr != nil {
			return fmt.Errorf("读取现有文件失败: %v", readErr)
		}
		existingContent = string(data)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("打开文件失败: %v", err)
	}
	// 如果文件不存在，existingContent 为空

	lines := strings.Split(existingContent, "\n")
	newLines := []string{}

	blockFound := false
	inBlock := false

	// 查找现有的配置块
	for _, line := range lines {
		if strings.TrimSpace(line) == startMarker {
			if inBlock {
				// 嵌套块，跳过
				continue
			}
			inBlock = true
			blockFound = true
			continue
		}

		if strings.TrimSpace(line) == endMarker {
			if inBlock {
				inBlock = false
				// 替换块内容
				newLines = append(newLines, startMarker)
				contentLines := strings.Split(content, "\n")
				newLines = append(newLines, contentLines...)
				newLines = append(newLines, endMarker)
				continue
			}
		}

		if !inBlock {
			newLines = append(newLines, line)
		}
	}

	// 如果没有找到块，追加到文件末尾
	if !blockFound {
		if len(newLines) > 0 && newLines[len(newLines)-1] != "" {
			newLines = append(newLines, "") // 添加空行
		}
		newLines = append(newLines, startMarker)
		contentLines := strings.Split(content, "\n")
		newLines = append(newLines, contentLines...)
		newLines = append(newLines, endMarker)
	}

	// 重新组合内容
	newContent := strings.Join(newLines, "\n")

	// 获取现有文件的权限，如果不存在则使用默认权限
	var perm os.FileMode = 0o644
	if info, err := os.Stat(filePath); err == nil {
		perm = info.Mode().Perm()
	}

	// 使用原子写入保存文件
	return AtomicWriteFile(filePath, []byte(newContent), perm)
}
