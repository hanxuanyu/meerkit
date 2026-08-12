package browsermonitor

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hanxuanyu/meerkit/sdk"
)

const htmlModuleType = "browser-example-html"

type HTMLConfig struct {
	pageConfig
	MaxLength int `json:"max_length"`
}

type HTMLModule struct{ moduleBase }

func NewHTML(browser sdk.BrowserClient) *HTMLModule {
	return &HTMLModule{moduleBase{browser: browser}}
}

func (m *HTMLModule) Descriptor() sdk.ModuleDescriptor {
	properties := commonConfigProperties()
	properties["max_length"] = map[string]any{"type": "integer", "default": 262144, "minimum": 1024, "maximum": 1048576}
	parameters := commonParameters()
	parameters = append(parameters, sdk.ParameterDescriptor{Key: "max_length", Label: "最大 HTML 长度", Type: sdk.ParameterInteger, Default: 262144, Minimum: sdk.Float64(1024), Maximum: sdk.Float64(1048576), Unit: "字符", Order: 20})
	fields := []sdk.ResultFieldDescriptor{
		{Name: "html", Label: "页面 HTML", Type: "text", Operators: stringOperators()},
		{Name: "title", Label: "页面标题", Type: "string", Operators: stringOperators()},
		{Name: "page_url", Label: "最终页面地址", Type: "string", Operators: stringOperators()},
		{Name: "truncated", Label: "内容已截断", Type: "boolean", Operators: boolOperators()},
		{Name: "duration_ms", Label: "执行耗时", Type: "number", Unit: "ms", Operators: numberOperators()},
	}
	return sdk.ModuleDescriptor{
		Type: htmlModuleType, Version: moduleVersion, ConfigVersion: configVersion, ResultSchemaVersion: resultSchemaVersion,
		Name: "Example - 页面 HTML", Description: "示例模块：获取页面最终 DOM 的完整 HTML。",
		ListSummary:  &sdk.ModuleListSummaryDescriptor{Fields: []string{"url"}},
		ConfigSchema: map[string]any{"type": "object", "required": []string{"url"}, "properties": properties}, Parameters: parameters,
		ResultSchema: resultSchema(fields), Fields: legacyFields(fields), ResultSets: []sdk.ResultSetDescriptor{{Key: "page", Label: "页面 HTML", Fields: fields}},
	}
}

func (m *HTMLModule) ValidateConfig(raw json.RawMessage) error {
	var config HTMLConfig
	if err := decodeConfig(raw, &config); err != nil {
		return err
	}
	if err := validatePageConfig(config.pageConfig, m.browser); err != nil {
		return err
	}
	if config.MaxLength != 0 && (config.MaxLength < 1024 || config.MaxLength > 1048576) {
		return fmt.Errorf("max_length must be between 1024 and 1048576")
	}
	return nil
}

func (m *HTMLModule) Execute(ctx context.Context, raw json.RawMessage) (sdk.Observation, error) {
	if err := m.ValidateConfig(raw); err != nil {
		return failedObservation("page", err.Error(), emptyHTMLResult()), err
	}
	var config HTMLConfig
	_ = json.Unmarshal(raw, &config)
	if config.MaxLength == 0 {
		config.MaxLength = 262144
	}
	result, err := m.run(ctx, config.pageConfig, []sdk.BrowserAction{{ID: "document", Type: "dom.document", Params: map[string]any{"max_length": config.MaxLength}}}, nil)
	if err != nil {
		return failedObservation("page", err.Error(), emptyHTMLResult()), err
	}
	document := actionData(result.Actions, "document")
	value := map[string]any{"html": stringValue(document, "html"), "title": stringValue(document, "title"), "page_url": stringValue(document, "url"), "truncated": boolValue(document, "truncated"), "duration_ms": result.Duration}
	return sdk.Observation{Success: true, SchemaVersion: resultSchemaVersion, Result: value, ResultSets: map[string]map[string]any{"page": value}, Summary: fmt.Sprintf("已获取页面 HTML：%s", value["page_url"])}, nil
}

func emptyHTMLResult() map[string]any {
	return map[string]any{"html": "", "title": "", "page_url": "", "truncated": false, "duration_ms": int64(0)}
}
