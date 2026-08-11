package browsermonitor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/hanxuanyu/meerkit/sdk"
)

const (
	moduleType          = "browser-example"
	moduleVersion       = "1"
	configVersion       = "1"
	resultSchemaVersion = "1"
	defaultTimeout      = 60 * time.Second
)

type Config struct {
	URL            string `json:"url"`
	Selector       string `json:"selector"`
	APIURLContains string `json:"api_url_contains,omitempty"`
}

type Module struct{ browser sdk.BrowserClient }

func New(browser sdk.BrowserClient) *Module { return &Module{browser: browser} }

func (m *Module) Descriptor() sdk.ModuleDescriptor {
	return sdk.ModuleDescriptor{
		Type: moduleType, Version: moduleVersion, ConfigVersion: configVersion, ResultSchemaVersion: resultSchemaVersion,
		Name: "浏览器页面示例", Description: "通过 Meerkit Browser Agent 打开页面，提取指定 DOM 文本并可捕获匹配的接口响应。",
		ListSummary: &sdk.ModuleListSummaryDescriptor{Fields: []string{"url", "selector"}, Separator: " · "},
		ConfigSchema: map[string]any{"type": "object", "required": []string{"url", "selector"}, "properties": map[string]any{
			"url": map[string]any{"type": "string", "format": "uri"}, "selector": map[string]any{"type": "string", "minLength": 1}, "api_url_contains": map[string]any{"type": "string"},
		}},
		Parameters: []sdk.ParameterDescriptor{
			{Key: "url", Label: "页面地址", Type: sdk.ParameterURL, Required: true, Order: 10, FullWidth: true, Placeholder: "https://example.com"},
			{Key: "selector", Label: "CSS Selector", Type: sdk.ParameterString, Required: true, Order: 20, FullWidth: true, Placeholder: "main h1"},
			{Key: "api_url_contains", Label: "接口 URL 片段", Type: sdk.ParameterString, Order: 30, FullWidth: true, Description: "可选。捕获 URL 中包含该文本的响应。", Placeholder: "/api/status"},
		},
		ResultSchema: map[string]any{"type": "object", "properties": map[string]any{
			"success": map[string]any{"type": "boolean"}, "text": map[string]any{"type": "string"}, "title": map[string]any{"type": "string"}, "page_url": map[string]any{"type": "string"}, "tag_name": map[string]any{"type": "string"}, "duration_ms": map[string]any{"type": "number"}, "api_url": map[string]any{"type": "string"}, "api_status": map[string]any{"type": "number"}, "api_body": map[string]any{"type": "string"},
		}},
		Fields: []sdk.FieldDescriptor{
			{Name: "success", Label: "采集成功", Type: "boolean", Operators: []string{"is_true", "is_false", "changed"}},
			{Name: "text", Label: "元素文本", Type: "string", Operators: []string{"equals", "not_equals", "contains", "not_contains", "regex", "changed"}},
			{Name: "title", Label: "页面标题", Type: "string", Operators: []string{"equals", "not_equals", "contains", "regex", "changed"}},
			{Name: "page_url", Label: "最终页面地址", Type: "string", Operators: []string{"equals", "not_equals", "contains", "regex", "changed"}},
			{Name: "api_status", Label: "接口状态码", Type: "number", Operators: []string{"equals", "gt", "gte", "lt", "lte", "changed"}},
			{Name: "api_body", Label: "接口响应", Type: "string", Operators: []string{"equals", "not_equals", "contains", "not_contains", "regex", "changed"}},
		},
		ResultSets: []sdk.ResultSetDescriptor{
			{Key: "page", Label: "浏览器页面", Fields: []sdk.ResultFieldDescriptor{
				{Name: "success", Label: "采集成功", Type: "boolean", Operators: []string{"is_true", "is_false", "changed"}},
				{Name: "text", Label: "元素文本", Type: "text", Operators: []string{"equals", "not_equals", "contains", "not_contains", "regex", "changed"}},
				{Name: "title", Label: "页面标题", Type: "string", Operators: []string{"equals", "not_equals", "contains", "regex", "changed"}},
				{Name: "page_url", Label: "页面地址", Type: "string", Operators: []string{"equals", "not_equals", "contains", "regex", "changed"}},
				{Name: "tag_name", Label: "元素标签", Type: "string", Operators: []string{"equals", "not_equals", "changed"}},
				{Name: "duration_ms", Label: "执行耗时", Type: "number", Unit: "ms", Operators: []string{"equals", "gt", "gte", "lt", "lte", "changed"}},
			}},
			{Key: "api", Label: "匹配接口", Fields: []sdk.ResultFieldDescriptor{
				{Name: "url", Label: "接口地址", Type: "string", Operators: []string{"equals", "not_equals", "contains", "regex", "changed"}},
				{Name: "status", Label: "状态码", Type: "number", Operators: []string{"equals", "gt", "gte", "lt", "lte", "changed"}},
				{Name: "mime_type", Label: "内容类型", Type: "string", Operators: []string{"equals", "not_equals", "contains", "changed"}},
				{Name: "body", Label: "响应正文", Type: "text", Operators: []string{"equals", "not_equals", "contains", "not_contains", "regex", "changed"}},
			}},
		},
	}
}

