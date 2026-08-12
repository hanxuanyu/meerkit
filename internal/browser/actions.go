package browser

import (
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/hanxuanyu/meerkit/sdk"
)

type ActionParameterOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

type ActionParameterDefinition struct {
	Key         string                  `json:"key"`
	Label       string                  `json:"label"`
	Type        string                  `json:"type"`
	Required    bool                    `json:"required,omitempty"`
	Default     any                     `json:"default,omitempty"`
	Placeholder string                  `json:"placeholder,omitempty"`
	Description string                  `json:"description,omitempty"`
	Min         *float64                `json:"min,omitempty"`
	Max         *float64                `json:"max,omitempty"`
	Step        *float64                `json:"step,omitempty"`
	Wide        bool                    `json:"wide,omitempty"`
	Options     []ActionParameterOption `json:"options,omitempty"`
	VisibleWhen map[string]any          `json:"visible_when,omitempty"`
}

type ActionDefinition struct {
	Type                   string                      `json:"type"`
	Label                  string                      `json:"label"`
	Description            string                      `json:"description"`
	Category               string                      `json:"category"`
	CategoryLabel          string                      `json:"category_label"`
	Icon                   string                      `json:"icon"`
	Capability             string                      `json:"capability"`
	ResultType             string                      `json:"result_type"`
	TargetMode             string                      `json:"target_mode"`
	Destructive            bool                        `json:"destructive,omitempty"`
	DefaultContinueOnError bool                        `json:"default_continue_on_error,omitempty"`
	Parameters             []ActionParameterDefinition `json:"parameters"`
}

type ActionCatalog struct {
	Actions []ActionDefinition `json:"actions"`
}

type actionSpec struct {
	definition ActionDefinition
	validate   func(map[string]any) error
}

var actionSpecs = buildActionSpecs()

func BrowserActionCatalog() ActionCatalog {
	actions := make([]ActionDefinition, 0, len(actionSpecs))
	for _, spec := range actionSpecs {
		actions = append(actions, spec.definition)
	}
	sort.Slice(actions, func(i, j int) bool {
		if actions[i].Category == actions[j].Category {
			return actionOrder(actions[i].Type) < actionOrder(actions[j].Type)
		}
		return categoryOrder(actions[i].Category) < categoryOrder(actions[j].Category)
	})
	return ActionCatalog{Actions: actions}
}

func ValidateBrowserActionRequest(request sdk.BrowserActionRequest) error {
	if request.Target.TabID < 0 || request.Target.WindowID < 0 {
		return errors.New("browser tab_id and window_id cannot be negative")
	}
	action := request.Action
	spec, ok := actionSpecs[action.Type]
	if !ok {
		return fmt.Errorf("browser action has unsupported type %q", action.Type)
	}
	if len(action.ID) > 128 {
		return errors.New("browser action id cannot exceed 128 characters")
	}
	if spec.definition.TargetMode == "tab_required" && request.Target.TabID <= 0 {
		return fmt.Errorf("browser action %q requires tab_id", action.Type)
	}
	if err := spec.validate(action.Params); err != nil {
		return fmt.Errorf("browser action %q: %w", action.Type, err)
	}
	return nil
}

func actionCapability(actionType string) string {
	if spec, ok := actionSpecs[actionType]; ok {
		return spec.definition.Capability
	}
	return actionType
}

