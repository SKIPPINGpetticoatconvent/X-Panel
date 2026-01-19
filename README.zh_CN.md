# X-Panel 面板

[![Star Chart](https://starchart.cc/SKIPPINGpetticoatconvent/X-Panel.svg)](https://starchart.cc/SKIPPINGpetticoatconvent/X-Panel)
[![Release](https://img.shields.io/github/v/release/SKIPPINGpetticoatconvent/X-Panel.svg?style=flat-square)](https://github.com/SKIPPINGpetticoatconvent/X-Panel/releases)
[![Downloads](https://img.shields.io/github/downloads/SKIPPINGpetticoatconvent/X-Panel/total.svg?style=flat-square)](https://github.com/SKIPPINGpetticoatconvent/X-Panel/releases)
[![License](https://img.shields.io/badge/license-GPL%20V3-blue.svg?style=flat-square)](LICENSE)

基于 [3x-ui](https://github.com/MHSanaei/3x-ui) 优化的 Xray 面板，支持多协议管理、流量统计及高级路由功能。

[English](README.md) | [中文文档](README.zh_CN.md)

## 🚀 快速开始

### 系统要求 (Recommended)
- **操作系统**: Ubuntu 20.04+, Debian 11+, CentOS 8+, Fedora 36+, Arch Linux, Manjaro, Armbian.
- **架构**: amd64, arm64, armv7, s390x.
- **配置**: 建议最低 1核 CPU, 1GB 内存.

### 一键安装 & 升级
使用 root 用户运行以下命令进行安装或升级：

```bash
bash <(curl -Ls https://raw.githubusercontent.com/SKIPPINGpetticoatconvent/X-Panel/main/install.sh)
```

如需安装指定版本：
```bash
VERSION=v25.10.25 bash <(curl -Ls https://raw.githubusercontent.com/SKIPPINGpetticoatconvent/X-Panel/$VERSION/install.sh) $VERSION
```

### 访问面板
安装完成后，脚本将输出登录详情。
- **默认端口**: `2053` (或安装时随机生成)
- **默认地址**: `http://你的IP:端口/你的路径/panel`
- **安全建议**: 强烈建议配置 HTTPS (SSL) 证书或使用 SSH 隧道进行访问，**避免使用 HTTP 明文裸连**。

## ✨ 核心功能
| 功能模块 | 详细说明 |
|----------|----------|
| **多协议支持** | 完整支持 VMess, VLESS, Trojan, Shadowsocks, WireGuard, Dokodemo-door, Socks, HTTP 协议。 |
| **XTLS & Reality** | 深度集成 XTLS-Vision 流控与 Reality 协议，支持 RPRX-Direct，提供更强的抗探测能力。 |
| **流量管理** | 支持实时流量监控、**到期自动重置流量**、**限制设备并发数** (IP Limit) 以防止账号滥用。 |
| **限速与审计** | 支持针对每个入站或账号设置独立的上传/下载限速 (KB/s)，支持灵活的路由审计规则。 |
| **便捷配置** | 面板及 Telegram 机器人支持“快速配置生成”，集成 **智能 SNI 优选** (自动选择低阻断的 SNI 域名)。 |
| **Telegram 集成** | 机器人支持节点查询、流量提醒、登录通知、系统状态监控、数据库自动备份。 |
| **订阅与转换** | 内置订阅管理，支持生成适配 Clash, Surge, V2Ray 等客户端的订阅链接。 |

## 💻 命令行管理 (CLI)

安装后，可直接在终端使用 `x-ui` 命令管理面板：

| 命令 | 说明 |
|------|------|
| `x-ui` | 打开交互式管理菜单 (推荐) |
| `x-ui start` | 启动面板服务 |
| `x-ui stop` | 停止面板服务 |
| `x-ui restart` | 重启面板 |
| `x-ui status` | 查看服务运行状态 |
| `x-ui settings` | 查看当前配置 (端口/路径/账号信息) |
| `x-ui enable` | 设置开机自启 |
| `x-ui log` | 查看面板运行日志 |
| `x-ui ssl` | SSL 证书管理 (ACME) |

## 🐳 Docker 部署

如果您偏好使用容器化部署：

1. **安装 Docker**:
   ```bash
   curl -fsSL https://get.docker.com | bash
   ```

2. **启动 X-Panel 容器**:
   ```bash
   docker run -itd \
     -e XRAY_VMESS_AEAD_FORCED=false \
     -v $PWD/db/:/etc/x-ui/ \
     -v $PWD/cert/:/root/cert/ \
     --network=host \
     --restart=unless-stopped \
     --name x-panel \
     ghcr.io/xeefei/x-panel:latest
   ```
   > **注意**: 推荐使用 `host` 网络模式以简化端口映射管理。

## 📖 进阶配置指南

### 1. Reality 回落 (Dest) 与 SNI 设置
在配置 VLESS + Reality 时：
- **Dest (目标网站)**: 建议指向本机 80 端口或其他未被屏蔽的国外大站（需支持 TLSv1.3）。
- **SNI (服务器名称指示)**: 填写与 Dest 匹配的域名。
- **回落 (Fallback)**: 可配置回落至 Nginx 或其他 Web 服务，实现伪装站点的访问。

### 2. Telegram 机器人配置
- 在 `@BotFather` 创建机器人获取 Token。
- 获取你的 Chat ID (可通过 `@userinfobot` 获取)。
- 在面板 `设置` -> `Telegram` 中填入 Token 和 Chat ID。
- 启用您需要的功能：登录提醒、流量预警、每日报表等。

## ⚠️ 免责声明
本项目仅供网络技术研究与学习交流使用。
- 用户在使用本项目时必须遵守当地法律法规。
- 作者不对因使用本项目而产生的任何后果负责。
- 请勿将本项目用于非法用途。

## 🙏 致谢
- [MHSanaei/3x-ui](https://github.com/MHSanaei/3x-ui)
- [FranzKafkaYu/x-ui](https://github.com/FranzKafkaYu/x-ui)
- [vaxilu/x-ui](https://github.com/vaxilu/x-ui)
