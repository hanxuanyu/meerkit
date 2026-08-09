# Meerkit TCP 插件

[English](README.en.md) · [插件总览](../README.md)

官方 `meerkit.tcp` 插件提供 `tcp` 监控模块，用于建立 TCP 连接，并可选择发送一段数据和读取一次服务端响应。

## 当前能力

- 使用主机名或 IP 与端口建立 TCP 连接。
- 配置 1 到 300 秒的连接超时。
- 连接后发送 UTF-8 文本，或先把输入按 Base64 解码再发送二进制数据。
- 可选择只探测连接，或读取一次响应。
- 单独配置 1 到 60 秒的读取超时和最多 1 MiB 的读取上限。
- 返回远端地址、连接耗时、响应文本、Base64 二进制、响应哈希和字节数。

此插件不实现 TLS 握手、持续会话、协议级重试或多轮收发。需要 HTTP/TLS 响应语义时应使用 HTTP 插件；需要特定应用协议时应实现独立监控插件。

## 配置参数

| 参数 | 默认值 | 说明 |
| --- | --- | --- |
| `host` | 必填 | 主机名或 IP |
| `port` | 必填 | 1 到 65535 |
| `timeout_seconds` | `10` | 连接超时 |
| `send` | 空 | 连接后发送的内容；留空时只连接 |
| `send_base64` | `false` | 将 `send` 按 Base64 解码后发送 |
| `read_response` | `false` | 发送后读取一次响应 |
| `read_timeout_seconds` | `3` | 读取超时，仅读取响应时生效 |
| `max_read_bytes` | `65536` | 最大读取字节数，范围 1 到 1048576 |

## 结果

结果集键为 `connection`：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `success` | boolean | 本次执行是否成功 |
| `connected` | boolean | TCP 连接是否建立 |
| `duration_ms` | number | 建立连接所需毫秒数 |
| `remote_addr` | string | 已连接的远端地址 |
| `response_text` | text | 响应的文本表示 |
| `response_bytes` | binary | Base64 编码的原始响应 |
| `response_hash` | string | 原始响应内容哈希 |
| `bytes_read` | number | 实际读取字节数 |

上述字段可以用于连接失败、耗时、响应内容变化或字节数阈值条件。

## 开发

```bash
cd plugins/tcp
go test ./...

cd ../..
go run . serve
```

源码清单为 [`meerkit-plugin.yaml`](meerkit-plugin.yaml)，实现位于 [`monitor/tcp.go`](monitor/tcp.go)。打包命令见 [`scripts/README.md`](../../scripts/README.md)。

## 许可证

本插件随 Meerkit 以 [Apache License 2.0](../../LICENSE) 开源。