func (m *Module) ValidateConfig(raw json.RawMessage) error {
	var config Config
	if err := json.Unmarshal(raw, &config); err != nil {
		return fmt.Errorf("invalid browser example config: %w", err)
	}
	parsed, err := url.Parse(strings.TrimSpace(config.URL))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return errors.New("url must be a valid HTTP or HTTPS URL")
	}
	if selector := strings.TrimSpace(config.Selector); selector == "" || len(selector) > 4096 {
		return errors.New("selector must contain between 1 and 4096 characters")
	}
	if len(config.APIURLContains) > 2048 {
		return errors.New("api_url_contains cannot exceed 2048 characters")
	}
	if m.browser == nil {
		return errors.New("browser capability is unavailable")
	}
	return nil
}

func (m *Module) Execute(ctx context.Context, raw json.RawMessage) (sdk.Observation, error) {
	if err := m.ValidateConfig(raw); err != nil {
		return failedObservation(err), err
	}
	var config Config
	_ = json.Unmarshal(raw, &config)
	request := sdk.BrowserRunRequest{TimeoutMS: int(defaultTimeout.Milliseconds()), Actions: []sdk.BrowserAction{
		{ID: "open", Type: "tab.open", Params: map[string]any{"url": config.URL, "active": false}},
		{ID: "wait", Type: "page.wait", Params: map[string]any{"selector": config.Selector, "timeout_ms": int(defaultTimeout.Milliseconds())}},
		{ID: "element", Type: "dom.query", Params: map[string]any{"selector": config.Selector, "max_length": 65536}},
	}}
	if config.APIURLContains != "" {
		request.NetworkCaptures = []sdk.BrowserNetworkCapture{{ID: "api", URLContains: config.APIURLContains, MaxBodyBytes: 262144}}
	}
	executionContext, cancel := context.WithTimeout(ctx, defaultTimeout+5*time.Second)
	defer cancel()
	result, err := m.browser.Run(executionContext, request)
	if err != nil {
		return failedObservation(err), err
	}
	element := actionData(result.Actions, "element")
	text := stringValue(element, "text")
	page := map[string]any{"success": true, "text": text, "title": stringValue(element, "title"), "page_url": stringValue(element, "url"), "tag_name": stringValue(element, "tag_name"), "duration_ms": result.Duration}
	api := map[string]any{"url": "", "status": 0, "mime_type": "", "body": ""}
	if len(result.Network) > 0 {
		captured := result.Network[len(result.Network)-1]
		api = map[string]any{"url": captured.URL, "status": captured.Status, "mime_type": captured.MimeType, "body": captured.Body}
	}
	resultMap := map[string]any{"success": true, "text": text, "title": page["title"], "page_url": page["page_url"], "tag_name": page["tag_name"], "duration_ms": result.Duration, "api_url": api["url"], "api_status": api["status"], "api_body": api["body"]}
	return sdk.Observation{Success: true, SchemaVersion: resultSchemaVersion, Result: resultMap, ResultSets: map[string]map[string]any{"page": page, "api": api}, Summary: fmt.Sprintf("浏览器采集成功：%s", summarize(text))}, nil
}

func failedObservation(err error) sdk.Observation {
	message := "浏览器采集失败"
	if err != nil {
		message += "：" + err.Error()
	}
	result := map[string]any{"success": false, "text": "", "title": "", "page_url": "", "tag_name": "", "duration_ms": int64(0), "api_url": "", "api_status": 0, "api_body": ""}
	return sdk.Observation{Success: false, SchemaVersion: resultSchemaVersion, Result: result, ResultSets: map[string]map[string]any{"page": result}, Summary: message, ErrorCode: "browser_execution_failed", ErrorMessage: message}
}

func actionData(results []sdk.BrowserActionResult, id string) map[string]any {
	for _, result := range results {
		if result.ID == id {
			return result.Data
		}
	}
	return map[string]any{}
}

func stringValue(values map[string]any, key string) string {
	value, _ := values[key].(string)
	return value
}

func summarize(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	if len([]rune(value)) > 80 {
		return string([]rune(value)[:80]) + "..."
	}
	if value == "" {
		return "元素文本为空"
	}
	return value
}
