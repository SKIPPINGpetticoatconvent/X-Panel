#!/bin/bash

red='\033[0;31m'
green='\033[0;32m'
blue='\033[0;34m'
yellow='\033[0;33m'
plain='\033[0m'

print_ok() {
  echo -e "${green}$1${plain}"
}

print_warn() {
  echo -e "${yellow}$1${plain}"
}

print_err() {
  echo -e "${red}$1${plain}"
}

print_info() {
  echo -e "$1"
}

require_root() {
  if [[ $EUID -ne 0 ]]; then
    print_err "致命错误:  请使用 root 权限运行此脚本\n"
    exit 1
  fi
  detect_os
  check_os_support
}

detect_os() {
  if [[ -f /etc/os-release ]]; then
    # shellcheck disable=SC1091
    source /etc/os-release
    release=$ID
  elif [[ -f /usr/lib/os-release ]]; then
    # shellcheck disable=SC1091
    source /usr/lib/os-release
    release=$ID
  else
    echo ""
    print_err "检查服务器操作系统失败，请联系作者!"
    exit 1
  fi
  echo ""
  print_ok "---------->>>>>目前服务器的操作系统为: $release"
}

arch() {
  case $(uname -m) in
  x86_64) echo 'amd64' ;;
  aarch64 | arm64 | armv8*) echo 'arm64' ;;
  armv7* | arm) echo 'armv7' ;;
  armv6*) echo 'armv6' ;;
  armv5*) echo 'armv5' ;;
  s390x) echo 's390x' ;;
  *) echo 'unknown' ;;
  esac
}

echo ""
# check_glibc_version() {
#    glibc_version=$(ldd --version | head -n1 | awk '{print $NF}')

#    required_version="2.32"
#    if [[ "$(printf '%s\n' "$required_version" "$glibc_version" | sort -V | head -n1)" != "$required_version" ]]; then
#        echo -e "${red}------>>>GLIBC版本 $glibc_version 太旧了！ 要求2.32或以上版本${plain}"
#        echo -e "${green}-------->>>>请升级到较新版本的操作系统以便获取更高版本的GLIBC${plain}"
#        exit 1
#    fi
#        echo -e "${green}-------->>>>GLIBC版本： $glibc_version（符合高于2.32的要求）${plain}"
# }
# check_glibc_version

print_ok "---------->>>>>当前系统的架构为: $(arch)"
echo ""

get_latest_version() {
  curl -Ls "https://api.github.com/repos/SKIPPINGpetticoatconvent/X-Panel/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/'
}

show_current_version() {
  local xui_version
  xui_version=$(/usr/local/x-ui/x-ui -v)
  if [[ -z $xui_version ]]; then
    echo ""
    print_err "------>>>当前服务器没有安装任何 x-ui 系列代理面板"
    echo ""
    print_ok "-------->>>>片刻之后脚本将会自动引导安装〔X-Panel面板〕"
    return
  fi

  if [[ $xui_version == *:* ]]; then
    print_ok "---------->>>>>当前代理面板的版本为: ${red}其他 x-ui 分支版本${plain}"
    echo ""
    print_ok "-------->>>>片刻之后脚本将会自动引导安装〔X-Panel面板〕"
  else
    print_ok "---------->>>>>当前代理面板的版本为: ${red}〔X-Panel面板〕v${xui_version}${plain}"
  fi
}

last_version=$(get_latest_version)
show_current_version
echo ""
print_warn "---------------------->>>>>〔X-Panel面板〕最新版为：${last_version}"
sleep 4

