package e2e

import (
	"path/filepath"
	"time"
)

func (s *E2ETestSuite) TestInstallation() {
	// 1. 注入核心文件
	s.T().Log("Copying install scripts and artifacts to container...")

	err := s.container.CopyFileToContainer(s.ctx, filepath.Join(s.projectRoot, "install.sh"), "/root/install.sh", 0o755)
	s.Require().NoError(err)

	err = s.container.CopyFileToContainer(s.ctx, filepath.Join(s.projectRoot, "x-ui-linux-amd64.tar.gz"), "/root/x-ui-linux-amd64.tar.gz", 0o644)
	s.Require().NoError(err)

	// 2. 模拟本地发布源 (Patch install.sh)
	s.T().Log("Setting up mock release server...")
	_, _, err = s.execCommand([]string{"mkdir", "-p", "/root/mock_server/releases/download/v1.0.0"})
	s.Require().NoError(err)
	_, _, err = s.execCommand([]string{"cp", "/root/x-ui-linux-amd64.tar.gz", "/root/mock_server/releases/download/v1.0.0/x-ui-linux-amd64.tar.gz"})
	s.Require().NoError(err)

	// 启动本地 mock 服务 (Python)
	go func() {
		s.execCommand([]string{"python3", "-m", "http.server", "8080", "--directory", "/root/mock_server"})
	}()
	time.Sleep(2 * time.Second)

	s.T().Log("Patching install.sh for local installation...")
	// 修改下载链接指向本地
	s.execCommand([]string{"sed", "-i", "s|https://github.com/SKIPPINGpetticoatconvent/X-Panel/releases/download|http://127.0.0.1:8080/releases/download|g", "/root/install.sh"})
	// 锁定版本为 v1.0.0 避免在线检查
	s.execCommand([]string{"sed", "-i", "s|last_version=$(curl.*|last_version=\"v1.0.0\"|g", "/root/install.sh"})

	// 3. 执行安装
	s.T().Log("Running install.sh...")
	// 输入默认值: IPv6(n), SSL(n), Settings(n)
	exitCode, output, err := s.execCommand([]string{"bash", "-c", "printf '\\nn\\nn\\n' | /root/install.sh v1.0.0"})
	s.T().Logf("Install Output: %s", output)
	s.Require().Equal(0, exitCode, "Installation failed")

	// 4. 验证服务状态
	s.T().Log("Verifying service and process...")
	// 给进程一点启动时间
	time.Sleep(5 * time.Second)

	exitCode, _, _ = s.execCommand([]string{"systemctl", "is-active", "--quiet", "x-ui"})
	s.Equal(0, exitCode, "x-ui service is not active")

	exitCode, _, _ = s.execCommand([]string{"pgrep", "x-ui"})
	s.Equal(0, exitCode, "x-ui process not found")

	// 灵活匹配 xray 进程名，因为在不同版本中可能带有架构后缀
	exitCode, _, _ = s.execCommand([]string{"bash", "-c", "pgrep xray || pgrep xray-linux"})
	s.Equal(0, exitCode, "xray process not found")

	s.T().Log("Installation E2E Test Passed Successfully!")
}
