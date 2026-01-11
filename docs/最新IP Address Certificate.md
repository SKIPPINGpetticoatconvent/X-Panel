# 第一步：安装 / 升级 acme.sh
acme.sh 是著名的自动化证书签发程序，支持 Let’s Encrypt、ZeroSSL 等不同的证书提供商。

```bash
curl https://get.acme.sh | sh -s email=my@example.com
```

如果安装过，那么升级方式：`./acme.sh upgrade`

第二步：签发证书
第一种：独立方式
独立方式是服务器中本身没有 Web 服务，acme.sh 会自己运行一个 Web 服务来进行验证：


```bash
./acme.sh --issue --server letsencrypt -d 64.23.194.105 --certificate-profile shortlived --days 3 --standalone
```
命令具体解析如下：

>./acme.sh：执行 acme.sh 脚本。
>--issue：申请一个新证书。
>--server letsencrypt：使用 Let’s Encrypt 服务器。
> -d 64.23.194.105：证书申请的目标是 IP 地址 64.23.194.105（该IP用于测试，已被删除）
>--certificate-profile shortlived：申请一个短期证书（最长90天？）。
>--days 3：证书的有效期是 3 天。
>--standalone：使用 standalone 模式验证，不依赖现有的 Web 服务器。（需要80/443端口