os_version=$(grep -i version_id /etc/os-release | cut -d \" -f2 | cut -d . -f1)

check_os_support() {
  case "${release}" in
  arch)
    print_info "您的操作系统是 ArchLinux"
    ;;
  manjaro)
    print_info "您的操作系统是 Manjaro"
    ;;
  armbian)
    print_info "您的操作系统是 Armbian"
    ;;
  alpine)
    print_info "您的操作系统是 Alpine Linux"
    ;;
  opensuse-tumbleweed)
    print_info "您的操作系统是 OpenSUSE Tumbleweed"
    ;;
  centos)
    if [[ ${os_version} -lt 8 ]]; then
      print_err " 请使用 CentOS 8 或更高版本 \n"
      exit 1
    fi
    ;;
  ubuntu)
    if [[ ${os_version} -lt 20 ]]; then
      print_err " 请使用 Ubuntu 20 或更高版本!\n"
      exit 1
    fi
    ;;
  fedora)
    if [[ ${os_version} -lt 36 ]]; then
      print_err " 请使用 Fedora 36 或更高版本!\n"
      exit 1
    fi
    ;;
  debian)
    if [[ ${os_version} -lt 11 ]]; then
      print_err " 请使用 Debian 11 或更高版本 \n"
      exit 1
    fi
    ;;
  almalinux)
    if [[ ${os_version} -lt 9 ]]; then
      print_err " 请使用 AlmaLinux 9 或更高版本 \n"
      exit 1
    fi
    ;;
  rocky)
    if [[ ${os_version} -lt 9 ]]; then
      print_err " 请使用 RockyLinux 9 或更高版本 \n"
      exit 1
    fi
    ;;
  oracle)
    if [[ ${os_version} -lt 8 ]]; then
      print_err " 请使用 Oracle Linux 8 或更高版本 \n"
      exit 1
    fi
    ;;
  *)
    print_err "此脚本不支持您的操作系统。\n"
    echo "请确保您使用的是以下受支持的操作系统之一："
    echo "- Ubuntu 20.04+"
    echo "- Debian 11+"
    echo "- CentOS 8+"
    echo "- Fedora 36+"
    echo "- Arch Linux"
    echo "- Manjaro"
    echo "- Armbian"
    echo "- Alpine Linux"
    echo "- AlmaLinux 9+"
    echo "- Rocky Linux 9+"
    echo "- Oracle Linux 8+"
    echo "- OpenSUSE Tumbleweed"
    exit 1
    ;;
  esac
}

# Port helpers
is_port_in_use() {
  local port="$1"
  if command -v ss >/dev/null 2>&1; then
    ss -ltn 2>/dev/null | awk -v p=":${port}$" '$4 ~ p {exit 0} END {exit 1}'
    return
  fi
  if command -v netstat >/dev/null 2>&1; then
    netstat -lnt 2>/dev/null | awk -v p=":${port} " '$4 ~ p {exit 0} END {exit 1}'
    return
  fi
  if command -v lsof >/dev/null 2>&1; then
    lsof -nP -iTCP:"${port}" -sTCP:LISTEN >/dev/null 2>&1 && return 0
  fi
  return 1
}

install_base() {
  case "${release}" in
  ubuntu | debian | armbian)
    apt-get update && apt-get install -y -q wget curl sudo tar tzdata ca-certificates chrony socat
    systemctl enable chrony && systemctl start chrony
    ;;
  centos | rhel | almalinux | rocky | ol)
    yum -y --exclude=kernel* update && yum install -y -q wget curl sudo tar tzdata ca-certificates chrony socat
    systemctl enable chronyd && systemctl start chronyd
    ;;
  fedora | amzn | virtuozzo)
    dnf -y --exclude=kernel* update && dnf install -y -q wget curl sudo tar tzdata ca-certificates chrony socat
    systemctl enable chronyd && systemctl start chronyd
    ;;
  arch | manjaro | parch)
    pacman -Sy && pacman -S --noconfirm wget curl sudo tar tzdata ca-certificates chrony socat
    systemctl enable chronyd && systemctl start chronyd
    ;;
  alpine)
    apk update && apk add --no-cache wget curl sudo tar tzdata ca-certificates chrony socat
    rc-update add chronyd default && rc-service chronyd start
    ;;
  opensuse-tumbleweed)
    zypper refresh && zypper -q install -y wget curl sudo tar timezone ca-certificates chrony socat
    systemctl enable chronyd && systemctl start chronyd
    ;;
  *)
    apt-get update && apt-get install -y -q wget curl sudo tar tzdata ca-certificates chrony socat
    systemctl enable chrony && systemctl start chrony
    ;;
  esac
}

gen_random_string() {
  local length="$1"
  local random_string
  random_string=$(LC_ALL=C tr -dc 'a-zA-Z0-9' </dev/urandom | fold -w "$length" | head -n 1)
  echo "$random_string"
}

