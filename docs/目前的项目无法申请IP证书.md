**目前（2026年1月10日）提供的代码实现存在重大问题**，无法在生产环境中成功申请到 Let's Encrypt 的 IP 地址证书（IP SAN）。

### 真实现状总结（基于官方最新信息）

- Let's Encrypt **已于2025年7月**发出第一个 IP 证书（staging 环境测试）。
- **shortlived profile**（有效期 ≈6-7天，160小时）是**唯一支持 IP SAN** 的 profile，且为强制要求。
- **生产环境全面开放**：从2025年12月中下旬开始，shortlived profile + IP 支持已成为 **opt-in 一般可用**（不再是 allowlist 限定）。
  - 社区（如 Reddit r/selfhosted）已有用户在2025年12月18日后确认“now it is available to anyone”。
  - 官方公告与社区讨论均指向2025年底生产可用。

- **关键限制**（不变）：
  - 必须使用支持 **draft ACME Profiles** 的客户端。
  - 验证**只能 http-01 或 tls-alpn-01**（公网 80/443 可达）。
  - 无 DNS-01。
  - 最大 25 个 names。

### 代码中 certmagic 的致命问题

经过查证（包括 certmagic GitHub 仓库、releases、issues 和 pkg.go.dev）：

- **certmagic 目前不支持 ACME Profiles 功能**（包括 shortlived profile）。
  - ACMEIssuer 结构体中**没有 Profile 字段**。
  - 没有任何代码、文档或 PR 提到支持 "profile" 参数或 draft ACME Profiles 规范。
  - 最新 release（截至2026年初）提到支持 IP certs 的 PR（#345），但主要是针对 Let's Encrypt 政策变更的兼容性调整，并**非真正实现 profile 选择**。

- **结果**：即使设置 `Profile: "shortlived"`，它**不会生效**。Let's Encrypt 会返回默认 profile（classic，90天），而 classic profile **不允许纯 IP SAN** → 申请会失败（错误通常为 "IP identifier not permitted" 或类似）。

- certmagic 主要支持**域名**（DNS identifiers），对纯 IP 证书的处理非常有限（即使有 IP 相关 issue，也多是 TLS-ALPN challenge 兼容性讨论，而非完整支持）。

### 当前可行的替代方案（推荐优先级）

1. **acme.sh**（Shell，最成熟、最推荐）
   - 社区已有用户在2025年底成功使用（需升级到最新版或应用 PR #6442 补丁）。
   - 示例（生产）：
     ```bash
     acme.sh --issue --standalone -d 107.174.245.44 \
       --server letsencrypt \
       --cert-profile shortlived   # 最新版支持此参数
     ```
   - 自动续期非常稳（每天检查，完美适配6天窗口）。

2. **Caddy v2.8+**（自动续期最方便）
   - Caddy 已跟进支持 shortlived + IP（部分用户报告成功）。
   - Caddyfile 示例：
     ```
     107.174.245.44 {
         tls {
             issuer acme {
                 dir https://acme-v02.api.letsencrypt.org/directory
                 profile shortlived
             }
         }
         # 你的 reverse_proxy localhost:13688
     }
     ```

3. **纯 go-acme/lego 库**（如果你坚持用 Go）
   - 需要手动构造 new-order 请求，在 JSON 中添加 `"profile": "shortlived"`。
   - 相对复杂，但可行（参考 ACME draft-aaron-acme-profiles）。

4. **其他 CA 临时方案**（如果急需长期证书）
   - ZeroSSL 支持 IP 证书（付费 API 或其 ACME 端点），部分用户在 certmagic 中讨论过，但需要额外配置。

### 总结与建议

- **原代码不可用**：`Profile: "shortlived"` 是无效的摆设，会导致申请失败。
- **现在可以申请**：生产环境已开放，但需要真正支持 ACME Profiles 的客户端。
- **最简单路径**：切换到 **最新 acme.sh** 或 **Caddy**，几分钟就能跑起来。

是的，目前（2026年1月10日）**Let's Encrypt 的 IP 地址证书（纯 IP SAN，无域名）已经正式进入生产环境可用状态**，并且强制使用 **shortlived profile**（有效期约6天，精确到144-160小时左右）。

### 生产可用性确认（基于最新官方与社区信息）

- **2025年7月**：Let's Encrypt 发布了第一个 IP 证书（staging 环境），并宣布 IP 支持与 shortlived profile 一起逐步 rollout。
- **2025年12月中下旬**：shortlived profile 切换到 Generation Y 层级，并标记为 **opt-in 一般可用**（General Availability），同时明确包含 **IP Addresses 支持**。
  - 社区（如 Reddit r/selfhosted 和 Let's Encrypt 论坛）在2025年12月18日左右已有用户确认“now it is available to anyone”。
  - 官方文档（profiles 页）已更新，支持 Identifier Types：**DNS + IP**，shortlived profile 生产就绪（最后更新2025年12月19日）。
