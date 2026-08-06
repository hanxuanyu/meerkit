# Meerkit HTTP 监控插件

通过 Meerkit 公共插件 SDK 提供 `http` 监控模块，用于请求 HTTP/HTTPS 服务、采集结构化响应，并基于状态码、耗时、正文或 JSON 字段建立触发条件。

## 主要能力

- 支持 `GET`、`HEAD`、`POST`、`PUT`、`PATCH`、`DELETE`、`OPTIONS`、`TRACE` 和 `CONNECT`。
- 支持查询参数、自定义请求头、HTTP/HTTPS 代理和可配置超时。
- 支持 URL 编码表单、Multipart 表单、Raw JSON 和 Raw 文本请求体。
- 支持重定向次数限制、TLS 证书校验和最大响应体限制。
- 可自动识别 JSON 响应，也可以强制按文本或 JSON 处理。
- 生成响应正文 SHA-256，适合只关注内容变化而不保存完整比较值的场景。
- JSON 结果支持使用 `data.status` 形式的路径选择嵌套字段。

模块类型为 `http`，模块版本为 `2`，配置版本和结果结构版本均为 `1`。

## 配置参数

### 请求目标

- `url`：必填，完整的 `http://` 或 `https://` URL。监控项列表使用该值作为内容摘要。
- `method`：请求方法，默认 `GET`。
- `timeout_seconds`：整个请求的超时时间，默认 `30` 秒，范围 `1-300`。
- `proxy_url`：可选 HTTP/HTTPS 代理地址，例如 `http://127.0.0.1:7890`。

### 查询参数与请求头

- `query`：键值形式的查询参数。一个键需要多个值时，可以在界面中输入多个值。
- `headers`：按原样发送的请求头，可用于 `Accept`、`Authorization`、`Cookie` 等字段。

URL 中原有的查询参数会被保留，再与 `query` 中配置的参数合并。

### 请求体

只有 `POST`、`PUT`、`PATCH`、`DELETE` 和 `OPTIONS` 会显示请求体配置。

- `body_mode`：`none`、`form_urlencoded`、`multipart_form`、`raw_json` 或 `raw_text`，默认 `none`。
- `form_fields`：表单模式下发送的字段。
- `json_body`：Raw JSON 模式下发送的 JSON 值，保存前会校验语法。
- `raw_body`：Raw 文本模式下发送的原始内容。

插件会为表单和 Raw 请求体自动设置合适的 `Content-Type`；显式配置的请求头优先。

### 响应处理

- `response_mode`：默认 `auto`。`auto` 根据响应 `Content-Type` 判断是否解析 JSON；`text` 只保留文本；`json` 尝试解析 JSON。
- `normalize`：正文哈希和文本结果的规范化方式。`raw` 保留原文，`trim` 去除首尾空白，`json` 将合法 JSON 规范化后再计算结果。
- `max_body_bytes`：最多读取并保存的响应体大小，默认 `262144` 字节，范围 `1024-1048576`。超出后设置 `truncated=true`。
- `follow_redirects`：是否跟随 3xx 响应，默认启用。
- `max_redirects`：最大重定向次数，默认 `10`，范围 `1-50`。
- `verify_tls`：是否验证 HTTPS 服务端证书，默认启用。关闭会降低连接安全性，只应在明确了解风险时使用。

## 返回结果

结果集 `response` 表示本次 HTTP 响应，包含：

- `success`：请求是否完成。该字段表示网络请求和响应读取是否成功，不代表状态码一定是 2xx。
- `status_code`：HTTP 状态码。
- `duration_ms`：请求耗时，单位毫秒。
- `response_headers`：响应头的键值集合。
- `body_text`：按 `normalize` 处理后的响应文本。
- `body_json`：解析成功后的 JSON 值；未解析或解析失败时不存在。
- `body_hash`：规范化正文的 SHA-256。
- `body_size`：实际保留的响应字节数。
- `truncated`：响应是否因 `max_body_bytes` 限制而截断。

Meerkit 还会为所有插件结果补充公共执行摘要结果集。插件详情弹窗中显示的是插件声明的输入参数和结果字段，执行详情则显示本次真实采集值。

## 条件示例

- 服务可用：`当前结果 · HTTP 响应 · 状态码 等于 200`。
- 延迟告警：`当前结果 · HTTP 响应 · 响应耗时 大于 1000`。
- JSON 业务状态：选择 `响应 JSON`，路径填写 `data.status`，再与固定值比较。
- 内容变化：`上次执行 · HTTP 响应 · 内容哈希 不等于 当前`。
- 正文匹配：使用 `包含`、`不包含` 或正则表达式比较 `响应文本`。

## 执行日志与隐私

插件会记录请求开始、请求失败、响应读取失败和响应处理完成，并包含方法、去除查询参数后的目标地址、状态码、耗时、响应大小、截断状态和内容哈希。日志不会记录请求头、请求体、查询参数值或响应正文。

日志由宿主配置：

```yaml
plugins:
  log_level: info
  log_format: simple
```

插件日志保存在 `${storage.data_dir}/plugins/logs`，也可以在插件管理页面实时查看。

## 开发与测试

在仓库根目录执行：

```sh
go run .
```

`dev` 宿主会自动构建并加载 `plugins/http` 源码，构建产物只写入根目录的 `data/plugins`。修改插件后重新启动即可验证，无需先打包和导入。

单独运行插件测试：

```sh
cd plugins/http
go test ./...
```

生成当前平台的可导入插件包：

```sh
scripts/package-plugins.sh --plugin ./plugins/http
```

发布签名、跨平台目标和信任模型参见上级目录的 `plugins/README.md`。