# This function will be called when user installed x-ui out of security
install_acme() {
  if command -v ~/.acme.sh/acme.sh &>/dev/null; then
    return 0
  fi
  if ! curl -s https://get.acme.sh | sh; then
    print_err "Install acme.sh failed"
    return 1
  fi
  return 0
}

setup_ip_certificate() {
  local existing_webBasePath
  existing_webBasePath=$(/usr/local/x-ui/x-ui setting -show true | grep -Eo 'webBasePath[^:]*: .+' | awk '{print $2}')
  local existing_port
  existing_port=$(/usr/local/x-ui/x-ui setting -show true | grep -Eo 'port[^:]*: .+' | awk '{print $2}')

  # 获取服务器IP
  local server_ip
  server_ip=$(curl -s4m8 https://api.ipify.org -k)
  if [[ -z ${server_ip} ]]; then
    server_ip=$(curl -s4m8 https://ip.sb -k)
  fi

  if [[ -z ${server_ip} ]]; then
    print_err "无法获取服务器IPv4地址"
    return 1
  fi

  print_ok "检测到服务器IPv4: ${server_ip}"

  # 询问 IPv6
  local ipv6_addr=""
  read -rp "您是否有 IPv6 地址需要包含？(留空跳过): " ipv6_addr
  ipv6_addr="${ipv6_addr// /}"

  # 检查 acme.sh
  if ! command -v ~/.acme.sh/acme.sh &>/dev/null; then
    print_warn "未找到 acme.sh，正在安装..."
    if ! install_acme; then
      print_err "acme.sh 安装失败"
      return 1
    fi
  fi

  # 检查 socat
  if ! command -v socat &>/dev/null; then
    print_warn "未找到 socat，正在安装..."
    case "${release}" in
    ubuntu | debian | armbian)
      apt-get update && apt-get install socat -y
      ;;
    centos | rhel | almalinux | rocky | ol)
      yum -y update && yum -y install socat
      ;;
    fedora | amzn | virtuozzo)
      dnf -y update && dnf -y install socat
      ;;
    arch | manjaro | parch)
      pacman -Sy --noconfirm socat
      ;;
    apk)
      apk add socat
      ;;
    *)
      print_err "不支持的系统，请手动安装 socat。"
      ;;
    esac
  fi

  local certPath="/root/cert/ip"
  mkdir -p "$certPath"

  local domain_args=("-d" "${server_ip}")
  if [[ -n $ipv6_addr ]]; then
    domain_args+=("-d" "${ipv6_addr}")
  fi

  # Choose port for HTTP-01 listener (default 80, prompt override)
  local WebPort=""
  read -rp "请选择用于 ACME HTTP-01 验证的端口 (默认 80): " WebPort
  WebPort="${WebPort:-80}"
  if ! [[ ${WebPort} =~ ^[0-9]+$ ]] || ((WebPort < 1 || WebPort > 65535)); then
    print_err "端口无效，将使用默认端口 80。"
    WebPort=80
  fi
  print_ok "使用端口 ${WebPort} 进行验证。"
  if [[ ${WebPort} -ne 80 ]]; then
    print_warn "提示：Let's Encrypt 仍会连接 80 端口；请确保外部 80 端口转发到了 ${WebPort}。"
  fi

  # Ensure chosen port is available
  while true; do
    if is_port_in_use "${WebPort}"; then
      print_warn "端口 ${WebPort} 被占用。"

      local alt_port=""
      read -rp "请输入其他端口用于 acme.sh 监听 (留空取消): " alt_port
      alt_port="${alt_port// /}"
      if [[ -z ${alt_port} ]]; then
        print_err "端口 ${WebPort} 被占用，无法继续。"
        return 1
      fi
      if ! [[ ${alt_port} =~ ^[0-9]+$ ]] || ((alt_port < 1 || alt_port > 65535)); then
        print_err "无效端口。"
        return 1
      fi
      WebPort="${alt_port}"
      continue
    else
      print_ok "端口 ${WebPort} 可用，准备验证。"
      break
    fi
  done

  print_warn "将使用端口 ${WebPort} 申请证书..."

  ~/.acme.sh/acme.sh --set-default-ca --server letsencrypt
  if ! ~/.acme.sh/acme.sh --issue \
    "${domain_args[@]}" \
    --standalone \
    --server letsencrypt \
    --certificate-profile shortlived \
    --days 6 \
    --httpport "${WebPort}" \
    --force; then
    print_err "证书申请失败，请确保端口 ${WebPort} (或映射到此端口的外部80端口) 已开放"
    return 1
  fi

  local reloadCmd="systemctl restart x-ui 2>/dev/null || rc-service x-ui restart 2>/dev/null || true"

  ~/.acme.sh/acme.sh --installcert -d "${server_ip}" \
    --key-file "${certPath}/privkey.pem" \
    --fullchain-file "${certPath}/fullchain.pem" \
    --reloadcmd "${reloadCmd}" 2>&1 || true

  if [[ ! -f "${certPath}/fullchain.pem" || ! -f "${certPath}/privkey.pem" ]]; then
    print_err "安装后未找到证书文件"
    return 1
  fi

  # 启用自动更新
  ~/.acme.sh/acme.sh --upgrade --auto-upgrade >/dev/null 2>&1
  chmod 600 ${certPath}/privkey.pem 2>/dev/null
  chmod 644 ${certPath}/fullchain.pem 2>/dev/null

  # 配置面板
  local webCertFile="${certPath}/fullchain.pem"
  local webKeyFile="${certPath}/privkey.pem"

  if [[ -f $webCertFile && -f $webKeyFile ]]; then
    /usr/local/x-ui/x-ui cert -webCert "$webCertFile" -webCertKey "$webKeyFile"
    print_ok "面板证书路径已配置"
    print_ok "访问地址: https://${server_ip}:${existing_port}${existing_webBasePath}"
    systemctl restart x-ui
  else
    print_err "配置面板证书失败"
  fi
}

