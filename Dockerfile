# ========================================================
# ----->> Stage: Builder
# ========================================================
FROM golang:1.25-alpine AS builder
WORKDIR /app
ARG TARGETARCH

# 安装构建依赖
RUN apk --no-cache --update add \
    build-base \
    gcc \
    wget \
    unzip \
    git

# 下载 Go 依赖 (利用 Docker 缓存)
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# 下载 x-ui.sh 管理脚本
RUN wget -O x-ui.sh https://raw.githubusercontent.com/SKIPPINGpetticoatconvent/X-Panel/main/x-ui.sh && \
    chmod +x x-ui.sh

# 编译 Go 应用
ENV CGO_ENABLED=1
ENV CGO_CFLAGS="-D_LARGEFILE64_SOURCE"
RUN go build -ldflags "-w -s" -o build/x-ui main.go

# 运行初始化脚本下载 xray-core 等依赖
RUN chmod +x DockerInit.sh && ./DockerInit.sh "$TARGETARCH"

# ========================================================
# ----->> Stage: Final Image of X-Panel
# ========================================================
FROM alpine:latest
ENV TZ=Asia/Shanghai
WORKDIR /app

# 安装运行时依赖
RUN apk add --no-cache --update \
    ca-certificates \
    tzdata \
    fail2ban \
    bash \
    curl

# 从 builder 阶段复制产物
COPY --from=builder /app/build/ /app/
COPY --from=builder /app/DockerEntrypoint.sh /app/
COPY --from=builder /app/x-ui.sh /usr/bin/x-ui

# 配置 fail2ban
RUN rm -f /etc/fail2ban/jail.d/alpine-ssh.conf \
    && cp /etc/fail2ban/jail.conf /etc/fail2ban/jail.local \
    && sed -i "s/^\[ssh\]$/&\nenabled = false/" /etc/fail2ban/jail.local \
    && sed -i "s/^\[sshd\]$/&\nenabled = false/" /etc/fail2ban/jail.local \
    && sed -i "s/#allowipv6 = auto/allowipv6 = auto/g" /etc/fail2ban/fail2ban.conf

# 设置权限
RUN chmod +x \
    /app/DockerEntrypoint.sh \
    /app/x-ui \
    /usr/bin/x-ui

ENV XUI_ENABLE_FAIL2BAN="true"
VOLUME [ "/etc/x-ui" ]
EXPOSE 13688 
CMD [ "./x-ui" ]
ENTRYPOINT [ "/app/DockerEntrypoint.sh" ]