func buildActionSpecs() map[string]actionSpec {
	minZero, minOne, minHundred, min256, min1024 := number(0), number(1), number(100), number(256), number(1024)
	maxQuality, maxContent, maxWait := number(100), number(1048576), number(300000)
	stepThousand := number(1000)
	colors := options("grey", "blue", "red", "yellow", "green", "pink", "purple", "cyan", "orange")
	specs := []actionSpec{
		newAction("tab.open", "打开标签页", "在指定窗口或当前窗口创建新标签页。", "tab", "标签页", "panel-top-open", "tab", "window_optional", false, []ActionParameterDefinition{
			{Key: "url", Label: "页面地址", Type: "url", Default: "about:blank", Placeholder: "https://example.com", Wide: true},
			{Key: "active", Label: "前台打开", Type: "boolean", Default: true},
			{Key: "wait", Label: "等待加载完成", Type: "boolean", Default: true},
		}, validateTabOpen),
		newAction("tab.navigate", "导航标签页", "将指定标签页导航到对应地址。", "tab", "标签页", "globe", "tab", "tab_required", false, []ActionParameterDefinition{
			{Key: "url", Label: "导航地址", Type: "url", Required: true, Placeholder: "https://example.com", Wide: true},
			{Key: "wait", Label: "等待加载完成", Type: "boolean", Default: true},
		}, func(params map[string]any) error { return validateHTTPURL(stringParam(params, "url"), false) }),
		newAction("tab.group", "加入标签页分组", "将指定标签页加入或复用 Chrome 分组。", "tab", "标签页", "group", "tab", "tab_required", false, []ActionParameterDefinition{
			{Key: "title", Label: "分组名称", Type: "string", Required: true, Default: "Meerkit", Wide: true},
			{Key: "color", Label: "分组颜色", Type: "select", Default: "blue", Options: colors},
			{Key: "collapsed", Label: "折叠分组", Type: "boolean", Default: false},
			{Key: "reuse_group", Label: "复用同名分组", Type: "boolean", Default: true},
		}, validateTabGroup),
		newAction("tab.close", "关闭标签页", "关闭指定标签页。", "tab", "标签页", "trash-2", "tab", "tab_required", true, nil, noValidation),
		newAction("page.wait", "等待页面", "等待页面加载、CSS 元素出现或固定时长。", "page", "页面", "timer", "status", "tab_required", false, []ActionParameterDefinition{
			{Key: "mode", Label: "等待方式", Type: "select", Default: "load", Options: []ActionParameterOption{{Value: "load", Label: "页面加载"}, {Value: "selector", Label: "CSS 元素"}, {Value: "duration", Label: "固定时长"}}},
			{Key: "selector", Label: "CSS Selector", Type: "string", Placeholder: "#app, main", Wide: true, VisibleWhen: map[string]any{"mode": "selector"}},
			{Key: "timeout_ms", Label: "最长等待", Type: "number", Default: 60000, Min: minHundred, Max: maxWait, Step: stepThousand, VisibleWhen: map[string]any{"mode": "selector"}},
			{Key: "duration_ms", Label: "等待时长", Type: "number", Default: 1000, Min: minZero, Max: maxWait, Step: minHundred, VisibleWhen: map[string]any{"mode": "duration"}},
		}, validatePageWait),
		newAction("page.screenshot", "页面截图", "使用 CDP 截取指定标签页。", "page", "页面", "camera", "screenshot", "tab_required", false, []ActionParameterDefinition{
			{Key: "format", Label: "图片格式", Type: "select", Default: "png", Options: []ActionParameterOption{{Value: "png", Label: "PNG"}, {Value: "jpeg", Label: "JPEG"}}},
			{Key: "quality", Label: "JPEG 质量", Type: "number", Default: 90, Min: minOne, Max: maxQuality, VisibleWhen: map[string]any{"format": "jpeg"}},
			{Key: "full_page", Label: "完整页面", Type: "boolean", Default: false},
		}, validateScreenshot),
		newAction("dom.document", "获取页面 HTML", "读取指定标签页文档的完整 HTML。", "dom", "DOM", "file-code-2", "document", "tab_required", false, []ActionParameterDefinition{
			{Key: "max_length", Label: "最大 HTML 长度", Type: "number", Default: 262144, Min: min1024, Max: maxContent, Step: min1024},
		}, func(params map[string]any) error {
			return validateNumberRange(params, "max_length", 1024, 1048576, true)
		}),
		newAction("dom.query", "查询元素", "通过 CSS Selector 读取元素文本、HTML 和属性。", "dom", "DOM", "search", "element", "tab_required", false, []ActionParameterDefinition{
			{Key: "selector", Label: "CSS Selector", Type: "string", Required: true, Placeholder: "#app, main, [data-id]", Description: "最多 4096 个字符", Wide: true},
			{Key: "max_length", Label: "最大返回长度", Type: "number", Default: 65536, Min: min256, Max: maxContent, Step: min1024},
		}, validateDOMQuery),
		newAction("dom.click", "点击元素", "点击 CSS Selector 匹配的第一个元素。", "dom", "DOM", "mouse-pointer-click", "status", "tab_required", false, selectorParameters(), validateSelector),
		newAction("dom.input", "填写控件", "设置输入框、文本域、下拉框或可编辑元素的值。", "dom", "DOM", "keyboard", "status", "tab_required", false, []ActionParameterDefinition{
			{Key: "selector", Label: "CSS Selector", Type: "string", Required: true, Placeholder: "input[name=email]", Wide: true},
			{Key: "value", Label: "输入内容", Type: "textarea", Default: "", Wide: true},
		}, validateSelector),
		newAction("runtime.evaluate", "执行 JavaScript", "在页面主世界执行表达式并返回可序列化结果。", "runtime", "运行时", "code-2", "script", "tab_required", false, []ActionParameterDefinition{
			{Key: "expression", Label: "JavaScript 表达式", Type: "code", Required: true, Default: "({ title: document.title, url: location.href })", Wide: true},
		}, validateExpression),
	}
	result := make(map[string]actionSpec, len(specs))
	for _, spec := range specs {
		result[spec.definition.Type] = spec
	}
	return result
}

func newAction(actionType, label, description, category, categoryLabel, icon, resultType, targetMode string, destructive bool, parameters []ActionParameterDefinition, validate func(map[string]any) error) actionSpec {
	return actionSpec{definition: ActionDefinition{Type: actionType, Label: label, Description: description, Category: category, CategoryLabel: categoryLabel, Icon: icon, Capability: actionType, ResultType: resultType, TargetMode: targetMode, Destructive: destructive, Parameters: parameters}, validate: validate}
}

