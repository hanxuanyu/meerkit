package httpmonitor

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/hanxuanyu/meerkit/sdk"
)

const (
	defaultTimeoutSeconds = 30
	defaultMaxBodyBytes   = 262144
	defaultMaxRedirects   = 10
)

var supportedMethods = []string{"GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "OPTIONS", "TRACE", "CONNECT"}

var supportedBodyModes = []string{"none", "form_urlencoded", "multipart_form", "raw_json", "raw_text"}

var bodyMethods = []string{"POST", "PUT", "PATCH", "DELETE", "OPTIONS"}

type Module struct {
	Client *http.Client
}

func New() *Module { return &Module{} }

func (m *Module) Descriptor() sdk.ModuleDescriptor {
	return sdk.ModuleDescriptor{
		Type: "http", Version: "2", Name: "HTTP", Description: "请求 HTTP/HTTPS 接口并观察响应内容变化。",
		ListSummary: &sdk.ModuleListSummaryDescriptor{Fields: []string{"url"}},
		ConfigSchema: map[string]any{
			"type":     "object",
			"required": []string{"url"},
			"properties": map[string]any{
				"url":              map[string]any{"type": "string", "title": "URL", "format": "uri", "required": true},
				"method":           map[string]any{"type": "string", "enum": supportedMethods, "default": "GET"},
				"body_mode":        map[string]any{"type": "string", "enum": supportedBodyModes, "default": "none", "visible_when": []map[string]any{{"field": "method", "operator": "in", "value": bodyMethods}}},
				"timeout_seconds":  map[string]any{"type": "integer", "default": defaultTimeoutSeconds, "minimum": 1, "maximum": 300},
				"follow_redirects": map[string]any{"type": "boolean", "default": true},
				"max_redirects":    map[string]any{"type": "integer", "default": defaultMaxRedirects, "minimum": 1, "maximum": 50},
				"verify_tls":       map[string]any{"type": "boolean", "default": true},
				"proxy_url":        map[string]any{"type": "string", "format": "uri"},
				"query":            map[string]any{"type": "object", "title": "查询参数"},
				"headers":          map[string]any{"type": "object", "title": "请求头"},
				"form_fields":      map[string]any{"type": "object", "title": "表单字段"},
				"json_body":        map[string]any{"type": "string", "title": "JSON 请求体", "multiline": true, "format": "json"},
				"raw_body":         map[string]any{"type": "string", "title": "原始请求体", "multiline": true},
				"response_mode":    map[string]any{"type": "string", "enum": []string{"auto", "text", "json"}, "default": "auto"},
				"normalize":        map[string]any{"type": "string", "enum": []string{"raw", "trim", "json"}, "default": "trim"},
				"max_body_bytes":   map[string]any{"type": "integer", "default": defaultMaxBodyBytes, "minimum": 1024, "maximum": 1048576},
			},
		},
		Parameters: []sdk.ParameterDescriptor{
			{Key: "url", Label: "请求 URL", Type: sdk.ParameterURL, Required: true, Placeholder: "https://example.com/api", Order: 10},
			{Key: "method", Label: "请求方法", Type: sdk.ParameterList, Default: "GET", Order: 20, Options: methodOptions()},
			{Key: "timeout_seconds", Label: "请求超时", Type: sdk.ParameterDuration, Default: defaultTimeoutSeconds, Minimum: sdk.Float64(1), Maximum: sdk.Float64(300), Unit: "秒", Order: 30},
			{Key: "proxy_url", Label: "HTTP 代理", Type: sdk.ParameterURL, Order: 40, Placeholder: "http://127.0.0.1:7890", Description: "可选，仅支持 HTTP/HTTPS 代理。"},
			{Key: "response_mode", Label: "响应解析", Type: sdk.ParameterList, Default: "auto", Order: 50, Options: []sdk.ParameterOption{{Value: "auto", Label: "自动"}, {Value: "text", Label: "仅文本"}, {Value: "json", Label: "JSON"}}},
			{Key: "normalize", Label: "内容规范化", Type: sdk.ParameterList, Default: "trim", Order: 60, Options: []sdk.ParameterOption{{Value: "raw", Label: "保留原文"}, {Value: "trim", Label: "去除首尾空白"}, {Value: "json", Label: "规范化 JSON"}}},
			{Key: "max_body_bytes", Label: "最大响应体大小", Type: sdk.ParameterInteger, Default: defaultMaxBodyBytes, Minimum: sdk.Float64(1024), Maximum: sdk.Float64(1048576), Unit: "字节", Order: 70},
			{Key: "query", Label: "查询参数", Type: sdk.ParameterMap, FullWidth: true, Order: 100, Description: "每行一个参数；同名参数可使用逗号分隔值。"},
			{Key: "headers", Label: "请求头", Type: sdk.ParameterMap, FullWidth: true, Order: 110, Description: "每行定义一个请求头，键和值都会按原样发送。"},
			{Key: "body_mode", Label: "请求体类型", Type: sdk.ParameterList, Default: "none", Order: 200, Options: []sdk.ParameterOption{{Value: "none", Label: "无请求体"}}, OptionsWhen: []sdk.ParameterOptionSet{{When: inCondition("method", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"), Options: bodyModeOptions()}}, VisibleWhen: inCondition("method", "POST", "PUT", "PATCH", "DELETE", "OPTIONS")},
			{Key: "form_fields", Label: "表单字段", Type: sdk.ParameterMap, FullWidth: true, Order: 210, Description: "表单模式下每行定义一个字段。", VisibleWhen: bodyModeConditions("form_urlencoded", "multipart_form")},
			{Key: "json_body", Label: "JSON 请求体", Type: sdk.ParameterText, FullWidth: true, Format: "json", Rows: 8, Order: 220, Placeholder: "{\n  \"key\": \"value\"\n}", Description: "支持对象、数组和其他合法 JSON 值。", VisibleWhen: bodyModeConditions("raw_json")},
			{Key: "raw_body", Label: "原始请求体", Type: sdk.ParameterText, FullWidth: true, Rows: 8, Order: 230, Placeholder: "输入要发送的原始文本内容", VisibleWhen: bodyModeConditions("raw_text")},
			{Key: "follow_redirects", Label: "跟随重定向", Type: sdk.ParameterBoolean, Default: true, Order: 300, Description: "自动跟随 3xx 重定向响应。"},
			{Key: "verify_tls", Label: "校验 TLS 证书", Type: sdk.ParameterBoolean, Default: true, Order: 310, Description: "校验 HTTPS 服务端证书；关闭后允许不受信任的证书。"},
			{Key: "max_redirects", Label: "最大重定向次数", Type: sdk.ParameterInteger, Default: defaultMaxRedirects, Minimum: sdk.Float64(1), Maximum: sdk.Float64(50), Order: 320, VisibleWhen: equalsCondition("follow_redirects", true)},
		},
		ResultSchema: map[string]any{"type": "object", "properties": map[string]any{
			"success": map[string]any{"type": "boolean"}, "status_code": map[string]any{"type": "integer"}, "duration_ms": map[string]any{"type": "number"}, "response_headers": map[string]any{"type": "object"},
			"body_text": map[string]any{"type": "string"}, "body_json": map[string]any{}, "body_hash": map[string]any{"type": "string"}, "body_size": map[string]any{"type": "integer"}, "truncated": map[string]any{"type": "boolean"},
		}},
		Fields: []sdk.FieldDescriptor{
			{Name: "success", Label: "请求成功", Type: "boolean", Operators: []string{"is_true", "is_false"}},
			{Name: "status_code", Label: "状态码", Type: "number", Operators: []string{"equals", "not_equals", "gt", "gte", "lt", "lte", "changed"}},
			{Name: "duration_ms", Label: "响应耗时(ms)", Type: "number", Operators: []string{"gt", "gte", "lt", "lte", "changed"}},
			{Name: "body_text", Label: "响应文本", Type: "string", Operators: []string{"equals", "not_equals", "contains", "not_contains", "regex", "changed"}},
			{Name: "body_json", Label: "响应 JSON", Type: "json", Path: true, Operators: []string{"equals", "not_equals", "contains", "gt", "gte", "lt", "lte", "changed"}},
			{Name: "body_hash", Label: "内容哈希", Type: "string", Operators: []string{"equals", "not_equals", "changed"}},
		},
		ResultSets: []sdk.ResultSetDescriptor{{
			Key: "response", Label: "HTTP 响应", Description: "本次 HTTP 请求得到的响应与解析结果。", Fields: []sdk.ResultFieldDescriptor{
				{Name: "success", Label: "请求成功", Type: "boolean", Operators: []string{"is_true", "is_false", "changed"}},
				{Name: "status_code", Label: "状态码", Type: "number", Operators: []string{"equals", "not_equals", "gt", "gte", "lt", "lte", "changed"}},
				{Name: "duration_ms", Label: "响应耗时", Description: "请求完成所需的毫秒数。", Type: "number", Unit: "ms", Operators: []string{"equals", "gt", "gte", "lt", "lte", "changed"}},
				{Name: "response_headers", Label: "响应头", Type: "map", Operators: []string{"exists", "contains", "changed"}},
				{Name: "body_text", Label: "响应文本", Type: "text", Operators: []string{"equals", "not_equals", "contains", "not_contains", "regex", "changed"}},
				{Name: "body_json", Label: "响应 JSON", Type: "json", Path: true, Operators: []string{"exists", "equals", "not_equals", "contains", "changed"}},
				{Name: "body_hash", Label: "内容哈希", Type: "string", Operators: []string{"equals", "not_equals", "changed"}},
				{Name: "body_size", Label: "响应大小", Type: "number", Unit: "字节", Operators: []string{"equals", "gt", "gte", "lt", "lte", "changed"}},
				{Name: "truncated", Label: "响应是否截断", Type: "boolean", Operators: []string{"is_true", "is_false", "changed"}},
			},
		}},
	}
}

func methodOptions() []sdk.ParameterOption {
	return []sdk.ParameterOption{{Value: "GET", Label: "GET"}, {Value: "HEAD", Label: "HEAD"}, {Value: "POST", Label: "POST"}, {Value: "PUT", Label: "PUT"}, {Value: "PATCH", Label: "PATCH"}, {Value: "DELETE", Label: "DELETE"}, {Value: "OPTIONS", Label: "OPTIONS"}, {Value: "TRACE", Label: "TRACE"}, {Value: "CONNECT", Label: "CONNECT"}}
}

func bodyModeOptions() []sdk.ParameterOption {
	return []sdk.ParameterOption{{Value: "none", Label: "无请求体"}, {Value: "form_urlencoded", Label: "表单（URL 编码）"}, {Value: "multipart_form", Label: "表单（Multipart）"}, {Value: "raw_json", Label: "Raw JSON"}, {Value: "raw_text", Label: "Raw 文本"}}
}

func equalsCondition(field string, value any) []sdk.ParameterCondition {
	return []sdk.ParameterCondition{{Field: field, Operator: "equals", Value: value}}
}

func inCondition(field string, values ...any) []sdk.ParameterCondition {
	return []sdk.ParameterCondition{{Field: field, Operator: "in", Value: values}}
}

func bodyModeConditions(modes ...string) []sdk.ParameterCondition {
	conditions := inCondition("method", "POST", "PUT", "PATCH", "DELETE", "OPTIONS")
	values := make([]any, 0, len(modes))
	for _, mode := range modes {
		values = append(values, mode)
	}
	return append(conditions, sdk.ParameterCondition{Field: "body_mode", Operator: "in", Value: values})
}

func (m *Module) ValidateConfig(raw json.RawMessage) error {
	var config map[string]any
	if err := json.Unmarshal(raw, &config); err != nil {
		return fmt.Errorf("invalid HTTP config: %w", err)
	}

	parsedURL, err := url.Parse(valueString(config, "url", ""))
	if err != nil || parsedURL.Host == "" || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
		return errors.New("url must be a valid http:// or https:// URL")
	}
	method := strings.ToUpper(valueString(config, "method", "GET"))
	if !contains(supportedMethods, method) {
		return fmt.Errorf("unsupported HTTP method %q", method)
	}
	bodyMode := strings.ToLower(valueString(config, "body_mode", "none"))
	if !contains(supportedBodyModes, bodyMode) {
		return fmt.Errorf("unsupported HTTP body mode %q", bodyMode)
	}
	if !contains(bodyMethods, method) && bodyMode != "none" {
		return fmt.Errorf("HTTP method %q does not support a request body", method)
	}
	if err := validateMap(config, "query"); err != nil {
		return err
	}
	if err := validateMap(config, "headers"); err != nil {
		return err
	}
	if bodyMode == "form_urlencoded" || bodyMode == "multipart_form" {
		if err := validateMap(config, "form_fields"); err != nil {
			return err
		}
	}
	if bodyMode == "raw_json" {
		body := valueString(config, "json_body", "")
		if body == "" {
			return errors.New("json_body is required for raw_json body mode")
		}
		var value any
		if err := json.Unmarshal([]byte(body), &value); err != nil {
			return fmt.Errorf("json_body must contain valid JSON: %w", err)
		}
	}
	if timeout := valueInt(config, "timeout_seconds", defaultTimeoutSeconds); timeout < 1 || timeout > 300 {
		return errors.New("timeout_seconds must be between 1 and 300")
	}
	if maxBytes := valueInt(config, "max_body_bytes", defaultMaxBodyBytes); maxBytes < 1024 || maxBytes > 1048576 {
		return errors.New("max_body_bytes must be between 1024 and 1048576")
	}
	if maxRedirects := valueInt(config, "max_redirects", defaultMaxRedirects); maxRedirects < 1 || maxRedirects > 50 {
		return errors.New("max_redirects must be between 1 and 50")
	}
	if proxy := valueString(config, "proxy_url", ""); proxy != "" {
		proxyURL, proxyErr := url.Parse(proxy)
		if proxyErr != nil || proxyURL.Host == "" || (proxyURL.Scheme != "http" && proxyURL.Scheme != "https") {
			return errors.New("proxy_url must be a valid http:// or https:// URL")
		}
	}
	return nil
}

func (m *Module) Execute(ctx context.Context, raw json.RawMessage) (sdk.Observation, error) {
	var config map[string]any
	if err := json.Unmarshal(raw, &config); err != nil {
		return failedObservation(), err
	}
	if err := m.ValidateConfig(raw); err != nil {
		return failedObservation(), err
	}

	requestContext, cancel := context.WithTimeout(ctx, time.Duration(valueInt(config, "timeout_seconds", defaultTimeoutSeconds))*time.Second)
	defer cancel()
	body, contentType, err := requestBody(config)
	if err != nil {
		return failedObservation(), err
	}
	requestURL, _ := url.Parse(valueString(config, "url", ""))
	addQuery(requestURL, config["query"])
	request, err := http.NewRequestWithContext(requestContext, strings.ToUpper(valueString(config, "method", "GET")), requestURL.String(), body)
	if err != nil {
		return failedObservation(), err
	}
	applyHeaders(request, config["headers"])
	if contentType != "" && request.Header.Get("Content-Type") == "" {
		request.Header.Set("Content-Type", contentType)
	}
	client := m.client(config)
	started := time.Now()
	response, err := client.Do(request)
	if err != nil {
		result := map[string]any{"success": false, "status_code": 0, "duration_ms": time.Since(started).Milliseconds()}
		return sdk.Observation{Success: false, SchemaVersion: "1", Result: result, ResultSets: map[string]map[string]any{"response": copyMap(result)}, Summary: requestFailureSummary(request, result["duration_ms"])}, err
	}
	defer response.Body.Close()
	maxBytes := int64(valueInt(config, "max_body_bytes", defaultMaxBodyBytes))
	data, readErr := io.ReadAll(io.LimitReader(response.Body, maxBytes+1))
	if readErr != nil {
		result := map[string]any{"success": false, "status_code": response.StatusCode, "duration_ms": time.Since(started).Milliseconds()}
		return sdk.Observation{Success: false, SchemaVersion: "1", Result: result, ResultSets: map[string]map[string]any{"response": copyMap(result)}, Summary: responseFailureSummary(request, response, result)}, readErr
	}
	truncated := int64(len(data)) > maxBytes
	if truncated {
		data = data[:maxBytes]
	}
	bodyText := normalize(string(data), valueString(config, "normalize", "trim"))
	result := map[string]any{"success": true, "status_code": response.StatusCode, "duration_ms": time.Since(started).Milliseconds(), "response_headers": flatten(response.Header), "body_text": bodyText, "body_hash": hash(bodyText), "body_size": len(data), "truncated": truncated}
	mode := strings.ToLower(valueString(config, "response_mode", "auto"))
	if mode == "json" || (mode == "auto" && strings.Contains(strings.ToLower(response.Header.Get("Content-Type")), "json")) {
		var parsed any
		if json.Unmarshal(data, &parsed) == nil {
			result["body_json"] = parsed
		}
	}
	return sdk.Observation{Success: true, SchemaVersion: "1", Result: result, ResultSets: map[string]map[string]any{"response": copyMap(result)}, Summary: responseSummary(request, response, result)}, nil
}

func copyMap(source map[string]any) map[string]any {
	result := make(map[string]any, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

func (m *Module) client(config map[string]any) *http.Client {
	var client http.Client
	if m.Client != nil {
		client = *m.Client
	} else {
		client.Transport = http.DefaultTransport.(*http.Transport).Clone()
	}
	if client.Transport == nil {
		client.Transport = http.DefaultTransport.(*http.Transport).Clone()
	}
	if transport, ok := client.Transport.(*http.Transport); ok {
		transport = transport.Clone()
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: !valueBool(config, "verify_tls", true)} //nolint:gosec -- explicitly configured by the user.
		if proxy := valueString(config, "proxy_url", ""); proxy != "" {
			if proxyURL, err := url.Parse(proxy); err == nil {
				transport.Proxy = http.ProxyURL(proxyURL)
			}
		}
		client.Transport = transport
	}
	if !valueBool(config, "follow_redirects", true) {
		client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	} else {
		maxRedirects := valueInt(config, "max_redirects", defaultMaxRedirects)
		client.CheckRedirect = func(_ *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return fmt.Errorf("stopped after %d redirects", maxRedirects)
			}
			return nil
		}
	}
	return &client
}

func requestBody(config map[string]any) (io.Reader, string, error) {
	switch mode := strings.ToLower(valueString(config, "body_mode", "none")); mode {
	case "none":
		return nil, "", nil
	case "form_urlencoded":
		values := url.Values{}
		addValues(values, config["form_fields"])
		return strings.NewReader(values.Encode()), "application/x-www-form-urlencoded", nil
	case "multipart_form":
		var buffer bytes.Buffer
		writer := multipart.NewWriter(&buffer)
		for key, values := range stringValues(config["form_fields"]) {
			for _, value := range values {
				if err := writer.WriteField(key, value); err != nil {
					return nil, "", fmt.Errorf("write multipart field %q: %w", key, err)
				}
			}
		}
		if err := writer.Close(); err != nil {
			return nil, "", err
		}
		return bytes.NewReader(buffer.Bytes()), writer.FormDataContentType(), nil
	case "raw_json":
		body := valueString(config, "json_body", "")
		return strings.NewReader(body), "application/json", nil
	case "raw_text":
		return strings.NewReader(valueString(config, "raw_body", "")), "text/plain; charset=utf-8", nil
	default:
		return nil, "", fmt.Errorf("unsupported HTTP body mode %q", mode)
	}
}

func addQuery(requestURL *url.URL, source any) {
	values := requestURL.Query()
	addValues(values, source)
	requestURL.RawQuery = values.Encode()
}

func addValues(values url.Values, source any) {
	for key, items := range stringValues(source) {
		for _, item := range items {
			values.Add(key, item)
		}
	}
}

func applyHeaders(request *http.Request, source any) {
	for key, values := range stringValues(source) {
		if strings.EqualFold(key, "Host") {
			if len(values) > 0 {
				request.Host = values[0]
			}
			continue
		}
		for index, value := range values {
			if index == 0 {
				request.Header.Set(key, value)
			} else {
				request.Header.Add(key, value)
			}
		}
	}
}

func stringValues(source any) map[string][]string {
	result := map[string][]string{}
	values, ok := source.(map[string]any)
	if !ok {
		return result
	}
	for key, value := range values {
		switch typed := value.(type) {
		case []any:
			for _, item := range typed {
				result[key] = append(result[key], fmt.Sprint(item))
			}
		case []string:
			result[key] = append(result[key], typed...)
		default:
			result[key] = []string{fmt.Sprint(value)}
		}
	}
	return result
}

func validateMap(config map[string]any, key string) error {
	if value, exists := config[key]; exists && value != nil {
		if _, ok := value.(map[string]any); !ok {
			return fmt.Errorf("%s must be an object", key)
		}
	}
	return nil
}

func contains(values []string, value string) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}

func failedObservation() sdk.Observation {
	return sdk.Observation{Success: false, SchemaVersion: "1", Result: map[string]any{"success": false}, Summary: "HTTP 请求未执行"}
}

func responseSummary(request *http.Request, response *http.Response, result map[string]any) string {
	status := response.Status
	if status == "" {
		status = fmt.Sprintf("%d", response.StatusCode)
		if text := http.StatusText(response.StatusCode); text != "" {
			status += " " + text
		}
	}
	lines := []string{
		fmt.Sprintf("HTTP 请求：%s %s", request.Method, request.URL.String()),
		fmt.Sprintf("响应状态：%s", status),
		fmt.Sprintf("响应耗时：%v ms", result["duration_ms"]),
	}
	contentType := response.Header.Get("Content-Type")
	bodyInfo := fmt.Sprintf("%v 字节", result["body_size"])
	if contentType != "" {
		bodyInfo = contentType + "，" + bodyInfo
	}
	if bodyJSON, ok := result["body_json"]; ok && bodyJSON != nil {
		bodyInfo = "JSON，" + bodyInfo
	}
	if truncated, ok := result["truncated"].(bool); ok && truncated {
		bodyInfo += "，已截断"
	} else {
		bodyInfo += "，未截断"
	}
	lines = append(lines, "响应体："+bodyInfo)
	if bodyHash, ok := result["body_hash"].(string); ok && bodyHash != "" {
		lines = append(lines, "内容哈希："+bodyHash)
	}
	return strings.Join(lines, "\n")
}

func requestFailureSummary(request *http.Request, duration any) string {
	return fmt.Sprintf("HTTP 请求：%s %s\n响应状态：未获取\n请求耗时：%v ms", request.Method, request.URL.String(), duration)
}

func responseFailureSummary(request *http.Request, response *http.Response, result map[string]any) string {
	status := fmt.Sprintf("%d", response.StatusCode)
	if response.Status != "" {
		status = response.Status
	}
	return fmt.Sprintf("HTTP 请求：%s %s\n响应状态：%s\n响应耗时：%v ms\n响应体：读取失败", request.Method, request.URL.String(), status, result["duration_ms"])
}

func valueString(config map[string]any, key, fallback string) string {
	if value, ok := config[key].(string); ok {
		return value
	}
	return fallback
}

func valueInt(config map[string]any, key string, fallback int) int {
	if value, ok := config[key].(float64); ok {
		return int(value)
	}
	if value, ok := config[key].(int); ok {
		return value
	}
	return fallback
}

func valueBool(config map[string]any, key string, fallback bool) bool {
	if value, ok := config[key].(bool); ok {
		return value
	}
	return fallback
}

func normalize(value, mode string) string {
	if mode == "trim" {
		return strings.TrimSpace(value)
	}
	if mode == "json" {
		var parsed any
		if json.Unmarshal([]byte(value), &parsed) == nil {
			data, _ := json.Marshal(parsed)
			return string(data)
		}
	}
	return value
}

func flatten(headers http.Header) map[string]any {
	result := make(map[string]any, len(headers))
	for key, values := range headers {
		if len(values) > 0 {
			result[strings.ToLower(key)] = values[0]
		}
	}
	return result
}

func hash(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}