prompt_and_setup_ssl() {
  print_warn "------------------------------------------------"
  print_ok "是否为服务器IP自动申请 SSL 证书? (Let's Encrypt)"
  print_warn "注意：需要占用80端口进行验证"
  read -rp "请输入 [y/n] (默认n): " choice
  if [[ $choice == "y" || $choice == "Y" ]]; then
    setup_ip_certificate
  else
    print_warn "已跳过 SSL 证书设置"
  fi
  print_warn "------------------------------------------------"
}

config_after_install() {
  print_warn "安装/更新完成！ 为了您的面板安全，建议修改面板设置"
  echo ""
  read -rp "$(echo -e "${green}想继续修改吗？${red}选择'n'以保留旧设置${plain} [y/n]? --->>请输入：")" config_confirm
  if [[ ${config_confirm} == "y" || ${config_confirm} == "Y" ]]; then
    read -rp "请设置您的用户名: " config_account
    print_warn "您的用户名将是: ${config_account}"
    read -rp "请设置您的密码: " config_password
    print_warn "您的密码将是: ${config_password}"
    read -rp "请设置面板端口: " config_port
    print_warn "您的面板端口号为: ${config_port}"
    read -rp "请设置面板登录访问路径: " config_webBasePath
    print_warn "您的面板访问路径为: ${config_webBasePath}"
    print_warn "正在初始化，请稍候..."
    /usr/local/x-ui/x-ui setting -username "${config_account}" -password "${config_password}"
    print_warn "用户名和密码设置成功!"
    /usr/local/x-ui/x-ui setting -port "${config_port}"
    print_warn "面板端口号设置成功!"
    /usr/local/x-ui/x-ui setting -webBasePath "${config_webBasePath}"
    print_warn "面板登录访问路径设置成功!"
    echo ""
    prompt_and_setup_ssl
  else
    echo ""
    sleep 1
    print_err "--------------->>>>Cancel...--------------->>>>>>>取消修改..."
    echo ""
    if [[ ! -f "/etc/x-ui/x-ui.db" ]]; then
      local usernameTemp
      usernameTemp=$(head -c 10 /dev/urandom | base64)
      local passwordTemp
      passwordTemp=$(head -c 10 /dev/urandom | base64)
      local webBasePathTemp
      webBasePathTemp=$(gen_random_string 15)
      /usr/local/x-ui/x-ui setting -username "${usernameTemp}" -password "${passwordTemp}" -webBasePath "${webBasePathTemp}"
      echo ""
      print_warn "检测到为全新安装，出于安全考虑将生成随机登录信息:"
      echo -e "###############################################"
      print_ok "用户名: ${usernameTemp}"
      print_ok "密  码: ${passwordTemp}"
      print_ok "访问路径: ${webBasePathTemp}"
      echo -e "###############################################"
      print_ok "如果您忘记了登录信息，可以在安装后通过 x-ui 命令然后输入${red}数字 10 选项${green}进行查看${plain}"
    else
      print_ok "此次操作属于版本升级，保留之前旧设置项，登录方式保持不变"
      echo ""
      print_ok "如果您忘记了登录信息，您可以通过 x-ui 命令然后输入${red}数字 10 选项${green}进行查看${plain}"
      echo ""
      echo ""
    fi
  fi
  sleep 1
  echo -e ">>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>"
  echo ""
  /usr/local/x-ui/x-ui migrate
}