func noValidation(map[string]any) error { return nil }

func validateTabOpen(params map[string]any) error {
	if value := stringParam(params, "url"); value != "" {
		if err := validateHTTPURL(value, true); err != nil {
			return err
		}
	}
	return nil
}

func validateTabGroup(params map[string]any) error {
	if value := strings.TrimSpace(stringParam(params, "title")); value == "" || len(value) > 128 {
		return errors.New("title must contain between 1 and 128 characters")
	}
	color := stringParam(params, "color")
	if color != "" && !contains([]string{"grey", "blue", "red", "yellow", "green", "pink", "purple", "cyan", "orange"}, color) {
		return errors.New("color is invalid")
	}
	return nil
}

func validatePageWait(params map[string]any) error {
	mode := stringParam(params, "mode")
	if mode == "" {
		if stringParam(params, "selector") != "" {
			mode = "selector"
		} else if _, ok := params["duration_ms"]; ok {
			mode = "duration"
		} else {
			mode = "load"
		}
	}
	switch mode {
	case "load":
		return nil
	case "selector":
		if err := validateSelector(params); err != nil {
			return err
		}
		return validateNumberRange(params, "timeout_ms", 100, 300000, true)
	case "duration":
		return validateNumberRange(params, "duration_ms", 0, 300000, false)
	default:
		return errors.New("mode must be load, selector, or duration")
	}
}

func validateScreenshot(params map[string]any) error {
	format := stringParam(params, "format")
	if format != "" && format != "png" && format != "jpeg" {
		return errors.New("format must be png or jpeg")
	}
	return validateNumberRange(params, "quality", 1, 100, true)
}

func validateDOMQuery(params map[string]any) error {
	if err := validateSelector(params); err != nil {
		return err
	}
	return validateNumberRange(params, "max_length", 256, 1048576, true)
}

func validateSelector(params map[string]any) error {
	selector := stringParam(params, "selector")
	if selector == "" || len(selector) > 4096 {
		return errors.New("selector must contain between 1 and 4096 characters")
	}
	return nil
}

func validateExpression(params map[string]any) error {
	expression := stringParam(params, "expression")
	if expression == "" || len(expression) > 100000 {
		return errors.New("expression must contain between 1 and 100000 characters")
	}
	return nil
}

func validateNetworkCapture(params map[string]any) error {
	if len(stringParam(params, "capture_id")) > 128 || len(stringParam(params, "url_contains")) > 4096 || len(stringParam(params, "resource_type")) > 64 {
		return errors.New("network capture parameters are too long")
	}
	return validateNumberRange(params, "max_body_bytes", 1024, 1048576, true)
}

func validateHTTPURL(value string, allowBlank bool) error {
	if value == "" && allowBlank {
		return nil
	}
	parsed, err := url.Parse(value)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https" && !(allowBlank && value == "about:blank")) || (parsed.Host == "" && value != "about:blank") {
		return errors.New("url must be a valid HTTP or HTTPS URL")
	}
	return nil
}

func validateNumberRange(params map[string]any, key string, minimum, maximum float64, optional bool) error {
	value, ok := numberParam(params, key)
	if !ok {
		if optional {
			return nil
		}
		return fmt.Errorf("%s is required", key)
	}
	if value < minimum || value > maximum {
		return fmt.Errorf("%s must be between %v and %v", key, minimum, maximum)
	}
	return nil
}

func stringParam(params map[string]any, key string) string {
	value, _ := params[key].(string)
	return strings.TrimSpace(value)
}

func numberParam(params map[string]any, key string) (float64, bool) {
	switch value := params[key].(type) {
	case int:
		return float64(value), true
	case int64:
		return float64(value), true
	case float64:
		return value, true
	case float32:
		return float64(value), true
	default:
		return 0, false
	}
}

func selectorParameters() []ActionParameterDefinition {
	return []ActionParameterDefinition{{Key: "selector", Label: "CSS Selector", Type: "string", Required: true, Placeholder: "#app, button[type=submit]", Wide: true}}
}

func options(values ...string) []ActionParameterOption {
	result := make([]ActionParameterOption, 0, len(values))
	for _, value := range values {
		label := value
		if label == "" {
			label = "全部"
		}
		result = append(result, ActionParameterOption{Value: value, Label: label})
	}
	return result
}

func number(value float64) *float64 { return &value }

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func categoryOrder(category string) int {
	switch category {
	case "tab":
		return 0
	case "page":
		return 1
	case "dom":
		return 2
	default:
		return 3
	}
}

func actionOrder(actionType string) int {
	for index, value := range []string{"tab.open", "tab.navigate", "tab.group", "tab.close", "page.wait", "page.screenshot", "dom.document", "dom.query", "dom.click", "dom.input", "runtime.evaluate"} {
		if value == actionType {
			return index
		}
	}
	return 100
}
