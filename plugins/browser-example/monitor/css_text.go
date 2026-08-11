package browsermonitor

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/hanxuanyu/meerkit/sdk"
)

const cssTextModuleType = "browser-example-css-text"

type CSSTextConfig struct {
	pageConfig
	Selector string `json:"selector"`
}

type CSSTextModule struct{ moduleBase }

func NewCSSText(browser sdk.BrowserClient) *CSSTextModule {
	return &CSSTextModule{moduleBase{browser: browser, reuseNamespace: cssTextModuleType}}
}

func (m *CSSTextModule) Descriptor() sdk.ModuleDescriptor {
	properties := commonConfigProperties()
	properties["selector"] = map[string]any{"type": "string", "minLength": 1, "maxLength": 4096}
	parameters := commonParameters()
	parameters = append(parameters, sdk.ParameterDescriptor{Key: "selector", Label: "CSS Selector", Type: sdk.ParameterString, Required: true, Order: 20, FullWidth: true, Placeholder: "main h1"})
	fields := []sdk.ResultFieldDescriptor{
		{Name: "text", Label: "元素文本", Type: "text", Operators: stringOperators()},
		{Name: "selector", Label: "CSS Selector", Type: "string", Operators: stringOperators()},
		{Name: "tag_name", Label: "元素标签", Type: "string", Operators: stringOperators()},
		{Name: "title", Label: "页面标题", Type: "string", Operators: stringOperators()},
		{Name: "page_url", Label: "最终页面地址", Type: "string", Operators: stringOperators()},
		{Name: "reused_tab", Label: "复用了标签页", Type: "boolean", Operators: boolOperators()},
		{Name: "duration_ms", Label: "执行耗时", Type: "number", Unit: "ms", Operators: numberOperators()},
	}
	return sdk.ModuleDescriptor{
		Type: cssTextModuleType, Version: moduleVersion, ConfigVersion: configVersion, ResultSchemaVersion: resultSchemaVersion,
		Name: "Example - CSS 文本", Description: "示例模块：获取指定 CSS Selector 对应元素的文本。",
		ListSummary:  &sdk.ModuleListSummaryDescriptor{Fields: []string{"url", "selector"}, Separator: " · "},
		ConfigSchema: map[string]any{"type": "object", "required": []string{"url", "selector"}, "properties": properties}, Parameters: parameters,
		ResultSchema: resultSchema(fields), Fields: legacyFields(fields), ResultSets: []sdk.ResultSetDescriptor{{Key: "element", Label: "CSS 元素文本", Fields: fields}},
	}
}

func (m *CSSTextModule) ValidateConfig(raw json.RawMessage) error {
	var config CSSTextConfig
	if err := decodeConfig(raw, &config); err != nil {
		return err
	}
	if err := validatePageConfig(config.pageConfig, m.browser); err != nil {
		return err
	}
	if selector := strings.TrimSpace(config.Selector); selector == "" || len(selector) > 4096 {
		return errors.New("selector must contain between 1 and 4096 characters")
	}
	return nil
}

func (m *CSSTextModule) Execute(ctx context.Context, raw json.RawMessage) (sdk.Observation, error) {
	if err := m.ValidateConfig(raw); err != nil {
		return failedObservation("element", err.Error(), emptyCSSTextResult()), err
	}
	var config CSSTextConfig
	_ = json.Unmarshal(raw, &config)
	result, err := m.run(ctx, config.pageConfig, []sdk.BrowserAction{
		{ID: "wait", Type: "page.wait", Params: map[string]any{"selector": config.Selector, "timeout_ms": int(defaultTimeout.Milliseconds())}},
		{ID: "element", Type: "dom.query", Params: map[string]any{"selector": config.Selector, "max_length": 65536}},
	})
	if err != nil {
		return failedObservation("element", err.Error(), emptyCSSTextResult()), err
	}
	element, open := actionData(result.Actions, "element"), actionData(result.Actions, "open")
	text := stringValue(element, "text")
	value := map[string]any{"text": text, "selector": config.Selector, "tag_name": stringValue(element, "tag_name"), "title": stringValue(element, "title"), "page_url": stringValue(element, "url"), "reused_tab": boolValue(open, "reused"), "duration_ms": result.Duration}
	return sdk.Observation{Success: true, SchemaVersion: resultSchemaVersion, Result: value, ResultSets: map[string]map[string]any{"element": value}, Summary: "已获取 CSS 文本：" + summarize(text)}, nil
}

func emptyCSSTextResult() map[string]any {
	return map[string]any{"text": "", "selector": "", "tag_name": "", "title": "", "page_url": "", "reused_tab": false, "duration_ms": int64(0)}
}
