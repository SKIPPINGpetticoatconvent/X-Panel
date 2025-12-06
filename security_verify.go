package main

import (
	"fmt"
	"os"

	"x-ui/util/security"
)

func main() {
	fmt.Println("=== X-Panel 命令注入安全漏洞修复验证测试 ===")

	// 测试用例
	testCases := []struct {
		name        string
		testFunc    func() error
		expectError bool
	}{
		{
			name: "端口号验证测试",
			testFunc: func() error {
				// 有效端口
				if _, err := security.ValidatePort("8080"); err != nil {
					return fmt.Errorf("有效端口验证失败: %v", err)
				}
				// 无效端口 - 超范围
				if _, err := security.ValidatePort("99999"); err == nil {
					return fmt.Errorf("无效端口应该被拒绝")
				}
				// 无效端口 - 非数字
				if _, err := security.ValidatePort("abc"); err == nil {
					return fmt.Errorf("非数字端口应该被拒绝")
				}
				return nil
			},
			expectError: false,
		},
		{
			name: "日志级别验证测试",
			testFunc: func() error {
				// 有效级别
				if err := security.ValidateLevel("info"); err != nil {
					return fmt.Errorf("有效级别验证失败: %v", err)
				}
				// 无效级别
				if err := security.ValidateLevel("invalid_level"); err == nil {
					return fmt.Errorf("无效级别应该被拒绝")
				}
				return nil
			},
			expectError: false,
		},
		{
			name: "域名验证测试",
			testFunc: func() error {
				// 有效域名
				if err := security.ValidateDomain("example.com"); err != nil {
					return fmt.Errorf("有效域名验证失败: %v", err)
				}
				// 包含危险字符的域名
				if err := security.ValidateDomain("example.com;rm -rf /"); err == nil {
					return fmt.Errorf("包含危险字符的域名应该被拒绝")
				}
				return nil
			},
			expectError: false,
		},
		{
			name: "文件路径验证测试",
			testFunc: func() error {
				// 有效相对路径
				if err := security.ValidateFilePath("config.json"); err != nil {
					return fmt.Errorf("有效路径验证失败: %v", err)
				}
				// 路径遍历攻击
				if err := security.ValidateFilePath("../../etc/passwd"); err == nil {
					return fmt.Errorf("路径遍历攻击应该被拒绝")
				}
				// 绝对路径
				if err := security.ValidateFilePath("/etc/passwd"); err == nil {
					return fmt.Errorf("绝对路径应该被拒绝")
				}
				return nil
			},
			expectError: false,
		},
		{
			name: "脚本路径验证测试",
			testFunc: func() error {
				// 有效脚本路径
				if err := security.ValidateScriptPath("/usr/bin/x-ui"); err != nil {
					return fmt.Errorf("有效脚本路径验证失败: %v", err)
				}
				// 无效路径
				if err := security.ValidateScriptPath("/tmp/malicious.sh"); err == nil {
					return fmt.Errorf("无效脚本路径应该被拒绝")
				}
				return nil
			},
			expectError: false,
		},
		{
			name: "命令参数验证测试",
			testFunc: func() error {
				// 有效参数
				if err := security.ValidateCommandArgs([]string{"journalctl", "-u", "x-ui", "-n", "100"}); err != nil {
					return fmt.Errorf("有效参数验证失败: %v", err)
				}
				// 包含危险字符的参数
				if err := security.ValidateCommandArgs([]string{"rm", "-rf", "/tmp; ls"}); err == nil {
					return fmt.Errorf("包含危险字符的参数应该被拒绝")
				}
				return nil
			},
			expectError: false,
		},
	}

	// 执行测试
	passed := 0
	failed := 0

	for _, tc := range testCases {
		fmt.Printf("\n--- 测试: %s ---\n", tc.name)
		
		err := tc.testFunc()
		
		if tc.expectError && err != nil {
			fmt.Printf("✅ PASS: %s\n", tc.name)
			passed++
		} else if !tc.expectError && err == nil {
			fmt.Printf("✅ PASS: %s\n", tc.name)
			passed++
		} else if tc.expectError && err == nil {
			fmt.Printf("❌ FAIL: %s - 期望错误但未得到错误\n", tc.name)
			failed++
		} else {
			fmt.Printf("❌ FAIL: %s - %v\n", tc.name, err)
			failed++
		}
	}

	// 输出测试结果
	fmt.Println("\n=== 测试结果 ===")
	fmt.Printf("通过: %d\n", passed)
	fmt.Printf("失败: %d\n", failed)
	fmt.Printf("总计: %d\n", passed+failed)

	if failed == 0 {
		fmt.Println("🎉 所有安全验证测试通过！命令注入漏洞修复成功！")
		os.Exit(0)
	} else {
		fmt.Println("❌ 部分测试失败，需要进一步检查修复")
		os.Exit(1)
	}
}