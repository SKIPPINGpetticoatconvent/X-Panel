# ----------------------------------------
# 第一阶段：构建 (Build Stage)
# ----------------------------------------
FROM golang:1.24-alpine AS builder
WORKDIR /app
# 安装构建依赖
RUN apk add --no-cache build-base gcc git
# 复制源码
COPY . .
# 编译应用
ENV CGO_ENABLED=1
RUN go build -ldflags "-w -s" -o x-ui main.go

# ----------------------------------------
# 第二阶段：运行 (Runtime Stage)
# ----------------------------------------
FROM alpine:latest
WORKDIR /app
ENV TZ=Asia/Shanghai

# 安装运行时依赖 + 测试工具 (sqlite)
# 🔴 关键点：这里直接装好了 sqlite，不用在运行时下载
RUN apk add --no-cache ca-certificates tzdata fail2ban bash curl sqlite

# 从构建阶段复制文件
COPY --from=builder /app/x-ui /app/
COPY --from=builder /app/bin /app/bin/ 2>/dev/null || true

# 复制原本的资源文件 (根据你的项目结构，确保这些存在)
# 如果你的项目里有 x-ui.sh 或其他资源，保留它们
# COPY x-ui.sh /usr/bin/x-ui

# 🔴 关键点：复制我们的测试启动脚本
COPY tests/e2e/entrypoint.sh /app/test_entrypoint.sh
RUN chmod +x /app/test_entrypoint.sh

# 设置环境变量
ENV XPANEL_RUN_IN_CONTAINER=true
ENV XUI_ENABLE_FAIL2BAN=false

# 暴露端口
EXPOSE 13688

# 设置数据卷
VOLUME [ "/etc/x-ui" ]

# 🔴 关键点：使用测试脚本作为入口
ENTRYPOINT ["/app/test_entrypoint.sh"]