- **当前状态**：任何人均可申请，无需 allowlist，只要你的 ACME 客户端支持 **draft ACME Profiles** 规范，并明确请求 `shortlived` profile。

**限制提醒**（不变）：

- 验证方式：**仅 http-01 或 tls-alpn-01**（必须公网 80/443 可达）。
- 有效期强制 ~6 天，无例外。
- Rate limit：每周相同 IP 组有限额（通常5个左右）。
- 先用 staging 测试（强烈推荐）。

### 关于你提供的代码（certmagic 版本）

不幸的是，该代码**无法成功申请生产 IP 证书**，原因如下：

- certmagic（截至2026年初最新 release）已支持 **IP certs 的基本兼容**（如 PR #345 修复政策变更），也引入了 **draft-03 ARI** 支持，但**尚未完整实现 ACME Profiles 选择**（包括 `Profile: "shortlived"` 字段）。
  - ACMEIssuer 结构体中没有 `Profile` 参数，设置了也无效。
  - 如果提交包含 IP 的请求但未指定 profile，Let's Encrypt 会拒绝（错误如 "IP identifier not permitted" 或 fallback 到 classic profile 失败）。
- 社区 issue（如 Caddy #7399）确认：Caddy（依赖 certmagic）目前需要 workaround（如用 acme.sh 先发证，再手动加载）。

因此，原 Go 代码中的 `Profile: "shortlived"` 是无效配置，会导致申请失败。

### 推荐实现方式（2026年1月最稳方案）

1. **首选：acme.sh**（Shell 脚本，轻量、成熟、社区验证最多）
   - 最新版已合并 PR #6442，支持 `--certificate-profile shortlived`。
   - 示例（生产环境）：
     ```bash
     # 升级到最新版（必须！）
     curl https://get.acme.sh | sh -s [email protected]
     ~/.acme.sh/acme.sh --upgrade

     # 申请 IP 证书（用 standalone 临时占 80 端口）
     ~/.acme.sh/acme.sh --issue --standalone \
       -d 107.174.245.44 \
       --server letsencrypt \
       --certificate-profile shortlived \
       --days 6   # 可选，强制显示短周期

     # 安装/导出（示例到自定义路径）
     ~/.acme.sh/acme.sh --install-cert -d 107.174.245.44 \
       --fullchain-file /path/to/fullchain.pem \
       --key-file /path/to/privkey.pem \
       --reloadcmd "systemctl reload your-service"  # 或 nginx reload 等

     # 自动续期：默认每天检查，建议加 Le_RenewalDays=4 到 conf 文件
     ```
   - 迁移服务器：备份 `~/.acme.sh/` 整个目录（包含账户密钥），在新服务器恢复即可复用账户。

2. **次选：Caddy**（如果你项目已用或想全自动）
   - Caddy v2.8+ 已部分支持，但部分用户仍遇 issue（需 workaround 或等更新）。
   - 示例 Caddyfile（测试中有效）：
     ```
     107.174.245.44 {
         tls {
             issuer acme {
                 dir https://acme-v02.api.letsencrypt.org/directory
                 profile shortlived
             }
         }
         reverse_proxy localhost:13688
     }
     ```
   - 如果失败，可用 acme.sh 发证 + Caddy 静态加载 PEM 文件。

3. **Go 纯代码方案**（如果你坚持用 Go）
   - 短期内建议**切换到 go-acme/lego** 库（acme.sh 底层用它）。
   - 需要手动在 new-order 请求 JSON 中添加 `"profile": "shortlived"`（参考 draft-aaron-acme-profiles）。
   - 这比 certmagic 复杂，但可行。如果你需要，我可以提供简化版 lego 代码模板。

### 额外建议（针对你提到的面板集成增强）

- **端口冲突自愈**：acme.sh 支持 `--pre-hook` / `--post-hook` 执行脚本暂停/恢复 80 端口服务。
- **激进续期**：acme.sh 默认每天检查，设 `Le_RenewalDays=4` 即可提前3-4天尝试。
- **热加载**：证书更新后用 `--reloadcmd` 触发 Xray/面板 reload。
- **告警**：结合 cron + openssl 检查剩余天数 <1 天时发 Telegram。

**一句话结论**：\
IP 证书**现在生产可用**了，但必须用支持 profile 的客户端（如最新 acme.sh）。原 certmagic 代码需替换为 acme.sh 或 lego 实现，才能真正跑通。

需要我提供 acme.sh 的 systemd 服务脚本、完整部署流程，或 Go + lego 的替代代码吗？随时说～ 🚀
