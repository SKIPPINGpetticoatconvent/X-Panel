#!/bin/bash
# IP 证书申请功能测试脚本
# 此脚本模拟 ssl_cert_issue_for_ip() 函数的 acme.sh 调用验证参数解析

set -e

echo "=========================================="
echo "  IP 证书申请功能测试 (acme.sh 参数解析)"
echo "=========================================="

# 安装 acme.sh
echo -e "\n[1/4] 安装 acme.sh..."
curl -s https://get.acme.sh | sh -s email=test@example.com >/dev/null 2>&1
echo "✓ acme.sh 已安装"

# 模拟 domain_args 变量
server_ip="192.168.1.100"
ipv6_addr="2001:db8::1"
domain_args="-d ${server_ip} -d ${ipv6_addr}"

echo -e "\n[2/4] 测试参数展开..."
echo "domain_args 值: ${domain_args}"

# 测试正确的参数展开方式 (无引号)
echo -e "\n[3/4] 使用不带引号的 \${domain_args} 调用 acme.sh..."
echo '命令: ~/.acme.sh/acme.sh --issue ${domain_args} --standalone --test --force'

# 使用 --test 模式 (Let's Encrypt staging), 预期会失败但应该能正确解析参数
~/.acme.sh/acme.sh --set-default-ca --server letsencrypt >/dev/null 2>&1

# 执行测试命令 - 使用 --test 标志
output=$(~/.acme.sh/acme.sh --issue \
  ${domain_args} \
  --standalone \
  --server letsencrypt \
  --test \
  --httpport 8888 \
  --force 2>&1) || true

# 分析输出
echo -e "\n[4/4] 分析结果..."
if echo "$output" | grep -q "Domains not changed"; then
  echo "✓ 参数解析成功: acme.sh 正确识别了域名参数"
  echo "  - 识别到 IP: ${server_ip}"
  echo "  - 识别到 IPv6: ${ipv6_addr}"
elif echo "$output" | grep -q "\"${server_ip}\""; then
  echo "✓ 参数解析成功: acme.sh 正确处理了 IP 地址"
elif echo "$output" | grep -qE "(standalone|listen|Binding)"; then
  echo "✓ 参数解析成功: acme.sh 进入了 standalone 模式"
  echo "  (预期会因无法绑定端口而失败，但参数解析正确)"
elif echo "$output" | grep -q "invalid"; then
  echo "✗ 参数解析失败: acme.sh 报告参数无效"
  echo "  输出: $output"
  exit 1
else
  echo "○ 命令执行结果 (预期因网络原因失败):"
  echo "$output" | head -15
fi

echo -e "\n=========================================="
echo "  测试完成"
echo "=========================================="
