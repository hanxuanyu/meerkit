# Meerkit HTTP 插件

[English](README.en.md) · [插件总览](../README.md)

官方 `meerkit.http` 插件提供 `http` 监控模块，用于发送 HTTP/HTTPS 请求并把响应状态、耗时、响应头、正文和内容哈希作为结构化结果返回。

## 当前能力

- 方法：`GET`、`HEAD`、`POST`、`PUT`、`PATCH`、`DELETE`、`OPTIONS`、`TRACE`、`CONNECT`。
- 查询参数和请求头。
- URL 编码表单、Multipart 表单、Raw JSON 与 Raw 文本请求体。
- 可选 HTTP/HTTPS 代理。
- 可配置请求超时、最大响应体、重定向开关和最大重定向次数。
- 默认校验 TLS 证书，也可在监控配置中关闭校验。
- 响应按 `auto`、`text` 或 `json` 解析，并可保留原文、去除首尾空白或规范化 JSON。
- 为文本与 JSON 内容生成稳定哈希，记录正文是否因大小上限被截断。

## 配置参数

| 参数 | 默认值 | 说明 |
| --- | --- | --- |
| `url` | 必填 | `http://` 或 `https://` 请求地址 |
| `method` | `GET` | 请求方法 |
| `timeout_seconds` | `30` | 1 到 300 秒 |
| `proxy_url` | 空 | 可选 HTTP/HTTPS 代理 |
| `response_mode` | `auto` | 自动、文本或 JSON |
| `normalize` | `trim` | 原文、去除首尾空白或规范化 JSON |
| `max_body_bytes` | `1048576` | 1 KiB 到 1 MiB |
| `query` | 空 | 查询参数映射 |
| `headers` | 空 | 请求头映射 |
| `body_mode` | `none` | 无、URL 编码表单、Multipart、Raw JSON 或 Raw 文本 |
| `follow_redirects` | `true` | 是否跟随 3xx |
| `verify_tls` | `true` | 是否校验服务端证书 |
| `max_redirects` | `10` | 1 到 50，仅跟随重定向时生效 |

请求体相关字段会根据方法和 `body_mode` 动态显示。Raw JSON 在执行前必须是合法 JSON。

## 结果

结果集键为 `response`：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `success` | boolean | 请求和响应读取是否成功 |
| `status_code` | string | HTTP 状态码，如 `200` |
| `duration_ms` | number | 请求耗时 |
| `response_headers` | map | 响应头 |
| `body_text` | text | 文本响应 |
| `body_json` | json | 解析后的 JSON，支持路径选择 |
| `body_hash` | string | 规范化内容的哈希 |
| `body_size` | number | 已读取响应字节数 |
| `truncated` | boolean | 是否达到最大响应体限制 |

模块只负责观察响应，不会把非 2xx 状态自动视为执行错误。需要告警时，请在 Meerkit 条件编辑器中对 `status_code`、正文、耗时或其他字段配置规则。

## 开发

```bash
cd plugins/http
go test ./...

# 从仓库根目录启动开发宿主，自动构建并加载本插件
cd ../..
go run . serve
```

源码清单为 [`meerkit-plugin.yaml`](meerkit-plugin.yaml)，实现位于 [`monitor/http.go`](monitor/http.go)。打包命令见 [`scripts/README.md`](../../scripts/README.md)。

## 许可证

本插件随 Meerkit 以 [Apache License 2.0](../../LICENSE) 开源。
