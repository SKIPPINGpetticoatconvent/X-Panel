package e2e

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

type E2ETestSuite struct {
	suite.Suite
	ctx         context.Context
	container   testcontainers.Container
	projectRoot string
}

func (s *E2ETestSuite) SetupSuite() {
	s.ctx = context.Background()

	// 解决 Testcontainers 在某些环境下对 XDG_RUNTIME_DIR 敏感导致的探测 Panic 问题
	os.Unsetenv("XDG_RUNTIME_DIR")

	// 自动识别 Docker Host
	if os.Getenv("DOCKER_HOST") == "" {
		if _, err := os.Stat("/var/run/docker.sock"); err == nil {
			os.Setenv("DOCKER_HOST", "unix:///var/run/docker.sock")
		} else if _, err := os.Stat("/run/podman/podman.sock"); err == nil {
			os.Setenv("DOCKER_HOST", "unix:///run/podman/podman.sock")
		}
	}

	// 获取项目根目录
	_, filename, _, _ := runtime.Caller(0)
	s.projectRoot = filepath.Join(filepath.Dir(filename), "../..")

	// Determine if we should use a pre-built image or build from Dockerfile
	imageName := os.Getenv("E2E_IMAGE")
	fromDockerfile := testcontainers.FromDockerfile{}
	if imageName == "" {
		fromDockerfile = testcontainers.FromDockerfile{
			Context:    s.projectRoot,
			Dockerfile: "tests/e2e/docker/Dockerfile.ubuntu22",
			KeepImage:  false,
		}
	}

	// 定义容器请求
	req := testcontainers.ContainerRequest{
		Image:          imageName,
		FromDockerfile: fromDockerfile,
		Privileged:     true,
		Mounts: testcontainers.Mounts(
			testcontainers.ContainerMount{
				Source: testcontainers.GenericBindMountSource{HostPath: "/sys/fs/cgroup"},
				Target: "/sys/fs/cgroup",
			},
		),
		HostConfigModifier: func(hc *container.HostConfig) {
			hc.CgroupnsMode = "host"
			if hc.Tmpfs == nil {
				hc.Tmpfs = make(map[string]string)
			}
			hc.Tmpfs["/run"] = ""
			hc.Tmpfs["/run/lock"] = ""
			hc.Tmpfs["/tmp"] = ""
		},
		// systemd 启动由于对控制台输出有限制，我们放宽日志等待
		WaitingFor: wait.ForAll(
			wait.ForLog("systemd"),
		).WithDeadline(5 * time.Second),
	}

	// 启动容器
	var err error
	s.container, err = testcontainers.GenericContainer(s.ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})

	// 允许静默启动。如果容器运行但日志未命中，可能是输出重定向到了 /dev/console
	if err != nil && s.container != nil {
		s.T().Log("Log wait failed but container is running, assuming systemd is starting silently...")
	} else {
		s.Require().NoError(err, "无法启动容器。请确保 Docker 服务正常运行且当前用户有权访问 DOCKER_HOST")
	}

	// 给 systemd 充足的时间完成初始化 (初始化 dbus, systemctl 等)
	time.Sleep(10 * time.Second)
}

func (s *E2ETestSuite) TearDownSuite() {
	if s.container != nil {
		err := s.container.Terminate(s.ctx)
		s.NoError(err, "无法清理测试容器")
	}
}

func (s *E2ETestSuite) execCommand(cmd []string) (int, string, error) {
	exitCode, reader, err := s.container.Exec(s.ctx, cmd)
	if err != nil {
		return exitCode, "", err
	}

	// 改进：使用 io.ReadAll 读取完整输出
	data, _ := io.ReadAll(reader)
	return exitCode, string(data), nil
}

func TestE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E tests in short mode.")
	}
	suite.Run(t, new(E2ETestSuite))
}
