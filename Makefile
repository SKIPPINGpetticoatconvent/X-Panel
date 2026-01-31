.PHONY: build test e2e clean

# 项目基础信息
BINARY_NAME=x-ui
VERSION_FILE=config/version
VERSION=$(shell cat $(VERSION_FILE))
LDFLAGS=-ldflags "-s -w -X 'github.com/SKIPPINGpetticoatconvent/X-Panel/config.version=$(VERSION)'"

# 默认目标
all: build

# 编译 Go 二进制文件
build:
	@echo "Building $(BINARY_NAME) v$(VERSION)..."
	go build $(LDFLAGS) -o $(BINARY_NAME) main.go
	@echo "Build success: ./$(BINARY_NAME)"

# 运行单元测试
test:
	@echo "Running unit tests..."
	go test ./...

# 运行 E2E 测试 (Local Mode)
# 自动编译并模拟安装验证
e2e:
	@echo "Running E2E Installation Test (Local Mode)..."
	chmod +x tests/e2e/runner.sh
	./tests/e2e/runner.sh --mode local

# 运行 E2E 测试 (Online Mode)
# 从 GitHub 下载真实 release 进行验证
e2e-online:
	@echo "Running E2E Installation Test (Online Mode)..."
	chmod +x tests/e2e/runner.sh
	./tests/e2e/runner.sh --mode online

# 清理构建产物
clean:
	@echo "Cleaning up..."
	rm -f $(BINARY_NAME)
	rm -f x-ui-linux-amd64.tar.gz
	docker rm -f xpanel-e2e-test >/dev/null 2>&1 || true
