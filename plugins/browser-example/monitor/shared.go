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
	moduleVersion       = "1"
	configVersion       = "1"
	resultSchemaVersion = "1"
	defaultTimeout      = 60 * time.Second
)

type pageConfig struct {
	URL          string `json:"url"`
	AlwaysNewTab bool   `json:"always_new_tab"`
	TabReuseKey  string `json:"tab_reuse_key,omitempty"`
}

type moduleBase struct {
	browser        sdk.BrowserClient
	reuseNamespace string
}

func validatePageConfig(config pageConfig, browser sdk.BrowserClient) error {
	parsed, err := url.Parse(strings.TrimSpace(config.URL))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return errors.New("url must be a valid HTTP or HTTPS URL")
	}
	if browser == nil {
		return errors.New("browser capability is unavailable")
	}
	if len(strings.TrimSpace(config.TabReuseKey)) > 256 {
		return errors.New("tab_reuse_key cannot exceed 256 characters")
	}
	return nil
}

func (m moduleBase) run(ctx context.Context, config pageConfig, actions []sdk.BrowserAction, captures ...sdk.BrowserNetworkCapture) (sdk.BrowserRunResult, error) {
	targetURL := strings.TrimSpace(config.URL)
	params := map[string]any{
		"url":         targetURL,
		"active":      false,
		"reuse":       !config.AlwaysNewTab,
		"reuse_key":   m.reusablePageKey(config),
		"group_title": "Meerkit",
	}
	actions = append([]sdk.BrowserAction{
		{ID: "open", Type: "tab.open", Params: params},
		{ID: "group", Type: "tab.group", Params: map[string]any{"title": "Meerkit", "color": "blue", "collapsed": false, "reuse_group": true}},
	}, actions...)
	request := sdk.BrowserRunRequest{
		TimeoutMS:       int(defaultTimeout.Milliseconds()),
		KeepTab:         !config.AlwaysNewTab,
		Actions:         actions,
		NetworkCaptures: captures,
	}
	executionContext, cancel := context.WithTimeout(ctx, defaultTimeout+5*time.Second)
	defer cancel()
	return m.browser.Run(executionContext, request)
}

func (m moduleBase) reusablePageKey(config pageConfig) string {
	value := strings.TrimSpace(config.TabReuseKey)
	if value != "" {
		return m.reuseNamespace + ":" + value
	}
	value = strings.TrimSpace(config.URL)
	parsed, err := url.Parse(value)
	if err != nil {
		return m.reuseNamespace + ":" + value
	}
	parsed.Fragment = ""
	return m.reuseNamespace + ":" + parsed.String()
}

func commonConfigProperties() map[string]any {
	return map[string]any{
		"url":            map[string]any{"type": "string", "format": "uri"},
		"always_new_tab": map[string]any{"type": "boolean", "default": false},
		"tab_reuse_key":  map[string]any{"type": "string", "maxLength": 256},
	}
}

func commonParameters() []sdk.ParameterDescriptor {
	return []sdk.ParameterDescriptor{
		{Key: "url", Label: "页面地址", Type: sdk.ParameterURL, Required: true, Order: 10, FullWidth: true, Placeholder: "https://example.com"},
		{Key: "always_new_tab", Label: "每次使用新标签页", Type: sdk.ParameterBoolean, Default: false, Order: 900, Description: "关闭时优先刷新此前为该地址保留的标签页，适合需要用户登录的页面。"},
		{Key: "tab_reuse_key", Label: "标签页复用标识", Type: sdk.ParameterString, Order: 910, FullWidth: true, Description: "可选。用于区分同一模块、同一地址的多个登录会话；保持不变即可跨跳转复用。", Placeholder: "例如 account-a", VisibleWhen: []sdk.ParameterCondition{{Field: "always_new_tab", Operator: "equals", Value: false}}},
	}
}

func decodeConfig(raw json.RawMessage, target any) error {
	if err := json.Unmarshal(raw, target); err != nil {
		return fmt.Errorf("invalid browser example config: %w", err)
	}
	return nil
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

func boolValue(values map[string]any, key string) bool {
	value, _ := values[key].(bool)
	return value
}

func summarize(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	if len([]rune(value)) > 80 {
		return string([]rune(value)[:80]) + "..."
	}
	if value == "" {
		return "内容为空"
	}
	return value
}

func failedObservation(resultSet, message string, result map[string]any) sdk.Observation {
	if message == "" {
		message = "浏览器示例采集失败"
	}
	return sdk.Observation{
		Success: false, SchemaVersion: resultSchemaVersion, Result: result,
		ResultSets: map[string]map[string]any{resultSet: result}, Summary: message,
		ErrorCode: "browser_example_execution_failed", ErrorMessage: message,
	}
}
