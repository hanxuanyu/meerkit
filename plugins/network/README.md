# Meerkit Network

官方 `meerkit.network` 插件集中提供五个独立网络监控模块。每个模块拥有自己的配置、结果集和条件字段，但共享同一个经过签名和管理的插件进程。

| 模块类型 | 用途 |
| --- | --- |
| `http` | HTTP/HTTPS 请求、响应状态、响应头、正文与内容哈希 |
| `tcp` | TCP 端口连接、可选数据发送和单次响应读取 |
| `dns` | 指定 DNS 服务器上的 A、AAAA、CNAME、MX、TXT、NS、SRV、CAA 和 PTR 查询 |
| `tls-certificate` | 直接 TLS 握手、协商参数、证书链、SAN、指纹和到期时间 |
| `icmp` | IPv4/IPv6 ICMP Echo 连通性、丢包率、RTT 和抖动 |

## DNS

DNS 模块要求查询名称和 DNS 服务器，支持 UDP、TCP 和自动模式。自动模式先使用 UDP，收到截断响应时以 TCP 重新查询。PTR 查询输入 IP 地址时会自动转换为 `in-addr.arpa` 或 `ip6.arpa` 名称。

结果集 `query` 包含 RCODE、响应标志、Answer/Authority/Additional 数量、最小 TTL、规范化记录、值数组和耗时。`NXDOMAIN`、`SERVFAIL` 等是合法 DNS 响应，模块会保存其 RCODE；连接、超时或解码失败才作为执行错误。

## TLS 证书

TLS 模块连接任意直接 TLS 端口。`host` 决定 TCP 目标，`server_name` 决定 SNI 和证书名称校验，因此支持连接 IP、校验域名。可附加企业私有根 CA PEM。

模块先以不阻断证书验证的方式完成握手并取得证书，再独立计算 `hostname_valid`、`chain_valid`、`expired`、`not_yet_valid` 和总体 `valid`。即使验证失败，也会返回证书 Subject、Issuer、SAN、到期时间、剩余天数、指纹和证书链摘要。`verify_certificate=true` 时验证失败会让本次执行失败；关闭后只记录验证结果。

第一版只支持直接 TLS，不处理 SMTP STARTTLS、PostgreSQL SSLRequest 等应用层协议升级。

## ICMP

ICMP 模块支持自动、IPv4 和 IPv6 地址选择，并输出发包/收包数量、丢包率、最小/平均/最大 RTT、相邻样本平均差值形式的抖动和原始 RTT 样本。

Socket 模式：

- `auto`：优先使用非特权 `udp4/udp6` Ping Socket，失败后尝试 Raw Socket。
- `unprivileged`：只使用非特权模式。
- `raw`：只使用 Raw Socket。

Linux 非特权模式受 `net.ipv4.ping_group_range` 控制；容器中的 Raw Socket 通常需要 `CAP_NET_RAW`。权限只在执行 ICMP 模块时检查，不影响插件健康和其他四个模块。

## 构建与测试

```bash
cd plugins/network
go test ./...
go build ./...
```

从仓库根目录执行黑盒协议检查：

```bash
go build -o /tmp/meerkit-network-plugin ./plugins/network
go run ./cmd/plugincheck \
  --manifest ./plugins/network/meerkit-plugin.yaml \
  --artifact /tmp/meerkit-network-plugin
```

插件只执行探测并返回结构化 Observation。阈值和期望值应在 Meerkit 条件中配置，例如 DNS RCODE 不为 `NOERROR`、证书剩余不足 30 天或 ICMP 丢包率大于 20%。
