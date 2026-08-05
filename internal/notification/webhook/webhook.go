package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"meerkit/internal/core"
	templateutil "meerkit/internal/template"
)

const defaultTimeoutSeconds = 30

var supportedMethods = []string{"GET", "POST"}

type Notifier struct {
	Client *http.Client
}

func New() *Notifier { return &Notifier{} }

func (n *Notifier) Descriptor() core.NotifierDescriptor {
	return core.NotifierDescriptor{Type: "webhook", Name: "Webhook", Description: "使用 GET 或 POST 将通知发送到自定义地址。", ConfigSchema: map[string]any{"type": "object", "required": []string{"url"}, "properties": map[string]any{
		"url":             map[string]any{"type": "string", "title": "URL", "format": "uri", "required": true},
		"method":          map[string]any{"type": "string", "enum": supportedMethods, "default": "POST"},
		"query":           map[string]any{"type": "object", "title": "查询参数"},
		"headers":         map[string]any{"type": "object", "title": "请求头"},
		"body_mode":       map[string]any{"type": "string", "enum": []string{"event_json", "form_urlencoded", "raw_json", "raw_text", "none"}, "default": "event_json"},
		"form_fields":     map[string]any{"type": "object", "title": "表单字段"},
		"json_body":       map[string]any{"type": "string", "title": "JSON 请求体", "multiline": true, "format": "json"},
		"raw_body":        map[string]any{"type": "string", "title": "原始请求体", "multiline": true},
		"timeout_seconds": map[string]any{"type": "integer", "default": defaultTimeoutSeconds},
	}}, Parameters: []core.ParameterDescriptor{
		{Key: "url", Label: "Webhook URL", Type: core.ParameterURL, Required: true, Placeholder: "https://example.com/webhook", Order: 10},
		{Key: "method", Label: "请求方法", Type: core.ParameterList, Default: "POST", Order: 20, Options: []core.ParameterOption{{Value: "GET", Label: "GET"}, {Value: "POST", Label: "POST"}}},
		{Key: "timeout_seconds", Label: "请求超时", Type: core.ParameterDuration, Default: defaultTimeoutSeconds, Minimum: core.Float64(1), Maximum: core.Float64(300), Unit: "秒", Order: 30},
		{Key: "query", Label: "查询参数", Type: core.ParameterMap, FullWidth: true, Order: 100, Description: "每行定义一个查询参数，GET 和 POST 均会附加到 URL。"},
		{Key: "headers", Label: "请求头", Type: core.ParameterMap, FullWidth: true, Order: 110, Description: "每行定义一个请求头，认证信息也可以直接写在这里。"},
		{Key: "body_mode", Label: "POST 请求体", Type: core.ParameterList, Default: "event_json", Order: 200, Options: bodyModeOptions(), VisibleWhen: equalsCondition("method", "POST")},
		{Key: "form_fields", Label: "表单字段", Type: core.ParameterMap, FullWidth: true, Order: 210, Description: "表单模式下每行定义一个字段。", VisibleWhen: bodyModeConditions("form_urlencoded")},
		{Key: "json_body", Label: "JSON 请求体", Type: core.ParameterText, FullWidth: true, Format: "json", Rows: 8, Order: 220, Placeholder: "{\n  \"event\": \"triggered\"\n}", Description: "发送固定 JSON 内容，不填写动态模板。", VisibleWhen: bodyModeConditions("raw_json")},
		{Key: "raw_body", Label: "原始请求体", Type: core.ParameterText, FullWidth: true, Rows: 8, Order: 230, Placeholder: "输入要发送的原始文本内容", VisibleWhen: bodyModeConditions("raw_text")},
	}}
}

func bodyModeOptions() []core.ParameterOption {
	return []core.ParameterOption{
		{Value: "event_json", Label: "事件 JSON"},
		{Value: "form_urlencoded", Label: "表单（URL 编码）"},
		{Value: "raw_json", Label: "Raw JSON"},
		{Value: "raw_text", Label: "Raw 文本"},
		{Value: "none", Label: "无请求体"},
	}
}

func equalsCondition(field string, value any) []core.ParameterCondition {
	return []core.ParameterCondition{{Field: field, Operator: "equals", Value: value}}
}

func bodyModeConditions(mode string) []core.ParameterCondition {
	return []core.ParameterCondition{equalsCondition("method", "POST")[0], {Field: "body_mode", Operator: "equals", Value: mode}}
}

func (n *Notifier) ValidateConfig(raw json.RawMessage) error {
	var config map[string]any
	if err := json.Unmarshal(raw, &config); err != nil {
		return err
	}
	parsedURL, err := url.Parse(stringValue(config, "url", ""))
	if err != nil || parsedURL.Host == "" || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
		return errors.New("webhook url must be a valid http:// or https:// URL")
	}
	method := strings.ToUpper(stringValue(config, "method", "POST"))
	if !contains(supportedMethods, method) {
		return fmt.Errorf("unsupported webhook method %q", method)
	}
	if err := validateMap(config, "query"); err != nil {
		return err
	}
	if err := validateMap(config, "headers"); err != nil {
		return err
	}
	bodyMode := strings.ToLower(stringValue(config, "body_mode", "event_json"))
	if method == "GET" {
		bodyMode = "none"
	}
	switch bodyMode {
	case "event_json", "none":
	case "form_urlencoded":
		if err := validateMap(config, "form_fields"); err != nil {
			return err
		}
	case "raw_json":
		body := stringValue(config, "json_body", "")
		if body == "" {
			return errors.New("json_body is required for raw_json body mode")
		}
		if len(templateutil.Scan(body)) > 0 {
			break
		}
		var value any
		if err := json.Unmarshal([]byte(body), &value); err != nil {
			return fmt.Errorf("json_body must contain valid JSON: %w", err)
		}
	case "raw_text":
	default:
		return fmt.Errorf("unsupported webhook body mode %q", bodyMode)
	}
	if timeout := intValue(config, "timeout_seconds", defaultTimeoutSeconds); timeout < 1 || timeout > 300 {
		return errors.New("timeout_seconds must be between 1 and 300")
	}
	return nil
}

