.PHONY: build test e2e clean all hooks changelog lint fmt

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

# 运行 E2E 测试
e2e:
	@echo "Running E2E tests (Standard)..."
	go test -v -timeout 15m ./tests/e2e/...

# 运行特定 E2E 安装测试
e2e-install:
	@echo "Running E2E Installation Test..."
	go test -v ./tests/e2e/... -run TestE2E/TestInstallation

# 清理构建产物
clean:
	@echo "Cleaning up..."
	rm -f $(BINARY_NAME)
	rm -f x-ui-linux-amd64.tar.gz
	docker rm -f xpanel-e2e-test >/dev/null 2>&1 || true

# 安装 git hooks
hooks:
	@echo "Installing git hooks..."
	git config core.hooksPath .githooks
	@echo "✅ Git hooks installed. Using .githooks directory."

# 生成 CHANGELOG
changelog:
	@echo "Generating CHANGELOG..."
	@if command -v git-cliff >/dev/null 2>&1; then \
		git-cliff -o CHANGELOG.md; \
		echo "✅ CHANGELOG.md generated."; \
	else \
		echo "❌ git-cliff not installed. Install with: cargo install git-cliff"; \
		exit 1; \
	fi

# 运行 lint 检查
lint:
	@echo "Running linters..."
	golangci-lint run --timeout=10m
	@echo "✅ Lint passed."

# 格式化代码
fmt:
	@echo "Formatting code..."
	gofmt -w .
	@echo "✅ Code formatted."
