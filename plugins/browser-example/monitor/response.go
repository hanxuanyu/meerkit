package browsermonitor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/hanxuanyu/meerkit/sdk"
)

const responseModuleType = "browser-example-response"

type ResponseConfig struct {
	pageConfig
	URLContains  string `json:"url_contains"`
	MaxBodyBytes int    `json:"max_body_bytes"`
}

type ResponseModule struct{ moduleBase }

func NewResponse(browser sdk.BrowserClient) *ResponseModule {
	return &ResponseModule{moduleBase{browser: browser, reuseNamespace: responseModuleType}}
}

func (m *ResponseModule) Descriptor() sdk.ModuleDescriptor {
	properties := commonConfigProperties()
	properties["url_contains"] = map[string]any{"type": "string", "minLength": 1, "maxLength": 2048}
	properties["max_body_bytes"] = map[string]any{"type": "integer", "default": 262144, "minimum": 1024, "maximum": 1048576}
	parameters := commonParameters()
	parameters = append(parameters,
		sdk.ParameterDescriptor{Key: "url_contains", Label: "响应 URL 匹配文本", Type: sdk.ParameterString, Required: true, Order: 20, FullWidth: true, Description: "返回 URL 中包含该文本的最近一次响应。", Placeholder: "/api/status"},
		sdk.ParameterDescriptor{Key: "max_body_bytes", Label: "最大响应体大小", Type: sdk.ParameterInteger, Default: 262144, Minimum: sdk.Float64(1024), Maximum: sdk.Float64(1048576), Unit: "字节", Order: 30},
	)
	fields := []sdk.ResultFieldDescriptor{
		{Name: "url", Label: "响应地址", Type: "string", Operators: stringOperators()},
		{Name: "method", Label: "请求方法", Type: "string", Operators: stringOperators()},
		{Name: "status", Label: "状态码", Type: "number", Operators: numberOperators()},
		{Name: "status_text", Label: "状态文本", Type: "string", Operators: stringOperators()},
		{Name: "resource_type", Label: "资源类型", Type: "string", Operators: stringOperators()},
		{Name: "mime_type", Label: "内容类型", Type: "string", Operators: stringOperators()},
		{Name: "headers", Label: "响应头", Type: "map", Operators: []string{"exists", "contains", "changed"}},
		{Name: "body", Label: "响应正文", Type: "text", Operators: stringOperators()},
		{Name: "body_base64", Label: "正文为 Base64", Type: "boolean", Operators: boolOperators()},
		{Name: "truncated", Label: "正文已截断", Type: "boolean", Operators: boolOperators()},
		{Name: "error", Label: "捕获错误", Type: "string", Operators: stringOperators()},
		{Name: "reused_tab", Label: "复用了标签页", Type: "boolean", Operators: boolOperators()},
		{Name: "duration_ms", Label: "执行耗时", Type: "number", Unit: "ms", Operators: numberOperators()},
	}
	return sdk.ModuleDescriptor{
		Type: responseModuleType, Version: moduleVersion, ConfigVersion: configVersion, ResultSchemaVersion: resultSchemaVersion,
		Name: "Example - 最近 URL 响应", Description: "示例模块：获取符合 URL 包含规则的最近一次网络响应信息。",
		ListSummary:  &sdk.ModuleListSummaryDescriptor{Fields: []string{"url", "url_contains"}, Separator: " · "},
		ConfigSchema: map[string]any{"type": "object", "required": []string{"url", "url_contains"}, "properties": properties}, Parameters: parameters,
		ResultSchema: resultSchema(fields), Fields: legacyFields(fields), ResultSets: []sdk.ResultSetDescriptor{{Key: "response", Label: "最近匹配响应", Fields: fields}},
	}
}

func (m *ResponseModule) ValidateConfig(raw json.RawMessage) error {
	var config ResponseConfig
	if err := decodeConfig(raw, &config); err != nil {
		return err
	}
	if err := validatePageConfig(config.pageConfig, m.browser); err != nil {
		return err
	}
	if rule := strings.TrimSpace(config.URLContains); rule == "" || len(rule) > 2048 {
		return errors.New("url_contains must contain between 1 and 2048 characters")
	}
	if config.MaxBodyBytes != 0 && (config.MaxBodyBytes < 1024 || config.MaxBodyBytes > 1048576) {
		return errors.New("max_body_bytes must be between 1024 and 1048576")
	}
	return nil
}

func (m *ResponseModule) Execute(ctx context.Context, raw json.RawMessage) (sdk.Observation, error) {
	if err := m.ValidateConfig(raw); err != nil {
		return failedObservation("response", err.Error(), emptyResponseResult()), err
	}
	var config ResponseConfig
	_ = json.Unmarshal(raw, &config)
	if config.MaxBodyBytes == 0 {
		config.MaxBodyBytes = 262144
	}
	result, err := m.run(ctx, config.pageConfig,
		[]sdk.BrowserAction{{ID: "settle", Type: "page.wait", Params: map[string]any{"duration_ms": 1000}}},
		sdk.BrowserNetworkCapture{ID: "response", URLContains: config.URLContains, MaxBodyBytes: config.MaxBodyBytes},
	)
	if err != nil {
		return failedObservation("response", err.Error(), emptyResponseResult()), err
	}
	var latest *sdk.BrowserNetworkResult
	for index := range result.Network {
		if result.Network[index].CaptureID == "response" {
			latest = &result.Network[index]
		}
	}
	if latest == nil {
		err = fmt.Errorf("no response URL contained %q", config.URLContains)
		return failedObservation("response", err.Error(), emptyResponseResult()), err
	}
	open := actionData(result.Actions, "open")
	value := map[string]any{"url": latest.URL, "method": latest.Method, "status": latest.Status, "status_text": latest.StatusText, "resource_type": latest.ResourceType, "mime_type": latest.MimeType, "headers": latest.Headers, "body": latest.Body, "body_base64": latest.BodyBase64, "truncated": latest.Truncated, "error": latest.Error, "reused_tab": boolValue(open, "reused"), "duration_ms": result.Duration}
	return sdk.Observation{Success: true, SchemaVersion: resultSchemaVersion, Result: value, ResultSets: map[string]map[string]any{"response": value}, Summary: fmt.Sprintf("最近匹配响应：HTTP %d %s", latest.Status, latest.URL)}, nil
}

func emptyResponseResult() map[string]any {
	return map[string]any{"url": "", "method": "", "status": 0, "status_text": "", "resource_type": "", "mime_type": "", "headers": map[string]string{}, "body": "", "body_base64": false, "truncated": false, "error": "", "reused_tab": false, "duration_ms": int64(0)}
}
