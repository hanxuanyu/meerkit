package browser

import (
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/hanxuanyu/meerkit/sdk"
)

type ActionDefinition struct {
	Type                   string                    `json:"type"`
	Label                  string                    `json:"label"`
	Description            string                    `json:"description"`
	Category               string                    `json:"category"`
	CategoryLabel          string                    `json:"category_label"`
	Icon                   string                    `json:"icon"`
	Capability             string                    `json:"capability"`
	ResultType             string                    `json:"result_type"`
	TargetMode             string                    `json:"target_mode"`
	Destructive            bool                      `json:"destructive,omitempty"`
	DefaultContinueOnError bool                      `json:"default_continue_on_error,omitempty"`
	Parameters             []sdk.ParameterDescriptor `json:"parameters"`
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
		definition := spec.definition
		if definition.Parameters == nil {
			definition.Parameters = []sdk.ParameterDescriptor{}
		}
		actions = append(actions, definition)
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
	specs := []actionSpec{
		tabOpenAction(),
		tabNavigateAction(),
		tabGroupAction(),
		tabCloseAction(),
		pageWaitAction(),
		pageScreenshotAction(),
		domDocumentAction(),
		domQueryAction(),
		domClickAction(),
		domInputAction(),
		runtimeEvaluateAction(),
	}
	result := make(map[string]actionSpec, len(specs))
	for _, spec := range specs {
		result[spec.definition.Type] = spec
	}
	return result
}

func newAction(actionType, label, description, category, categoryLabel, icon, resultType, targetMode string, destructive bool, parameters []sdk.ParameterDescriptor, validate func(map[string]any) error) actionSpec {
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
	if format != "" && format != "png" && format != "jpeg" && format != "webp" {
		return errors.New("format must be png, jpeg, or webp")
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

func selectorParameters() []sdk.ParameterDescriptor {
	return []sdk.ParameterDescriptor{{Key: "selector", Label: "CSS Selector", Description: "点击页面中第一个匹配元素。", Type: sdk.ParameterString, Required: true, Placeholder: "#app, button[type=submit]", FullWidth: true}}
}

func options(values ...string) []sdk.ParameterOption {
	result := make([]sdk.ParameterOption, 0, len(values))
	for _, value := range values {
		label := value
		if label == "" {
			label = "全部"
		}
		result = append(result, sdk.ParameterOption{Value: value, Label: label})
	}
	return result
}

func visibleWhen(field string, value any) []sdk.ParameterCondition {
	return []sdk.ParameterCondition{{Field: field, Operator: "equals", Value: value}}
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
