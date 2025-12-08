package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println("开始运行一键配置菜单测试...")
	fmt.Println()

	// 调用一键配置菜单测试
	RunOneClickMenuTest()

	fmt.Println()
	fmt.Println("测试执行完成")
	os.Exit(0)
}