func (n *Notifier) Send(ctx context.Context, raw json.RawMessage, event core.NotificationEvent) error {
	var config map[string]any
	if err := json.Unmarshal(raw, &config); err != nil {
		return err
	}
	if err := n.ValidateConfig(raw); err != nil {
		return err
	}
	rendered, missing, err := templateutil.Render(config, templateutil.NewContext(event))
	if err != nil {
		return err
	}
	if len(missing) > 0 {
		return fmt.Errorf("webhook template placeholders not found: %s", strings.Join(missing, ", "))
	}
	config, ok := rendered.(map[string]any)
	if !ok {
		return errors.New("invalid webhook configuration")
	}
	method := strings.ToUpper(stringValue(config, "method", "POST"))
	requestURL, err := url.Parse(stringValue(config, "url", ""))
	if err != nil {
		return err
	}
	addQuery(requestURL, config["query"])
	if method == "GET" {
		addQuery(requestURL, eventQuery(event))
	}
	body, contentType, err := requestBody(config, event)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, method, requestURL.String(), body)
	if err != nil {
		return err
	}
	applyHeaders(request, config["headers"])
	if contentType != "" && request.Header.Get("Content-Type") == "" {
		request.Header.Set("Content-Type", contentType)
	}
	client := n.Client
	if client == nil {
		client = &http.Client{}
	}
	requestContext, cancel := context.WithTimeout(request.Context(), time.Duration(intValue(config, "timeout_seconds", defaultTimeoutSeconds))*time.Second)
	defer cancel()
	response, err := client.Do(request.WithContext(requestContext))
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("webhook returned HTTP %d", response.StatusCode)
	}
	return nil
}

func requestBody(config map[string]any, event core.NotificationEvent) (*bytes.Reader, string, error) {
	if strings.ToUpper(stringValue(config, "method", "POST")) == "GET" {
		return bytes.NewReader(nil), "", nil
	}
	switch mode := strings.ToLower(stringValue(config, "body_mode", "event_json")); mode {
	case "event_json":
		data, err := json.Marshal(event)
		return bytes.NewReader(data), "application/json", err
	case "form_urlencoded":
		values := url.Values{}
		addValues(values, config["form_fields"])
		return bytes.NewReader([]byte(values.Encode())), "application/x-www-form-urlencoded", nil
	case "raw_json":
		body := stringValue(config, "json_body", "")
		var value any
		if err := json.Unmarshal([]byte(body), &value); err != nil {
			return nil, "", fmt.Errorf("json_body must contain valid JSON after template rendering: %w", err)
		}
		return bytes.NewReader([]byte(body)), "application/json", nil
	case "raw_text":
		return bytes.NewReader([]byte(stringValue(config, "raw_body", ""))), "text/plain; charset=utf-8", nil
	case "none":
		return bytes.NewReader(nil), "", nil
	default:
		return nil, "", fmt.Errorf("unsupported webhook body mode %q", mode)
	}
}

func eventQuery(event core.NotificationEvent) map[string]any {
	return map[string]any{"event_type": event.EventType, "monitor_name": event.MonitorName, "module_type": event.ModuleType, "summary": event.Summary, "triggered_at": event.TriggeredAt.Format(time.RFC3339)}
}

func addQuery(target *url.URL, source any) {
	values := target.Query()
	for key, items := range stringValues(source) {
		for _, item := range items {
			values.Add(key, item)
		}
	}
	target.RawQuery = values.Encode()
}

func applyHeaders(request *http.Request, source any) {
	for key, value := range stringValues(source) {
		if len(value) > 0 {
			request.Header.Set(key, value[0])
		}
	}
}

func validateMap(config map[string]any, key string) error {
	if value, ok := config[key]; ok && value != nil {
		if _, ok := value.(map[string]any); !ok {
			return fmt.Errorf("%s must be an object", key)
		}
	}
	return nil
}

func stringValues(value any) map[string][]string {
	result := map[string][]string{}
	items, ok := value.(map[string]any)
	if !ok {
		return result
	}
	for key, item := range items {
		switch typed := item.(type) {
		case []any:
			for _, value := range typed {
				result[key] = append(result[key], fmt.Sprint(value))
			}
		case string:
			result[key] = []string{typed}
		default:
			result[key] = []string{fmt.Sprint(typed)}
		}
	}
	return result
}

func addValues(target url.Values, source any) {
	for key, values := range stringValues(source) {
		for _, value := range values {
			target.Add(key, value)
		}
	}
}

func stringValue(config map[string]any, key, fallback string) string {
	if value, ok := config[key].(string); ok {
		return value
	}
	return fallback
}

func intValue(config map[string]any, key string, fallback int) int {
	if value, ok := config[key].(float64); ok {
		return int(value)
	}
	if value, ok := config[key].(int); ok {
		return value
	}
	return fallback
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