echo ""

ssh_forwarding() {
  # 获取 IPv4 和 IPv6 地址
  local v4
  local v6
  v4=$(curl -s4m8 http://ip.sb -k)
  v6=$(curl -s6m8 http://ip.sb -k)
  local existing_webBasePath
  existing_webBasePath=$(/usr/local/x-ui/x-ui setting -show true | grep -Eo 'webBasePath[^:]*: .+' | awk '{print $2}')
  local existing_port
  existing_port=$(/usr/local/x-ui/x-ui setting -show true | grep -Eo 'port[^:]*: .+' | awk '{print $2}')
  local existing_cert
  existing_cert=$(/usr/local/x-ui/x-ui setting -getCert true | grep -Eo 'cert: .+' | awk '{print $2}')
  local existing_key
  existing_key=$(/usr/local/x-ui/x-ui setting -getCert true | grep -Eo 'key: .+' | awk '{print $2}')

  if [[ -n $existing_cert && -n $existing_key ]]; then
    print_ok "面板已安装证书采用SSL保护"
    echo ""
    existing_cert=$(/usr/local/x-ui/x-ui setting -getCert true | grep -Eo 'cert: .+' | awk '{print $2}')
    domain=$(basename "$(dirname "$existing_cert")")
    print_ok "登录访问面板URL: https://${domain}:${existing_port}${green}${existing_webBasePath}${plain}"
  fi
  echo ""
  if [[ -z $existing_cert && -z $existing_key ]]; then
    print_err "警告：未找到证书和密钥，面板不安全！"
    echo ""
    print_ok "------->>>>请按照下述方法设置〔ssh转发〕<<<<-------"
    echo ""

    # 检查 IP 并输出相应的 SSH 和浏览器访问信息
    if [[ -z $v4 ]]; then
      print_ok "1、本地电脑客户端转发命令：${plain} ${blue}ssh  -L [::]:15208:127.0.0.1:${existing_port}${blue} root@[$v6]${plain}"
      echo ""
      print_ok "2、请通过快捷键【Win + R】调出运行窗口，在里面输入【cmd】打开本地终端服务"
      echo ""
      print_ok "3、请在终端中成功输入服务器的〔root密码〕，注意区分大小写，用以上命令进行转发"
      echo ""
      print_ok "4、请在浏览器地址栏复制${plain} ${blue}[::1]:15208${existing_webBasePath}${plain} ${green}进入〔X-Panel面板〕登录界面"
      echo ""
      print_err "注意：若不使用〔ssh转发〕请为X-Panel面板配置安装证书再行登录管理后台"
    elif [[ -n $v4 && -n $v6 ]]; then
      print_ok "1、本地电脑客户端转发命令：${plain} ${blue}ssh -L 15208:127.0.0.1:${existing_port}${blue} root@$v4${plain} ${yellow}或者 ${blue}ssh  -L [::]:15208:127.0.0.1:${existing_port}${blue} root@[$v6]${plain}"
      echo ""
      print_ok "2、请通过快捷键【Win + R】调出运行窗口，在里面输入【cmd】打开本地终端服务"
      echo ""
      print_ok "3、请在终端中成功输入服务器的〔root密码〕，注意区分大小写，用以上命令进行转发"
      echo ""
      print_ok "4、请在浏览器地址栏复制${plain} ${blue}127.0.0.1:15208${existing_webBasePath}${plain} ${yellow}或者${plain} ${blue}[::1]:15208${existing_webBasePath}${plain} ${green}进入〔X-Panel面板〕登录界面"
      echo ""
      print_err "注意：若不使用〔ssh转发〕请为X-Panel面板配置安装证书再行登录管理后台"
    else
      print_ok "1、本地电脑客户端转发命令：${plain} ${blue}ssh -L 15208:127.0.0.1:${existing_port}${blue} root@$v4${plain}"
      echo ""
      print_ok "2、请通过快捷键【Win + R】调出运行窗口，在里面输入【cmd】打开本地终端服务"
      echo ""
      print_ok "3、请在终端中成功输入服务器的〔root密码〕，注意区分大小写，用以上命令进行转发"
      echo ""
      print_ok "4、请在浏览器地址栏复制${plain} ${blue}127.0.0.1:15208${existing_webBasePath}${plain} ${green}进入〔X-Panel面板〕登录界面"
      echo ""
      print_err "注意：若不使用〔ssh转发〕请为X-Panel面板配置安装证书再行登录管理后台"
      echo ""
    fi
  fi
}

install_x-ui() {
  cd /usr/local/ || exit 1

  # Download resources
  if [[ -z $1 ]]; then
    local last_version
    last_version=$(get_latest_version)
    if [[ -z $last_version ]]; then
      print_err "获取 X-Panel 版本失败，可能是 Github API 限制，请稍后再试"
      exit 1
    fi
    echo ""
    print_info "-----------------------------------------------------"
    print_ok "--------->>获取 X-Panel 最新版本：${yellow}${last_version}${plain}${green}，开始安装...${plain}"
    print_info "-----------------------------------------------------"
    echo ""
    sleep 2
    print_ok "---------------->>>>>>>>>安装进度50%"
    sleep 3
    echo ""
    print_ok "---------------->>>>>>>>>>>>>>>>>>>>>安装进度100%"
    echo ""
    sleep 2
    if ! wget --no-check-certificate -O "/usr/local/x-ui-linux-$(arch).tar.gz" "https://github.com/SKIPPINGpetticoatconvent/X-Panel/releases/latest/download/x-ui-linux-$(arch).tar.gz"; then
      print_err "下载 X-Panel 失败, 请检查服务器是否可以连接至 GitHub？"
      exit 1
    fi
  else
    last_version=$1
    url="https://github.com/SKIPPINGpetticoatconvent/X-Panel/releases/download/${last_version}/x-ui-linux-$(arch).tar.gz"
    echo ""
    echo -e "--------------------------------------------"
    print_ok "---------------->>>>开始安装 X-Panel $1"
    echo -e "--------------------------------------------"
    echo ""
    sleep 2
    echo -e "${green}---------------->>>>>>>>>安装进度50%${plain}"
    sleep 3
    echo ""
    echo -e "${green}---------------->>>>>>>>>>>>>>>>>>>>>安装进度100%${plain}"
    echo ""
    sleep 2
    if ! wget --no-check-certificate -O "/usr/local/x-ui-linux-$(arch).tar.gz" "${url}"; then
      print_err "下载 X-Panel $1 失败, 请检查此版本是否存在"
      exit 1
    fi
  fi
  wget -O /usr/bin/x-ui-temp https://raw.githubusercontent.com/SKIPPINGpetticoatconvent/X-Panel/main/x-ui.sh

  # Stop x-ui service and remove old resources
  if [[ -e /usr/local/x-ui/ ]]; then
    systemctl stop x-ui
    rm /usr/local/x-ui/ -rf
  fi

  sleep 3
  print_ok "------->>>>>>>>>>>检查并保存安装目录"
  echo ""
  tar zxvf "x-ui-linux-$(arch).tar.gz"
  rm "x-ui-linux-$(arch).tar.gz" -f

  cd x-ui || exit 1
  chmod +x x-ui
  chmod +x x-ui.sh

  # Check the system's architecture and rename the file accordingly
  if [[ "$(arch)" == "armv5" || "$(arch)" == "armv6" || "$(arch)" == "armv7" ]]; then
    mv "bin/xray-linux-$(arch)" bin/xray-linux-arm
    chmod +x bin/xray-linux-arm
  else
    chmod +x "bin/xray-linux-$(arch)"
  fi
  chmod +x x-ui

  # Update x-ui cli and se set permission
  mv -f /usr/bin/x-ui-temp /usr/bin/x-ui
  chmod +x /usr/bin/x-ui
  sleep 2
  print_ok "------->>>>>>>>>>>保存成功"
  sleep 2
  echo ""
  config_after_install

  # 执行ssh端口转发
  ssh_forwarding

  cp -f x-ui.service /etc/systemd/system/
  systemctl daemon-reload
  systemctl enable x-ui
  systemctl start x-ui
  systemctl stop warp-go >/dev/null 2>&1
  wg-quick down wgcf >/dev/null 2>&1
  local v4
  local ipv6
  v4=$(curl -s4m8 ip.p3terx.com -k | sed -n 1p)
  ipv6=$(curl -s6m8 ip.p3terx.com -k | sed -n 1p)
  print_warn "检测到服务器IPv4: ${v4}"
  print_warn "检测到服务器IPv6: ${ipv6}"
  systemctl start warp-go >/dev/null 2>&1
  wg-quick up wgcf >/dev/null 2>&1

  echo ""
  print_ok "------->>>>X-Panel ${last_version}<<<<安装成功，正在启动..."
  sleep 1
  echo ""
  echo -e "         ---------------------"
  echo -e "         |${green}X-Panel 控制菜单用法 ${plain}|${plain}"
  echo -e "         |  ${yellow}一个更好的面板   ${plain}|${plain}"
  echo -e "         | ${yellow}基于Xray Core构建 ${plain}|${plain}"
  echo -e "--------------------------------------------"
  echo -e "x-ui              - 进入管理脚本"
  echo -e "x-ui start        - 启动 X-Panel 面板"
  echo -e "x-ui stop         - 关闭 X-Panel 面板"
  echo -e "x-ui restart      - 重启 X-Panel 面板"
  echo -e "x-ui status       - 查看 X-Panel 状态"
  echo -e "x-ui settings     - 查看当前设置信息"
  echo -e "x-ui enable       - 启用 X-Panel 开机启动"
  echo -e "x-ui disable      - 禁用 X-Panel 开机启动"
  echo -e "x-ui log          - 查看 X-Panel 运行日志"
  echo -e "x-ui banlog       - 检查 Fail2ban 禁止日志"
  echo -e "x-ui update       - 更新 X-Panel 面板"
  echo -e "x-ui custom       - 自定义 X-Panel 版本"
  echo -e "x-ui install      - 安装 X-Panel 面板"
  echo -e "x-ui uninstall    - 卸载 X-Panel 面板"
  echo -e "--------------------------------------------"
  echo ""
  # if [[ -n $ipv4 ]]; then
  #    echo -e "${yellow}面板 IPv4 访问地址为：${green}http://$ipv4:${config_port}/${config_webBasePath}${plain}"
  # fi
  # if [[ -n $ipv6 ]]; then
  #    echo -e "${yellow}面板 IPv6 访问地址为：${green}http://[$ipv6]:${config_port}/${config_webBasePath}${plain}"
  # fi
  #    echo -e "请自行确保此端口没有被其他程序占用，${yellow}并且确保${red} ${config_port} ${yellow}端口已放行${plain}"
  sleep 3
  echo -e ">>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>"
  echo ""
  echo -e "${yellow}----->>>X-Panel面板和Xray启动成功<<<-----${plain}"
}

require_root
install_base
install_x-ui "$1"
echo ""
echo -e "----------------------------------------------"
sleep 4
info=$(/usr/local/x-ui/x-ui setting -show true)
echo -e "${info}${plain}"
echo ""
echo -e "若您忘记了上述面板信息，后期可通过x-ui命令进入脚本${red}输入数字〔10〕选项获取${plain}"
echo ""
echo -e "----------------------------------------------"
echo ""
sleep 2
echo -e "${green}安装/更新完成${plain}"
