# X-Panel

[![Stargazers over time](https://starchart.cc/SKIPPINGpetticoatconvent/X-Panel.svg?variant=adaptive)](https://starchart.cc/SKIPPINGpetticoatconvent/X-Panel)
[![Release](https://img.shields.io/github/v/release/SKIPPINGpetticoatconvent/X-Panel.svg?style=flat-square)](https://github.com/SKIPPINGpetticoatconvent/X-Panel/releases)
[![Downloads](https://img.shields.io/github/downloads/SKIPPINGpetticoatconvent/X-Panel/total.svg?style=flat-square)](https://github.com/SKIPPINGpetticoatconvent/X-Panel/releases)
[![License](https://img.shields.io/badge/license-GPL%20V3-blue.svg?style=flat-square)](LICENSE)

An optimized version of [3x-ui](https://github.com/MHSanaei/3x-ui), supporting Xray-core and providing a powerful multi-protocol proxy management panel.

[中文文档](README.zh_CN.md) | [English](README.md)

## 🚀 Quick Start

### System Requirements
- **OS**: Ubuntu 20.04+, Debian 11+, CentOS 8+, Fedora 36+, Arch Linux, Manjaro, Armbian.
- **Architecture**: amd64, arm64, armv7, s390x.
- **Specs**: Minimum 1 Core CPU, 1GB RAM.

### Installation & Upgrade
Run the following command to install or upgrade X-Panel:

```bash
bash <(curl -Ls https://raw.githubusercontent.com/SKIPPINGpetticoatconvent/X-Panel/main/install.sh)
```

To install a specific version:
```bash
VERSION=v25.10.25 bash <(curl -Ls https://raw.githubusercontent.com/SKIPPINGpetticoatconvent/X-Panel/$VERSION/install.sh) $VERSION
```

### Accessing the Panel
After installation, the script will display your login details.
- **Default Port**: `2053` (or randomized)
- **Default URL**: `http://YOUR_IP:PORT/YOUR_PATH/panel`
- **Security Recommendation**: It is highly recommended to enable HTTPS (SSL) or use SSH tunneling for access.

## ✨ Features

| Feature | Description |
|---------|-------------|
| **Multi-Protocol** | Support for VMess, VLESS, Trojan, Shadowsocks, WireGuard, Dokodemo-door, Socks, HTTP. |
| **XTLS & Reality** | Full support for Vision flow control, Reality, and RPRX-Direct. |
| **Traffic Management** | Real-time traffic monitoring, **automatic traffic reset**, device limit (anti-sharing), single-port multi-user. |
| **Speed Limit & Auditing** | Independent speed limits (KB/s) per inbound/account, flexible auditing rules. |
| **Quick Config & SNI** | Panel/Telegram Bot quick node generation, **Smart SNI Selection** (Geographic awareness). |
| **Telegram Integration** | Notifications for login/traffic/expiration, Bot commands for management (restart/backup/status). |
| **Subscription** | Support for Clash, Surge, V2Ray formats with customizable templates. |
| **Backup & Restore** | Automatic daily backups to Telegram, manual import/export from the panel. |

## 💻 CLI Usage (`x-ui`)

Manage the panel via the `x-ui` command:

| Command | Description |
|---------|-------------|
| `x-ui` | Open the interactive management menu |
| `x-ui start` | Start the panel service |
| `x-ui stop` | Stop the panel service |
| `x-ui restart` | Restart the panel |
| `x-ui status` | Check service status |
| `x-ui settings` | View current settings (port, path, etc.) |
| `x-ui enable` | Enable auto-start on boot |
| `x-ui log` | View logs |
| `x-ui banlog` | View Fail2Ban logs |
| `x-ui ssl` | Manage SSL certificates (ACME) |

## 🐳 Docker Installation

1. **Install Docker**:
   ```bash
   curl -fsSL https://get.docker.com | bash
   ```

2. **Run X-Panel**:
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

   *Note: HOST networking is recommended for performance and port management.*

## 🛠️ Development

### Prerequisites
- Go 1.22+
- Node.js 18+ (yarn recommended)

### Build Steps
1. **Frontend**:
   ```bash
   cd web
   npm install && npm run build
   ```
2. **Backend**:
   ```bash
   go mod tidy
   go build -o x-ui main.go
   ```

## ⚠️ Safe Use Policy
This project is for educational and technical research purposes only. Users are responsible for complying with local laws and regulations.
The authors are not responsible for any misuse of this software.

## 🙏 Credits
- [MHSanaei/3x-ui](https://github.com/MHSanaei/3x-ui)
- [FranzKafkaYu/x-ui](https://github.com/FranzKafkaYu/x-ui)
- [vaxilu/x-ui](https://github.com/vaxilu/x-ui)
