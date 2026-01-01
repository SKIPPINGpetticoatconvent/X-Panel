package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func main() {
	// 1. 设置路径：目标是 web/service/tgbot 文件夹
	baseDir := filepath.Join("web", "service")
	targetSubDir := filepath.Join(baseDir, "tgbot")
	inputPath := filepath.Join(baseDir, "tgbot.go")

	// 创建子文件夹
	if err := os.MkdirAll(targetSubDir, 0755); err != nil {
		fmt.Printf("❌ 创建目录失败: %v\n", err)
		return
	}

	// 2. 读取原始大文件
	contentByte, err := os.ReadFile(inputPath)
	if err != nil {
		fmt.Printf("❌ 找不到源文件: %v\n", inputPath)
		return
	}
	content := string(contentByte)

	// 3. 提取 Import 块
	re := regexp.MustCompile(`(?s)import \(.*?\)`)
	importBlock := re.FindString(content)

	lines := strings.Split(content, "\n")

	// 4. 定义文件映射 (注意：这里 package 改为 tgbot)
	filesData := map[string][]string{
		"tgbot_core.go":     {"package tgbot\n", importBlock, "\n"},
		"tgbot_cmds.go":     {"package tgbot\n", importBlock, "\n"},
		"tgbot_callback.go": {"package tgbot\n", importBlock, "\n"},
		"tgbot_utils.go":    {"package tgbot\n", importBlock, "\n"},
	}

	var currentFile = "tgbot_core.go"

	// 5. 分拣逻辑
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "package ") || strings.HasPrefix(trimmed, "import ") {
			continue
		}

		if strings.HasPrefix(trimmed, "func (t *Tgbot) answerCommand") ||
			strings.HasPrefix(trimmed, "func (t *Tgbot) OnReceive") {
			currentFile = "tgbot_cmds.go"
		} else if strings.HasPrefix(trimmed, "func (t *Tgbot) answerCallback") {
			currentFile = "tgbot_callback.go"
		} else if strings.HasPrefix(trimmed, "func (t *Tgbot) random") ||
			strings.HasPrefix(trimmed, "func (t *Tgbot) SetHostname") {
			currentFile = "tgbot_utils.go"
		}

		filesData[currentFile] = append(filesData[currentFile], line)
	}

	// 6. 写入到 web/service/tgbot/ 目录下
	for fileName, fileLines := range filesData {
		finalPath := filepath.Join(targetSubDir, fileName)
		err := os.WriteFile(finalPath, []byte(strings.Join(fileLines, "\n")), 0644)
		if err != nil {
			fmt.Printf("❌ 写入失败 %s: %v\n", finalPath, err)
		} else {
			fmt.Printf("✅ 已成功保存到: %s\n", finalPath)
		}
	}

	// 7. 备份原文件
	backupPath := inputPath + ".bak"
	os.Rename(inputPath, backupPath)

	fmt.Println("\n✨ 任务完成！所有文件已移至 web/service/tgbot 文件夹。")
}
