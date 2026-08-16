package browsermonitor

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/hanxuanyu/meerkit/sdk"
)

const elementModuleType = "browser-example-element"

type ElementConfig struct {
	pageConfig
	Selector  string `json:"selector"`
	MaxLength int    `json:"max_length"`
}

type ElementModule struct{ moduleBase }

func NewElement(browser sdk.BrowserClient) *ElementModule {
	return newElement(newBrowserWorkspace(browser))
}

func newElement(workspace *browserWorkspace) *ElementModule {
	return &ElementModule{moduleBase{workspace: workspace}}
}

func (m *ElementModule) Descriptor() sdk.ModuleDescriptor {
	properties := commonConfigProperties()
	properties["selector"] = map[string]any{"type": "string", "minLength": 1, "maxLength": 4096}
	properties["max_length"] = map[string]any{"type": "integer", "default": 65536, "minimum": 256, "maximum": 1048576}
	parameters := append(commonParameters(),
		sdk.ParameterDescriptor{Key: "selector", Label: "CSS Selector", Type: sdk.ParameterCSSSelector, Required: true, Order: 100, FullWidth: true, Placeholder: "main h1", Description: "读取页面中第一个匹配元素。", SelectorCandidates: &sdk.SelectorCandidateDescriptor{Queries: []string{"main", "article", "h1", "h2", "[data-testid]", "input", "button", "a"}, Limit: 80}},
		sdk.ParameterDescriptor{Key: "max_length", Label: "最大内容长度", Type: sdk.ParameterInteger, Default: 65536, Minimum: sdk.Float64(256), Maximum: sdk.Float64(1048576), Unit: "字符", Order: 110},
	)
	fields := []sdk.ResultFieldDescriptor{
		{Name: "text", Label: "元素文本", Type: "text", Operators: stringOperators()},
		{Name: "html", Label: "元素 HTML", Type: "text", Operators: stringOperators()},
		{Name: "value", Label: "控件值", Type: "string", Operators: stringOperators()},
		{Name: "attributes", Label: "元素属性", Type: "map", Operators: []string{"exists", "contains", "changed"}},
		{Name: "tag_name", Label: "元素标签", Type: "string", Operators: stringOperators()},
		{Name: "visible", Label: "元素可见", Type: "boolean", Operators: boolOperators()},
		{Name: "truncated", Label: "内容已截断", Type: "boolean", Operators: boolOperators()},
		{Name: "title", Label: "页面标题", Type: "string", Operators: stringOperators()},
		{Name: "page_url", Label: "最终页面地址", Type: "string", Operators: stringOperators()},
		{Name: "tab_id", Label: "标签页 ID", Type: "number", Operators: numberOperators()},
		{Name: "tab_reused", Label: "复用标签页", Type: "boolean", Operators: boolOperators()},
		{Name: "tab_refreshed", Label: "执行前已刷新", Type: "boolean", Operators: boolOperators()},
		{Name: "tab_kept_open", Label: "标签页保持打开", Type: "boolean", Operators: boolOperators()},
		{Name: "duration_ms", Label: "执行耗时", Type: "number", Unit: "ms", Operators: numberOperators()},
	}
	return sdk.ModuleDescriptor{
		Type: elementModuleType, Version: moduleVersion, ConfigVersion: configVersion, ResultSchemaVersion: resultSchemaVersion,
		Name: "Example - CSS 元素内容", Description: "读取 CSS Selector 匹配元素的文本、HTML、控件值和属性。",
		ListSummary:  &sdk.ModuleListSummaryDescriptor{Fields: []string{"url", "selector"}, Separator: " · "},
		ConfigSchema: map[string]any{"type": "object", "required": []string{"url", "selector"}, "properties": properties}, Parameters: parameters,
		ResultSchema: resultSchema(fields), Fields: legacyFields(fields), ResultSets: []sdk.ResultSetDescriptor{{Key: "element", Label: "CSS 元素内容", Fields: fields}},
	}
}

func (m *ElementModule) ValidateConfig(raw json.RawMessage) error {
	var config ElementConfig
	if err := decodeConfig(raw, &config); err != nil {
		return err
	}
	if err := validatePageConfig(config.pageConfig, m.workspace.browser); err != nil {
		return err
	}
	if selector := strings.TrimSpace(config.Selector); selector == "" || len(selector) > 4096 {
		return errors.New("selector must contain between 1 and 4096 characters")
	}
	if config.MaxLength != 0 && (config.MaxLength < 256 || config.MaxLength > 1048576) {
		return errors.New("max_length must be between 256 and 1048576")
	}
	return nil
}

func (m *ElementModule) Execute(ctx context.Context, raw json.RawMessage) (sdk.Observation, error) {
	if err := m.ValidateConfig(raw); err != nil {
		return failedObservation("element", err.Error(), emptyElementResult()), err
	}
	var config ElementConfig
	_ = json.Unmarshal(raw, &config)
	if config.MaxLength == 0 {
		config.MaxLength = 65536
	}
	result, err := m.workspace.run(ctx, config.pageConfig, []sdk.BrowserAction{
		{ID: "wait", Type: "page.wait", Params: map[string]any{"mode": "visible", "selector": config.Selector, "timeout_ms": int(defaultTimeout.Milliseconds())}},
		{ID: "element", Type: "dom.query", Params: map[string]any{"selector": config.Selector, "max_length": config.MaxLength}},
	}, nil)
	if err != nil {
		return failedObservation("element", err.Error(), emptyElementResult()), err
	}
	element := actionData(result.Actions, "element")
	text := stringValue(element, "text")
	value := map[string]any{
		"text": text, "html": stringValue(element, "html"), "value": stringValue(element, "value"), "attributes": mapValue(element, "attributes"),
		"tag_name": stringValue(element, "tag_name"), "visible": boolValue(element, "visible"), "truncated": boolValue(element, "truncated"),
		"title": stringValue(element, "title"), "page_url": stringValue(element, "url"), "tab_id": result.Target.TabID,
		"tab_reused": result.Reused, "tab_refreshed": result.Refreshed, "tab_kept_open": result.KeptOpen, "duration_ms": result.Duration,
	}
	return sdk.Observation{Success: true, SchemaVersion: resultSchemaVersion, Result: value, ResultSets: map[string]map[string]any{"element": value}, Summary: "已获取 CSS 元素内容：" + summarize(text)}, nil
}

func emptyElementResult() map[string]any {
	return map[string]any{"text": "", "html": "", "value": "", "attributes": map[string]any{}, "tag_name": "", "visible": false, "truncated": false, "title": "", "page_url": "", "tab_id": 0, "tab_reused": false, "tab_refreshed": false, "tab_kept_open": false, "duration_ms": int64(0)}
}
