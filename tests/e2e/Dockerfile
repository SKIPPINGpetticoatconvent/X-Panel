# ----------------------------------------
# 第一阶段：构建 (Build Stage)
# ----------------------------------------
FROM golang:alpine AS builder

WORKDIR /app
# 安装构建依赖
RUN apk add --no-cache build-base gcc git wget unzip

# 复制源码
COPY . .

# 运行初始化脚本 (下载 geoip.dat / geosite.dat)
ARG TARGETARCH
RUN if [ -f DockerInit.sh ]; then \
      chmod +x DockerInit.sh && ./DockerInit.sh "$TARGETARCH"; \
    fi

# 编译应用
ENV CGO_ENABLED=1
RUN go build -ldflags "-w -s" -o x-ui main.go

# 🟢 关键修改：创建一个统一的输出目录，整理所有文件
# 这样可以避免 Runtime 阶段 COPY 失败
RUN mkdir -p /output/bin

# 1. 复制主程序
RUN cp x-ui /output/

# 2. 复制 .dat 文件到 /output/bin (如果存在)
RUN if ls *.dat 1> /dev/null 2>&1; then cp *.dat /output/bin/; fi

# 3. 复制 bin 目录下的内容 (如果存在)
RUN if [ -d bin ]; then cp -r bin/* /output/bin/; fi

# 4. 复制 x-ui.sh (如果存在)
RUN if [ -f x-ui.sh ]; then cp x-ui.sh /output/; fi

# ----------------------------------------
# 第二阶段：运行 (Runtime Stage)
# ----------------------------------------
FROM alpine:latest
WORKDIR /app
ENV TZ=Asia/Shanghai

# 安装运行时依赖 + 测试工具 (sqlite)
RUN apk add --no-cache ca-certificates tzdata fail2ban bash curl sqlite

# 🟢 关键修改：从 builder 的输出目录一次性复制所有文件
# 这样保证文件结构正确，且不会因为缺少某个文件报错
COPY --from=builder /output/ /app/

# 复制测试启动脚本
COPY tests/e2e/entrypoint.sh /app/test_entrypoint.sh
RUN chmod +x /app/test_entrypoint.sh

# 环境变量
ENV XPANEL_RUN_IN_CONTAINER=true
ENV XUI_ENABLE_FAIL2BAN=false

# 端口与卷
EXPOSE 13688
VOLUME [ "/etc/x-ui" ]

# 入口
ENTRYPOINT ["/app/test_entrypoint.sh"]