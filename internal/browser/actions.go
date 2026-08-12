package browser

import (
	"encoding/json"
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
	Sensitive              bool                      `json:"sensitive,omitempty"`
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
	if spec.definition.TargetMode == "window_required" && request.Target.WindowID <= 0 {
		return fmt.Errorf("browser action %q requires window_id", action.Type)
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
		windowOpenAction(),
		windowFocusAction(),
		windowStateAction(),
		windowResizeAction(),
		windowCloseAction(),
		tabActivateAction(),
		tabReloadAction(),
		tabBackAction(),
		tabForwardAction(),
		tabDuplicateAction(),
		tabMoveAction(),
		tabPinAction(),
		tabMuteAction(),
		tabUngroupAction(),
		tabZoomAction(),
		pageWaitAction(),
		pageScreenshotAction(),
		pageInfoAction(),
		pageScrollAction(),
		domDocumentAction(),
		domQueryAction(),
		domQueryAllAction(),
		domClickAction(),
		domInputAction(),
		domFocusAction(),
		domCheckAction(),
		domSelectAction(),
		domScrollIntoViewAction(),
		inputClickAction(),
		inputHoverAction(),
		inputTypeAction(),
		inputKeyAction(),
		inputWheelAction(),
		cookieListAction(),
		cookieSetAction(),
		cookieDeleteAction(),
		cookieClearAction(),
		storageGetAction(),
		storageSetAction(),
		storageRemoveAction(),
		storageClearAction(),
		runtimeEvaluateAction(),
	}
	return buildActionSpecMap(specs)
}

func buildActionSpecMap(specs []actionSpec) map[string]actionSpec {
	result := make(map[string]actionSpec, len(specs))
	for _, spec := range specs {
		if _, exists := result[spec.definition.Type]; exists {
			panic(fmt.Sprintf("duplicate browser action type %q", spec.definition.Type))
		}
		result[spec.definition.Type] = spec
	}
	return result
}

func newAction(actionType, label, description, category, categoryLabel, icon, resultType, targetMode string, destructive bool, parameters []sdk.ParameterDescriptor, validate func(map[string]any) error) actionSpec {
	return actionSpec{definition: ActionDefinition{Type: actionType, Label: label, Description: description, Category: category, CategoryLabel: categoryLabel, Icon: icon, Capability: actionType, ResultType: resultType, TargetMode: targetMode, Destructive: destructive, Parameters: parameters}, validate: validate}
}

func sensitiveAction(spec actionSpec) actionSpec {
	spec.definition.Sensitive = true
	return spec
}

func noValidation(map[string]any) error { return nil }

func validateTabOpen(params map[string]any) error {
	if value := stringParam(params, "url"); value != "" {
		if err := validateHTTPURL(value, true); err != nil {
			return err
		}
	}
	return validateOptionalBooleans(params, "active", "wait")
}

func validateTabGroup(params map[string]any) error {
	if value := strings.TrimSpace(stringParam(params, "title")); value == "" || len(value) > 128 {
		return errors.New("title must contain between 1 and 128 characters")
	}
	color := stringParam(params, "color")
	if color != "" && !contains([]string{"grey", "blue", "red", "yellow", "green", "pink", "purple", "cyan", "orange"}, color) {
		return errors.New("color is invalid")
	}
	return validateOptionalBooleans(params, "collapsed", "reuse_group")
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
	if err := validateNumberRange(params, "quality", 1, 100, true); err != nil {
		return err
	}
	return validateOptionalBooleans(params, "full_page")
}

func validateWindowOpen(params map[string]any) error {
	if value := stringParam(params, "url"); value != "" {
		if err := validateHTTPURL(value, true); err != nil {
			return err
		}
	}
	if err := validateChoice(params, "type", false, "normal", "popup"); err != nil {
		return err
	}
	if err := validateChoice(params, "state", false, "normal", "minimized", "maximized", "fullscreen"); err != nil {
		return err
	}
	return validateOptionalNumbers(params, map[string][2]float64{"width": {100, 10000}, "height": {100, 10000}, "left": {-10000, 10000}, "top": {-10000, 10000}})
}

func validateWindowResize(params map[string]any) error {
	if err := validateNumberRange(params, "width", 100, 10000, false); err != nil {
		return err
	}
	if err := validateNumberRange(params, "height", 100, 10000, false); err != nil {
		return err
	}
	return validateOptionalNumbers(params, map[string][2]float64{"left": {-10000, 10000}, "top": {-10000, 10000}})
}

func validateTabMove(params map[string]any) error {
	return validateOptionalNumbers(params, map[string][2]float64{"destination_window_id": {1, 1<<31 - 1}, "index": {-1, 100000}})
}

func validatePageScroll(params map[string]any) error {
	if err := validateChoice(params, "mode", false, "absolute", "relative", "top", "bottom"); err != nil {
		return err
	}
	if err := validateChoice(params, "behavior", false, "auto", "smooth"); err != nil {
		return err
	}
	return validateOptionalNumbers(params, map[string][2]float64{"x": {-10000000, 10000000}, "y": {-10000000, 10000000}})
}

func validateDOMQueryAll(params map[string]any) error {
	if err := validateSelector(params); err != nil {
		return err
	}
	if err := validateNumberRange(params, "limit", 1, 500, true); err != nil {
		return err
	}
	return validateNumberRange(params, "max_length", 64, 65536, true)
}

func validateDOMSelect(params map[string]any) error {
	if err := validateSelector(params); err != nil {
		return err
	}
	return validateRequiredString(params, "value", 1048576)
}

func validateDOMScrollIntoView(params map[string]any) error {
	if err := validateSelector(params); err != nil {
		return err
	}
	if err := validateChoice(params, "block", false, "start", "center", "end", "nearest"); err != nil {
		return err
	}
	if err := validateChoice(params, "inline", false, "start", "center", "end", "nearest"); err != nil {
		return err
	}
	return validateChoice(params, "behavior", false, "auto", "smooth")
}

func validateInputClick(params map[string]any) error {
	if err := validateSelector(params); err != nil {
		return err
	}
	if err := validateChoice(params, "button", false, "left", "right", "middle"); err != nil {
		return err
	}
	return validateNumberRange(params, "click_count", 1, 3, true)
}

func validateInputType(params map[string]any) error {
	if err := validateSelector(params); err != nil {
		return err
	}
	if len(stringParamRaw(params, "text")) > 100000 {
		return errors.New("text cannot exceed 100000 characters")
	}
	if err := validateNumberRange(params, "interval_ms", 0, 5000, true); err != nil {
		return err
	}
	return validateOptionalBooleans(params, "clear")
}

func validateInputKey(params map[string]any) error {
	if err := validateRequiredString(params, "key", 128); err != nil {
		return err
	}
	for _, key := range []string{"code", "text", "modifiers"} {
		if len(stringParamRaw(params, key)) > 256 {
			return fmt.Errorf("%s cannot exceed 256 characters", key)
		}
	}
	return validateNumberRange(params, "repeat", 1, 100, true)
}

func validateInputWheel(params map[string]any) error {
	if selector := stringParam(params, "selector"); len(selector) > 4096 {
		return errors.New("selector cannot exceed 4096 characters")
	}
	return validateOptionalNumbers(params, map[string][2]float64{"delta_x": {-1000000, 1000000}, "delta_y": {-1000000, 1000000}})
}

func validateCookieSet(params map[string]any) error {
	if err := validateRequiredString(params, "name", 256); err != nil {
		return err
	}
	if len(stringParamRaw(params, "value")) > 16384 {
		return errors.New("value cannot exceed 16384 characters")
	}
	for _, key := range []string{"domain", "path"} {
		if len(stringParamRaw(params, key)) > 2048 {
			return fmt.Errorf("%s cannot exceed 2048 characters", key)
		}
	}
	if err := validateChoice(params, "same_site", false, "unspecified", "no_restriction", "lax", "strict"); err != nil {
		return err
	}
	if err := validateNumberRange(params, "expiration_date", 0, 253402300799, true); err != nil {
		return err
	}
	return validateOptionalBooleans(params, "secure", "http_only")
}

func validateStorageGet(params map[string]any) error {
	if err := validateChoice(params, "area", false, "local", "session"); err != nil {
		return err
	}
	if len(stringParamRaw(params, "key")) > 1048576 {
		return errors.New("key cannot exceed 1048576 characters")
	}
	return validateNumberRange(params, "max_value_length", 1, 1048576, true)
}

func validateStorageMutation(params map[string]any) error {
	if err := validateChoice(params, "area", false, "local", "session"); err != nil {
		return err
	}
	if err := validateRequiredString(params, "key", 1048576); err != nil {
		return err
	}
	if len(stringParamRaw(params, "value")) > 1048576 {
		return errors.New("value cannot exceed 1048576 characters")
	}
	return nil
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
	return strings.TrimSpace(stringParamRaw(params, key))
}

func stringParamRaw(params map[string]any, key string) string {
	value, _ := params[key].(string)
	return value
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
	case json.Number:
		number, err := value.Float64()
		return number, err == nil
	default:
		return 0, false
	}
}

func selectorParameters() []sdk.ParameterDescriptor {
	return genericSelectorParameters("点击页面中第一个匹配元素。")
}

func genericSelectorParameters(description string) []sdk.ParameterDescriptor {
	return []sdk.ParameterDescriptor{{Key: "selector", Label: "CSS Selector", Description: description, Type: sdk.ParameterString, Required: true, Placeholder: "#app, button[type=submit]", FullWidth: true}}
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

func labeledOptions(values ...string) []sdk.ParameterOption {
	result := make([]sdk.ParameterOption, 0, len(values)/2)
	for index := 0; index+1 < len(values); index += 2 {
		result = append(result, sdk.ParameterOption{Value: values[index], Label: values[index+1]})
	}
	return result
}

func windowStateOptions() []sdk.ParameterOption {
	return labeledOptions("normal", "普通", "minimized", "最小化", "maximized", "最大化", "fullscreen", "全屏")
}

func storageAreaOptions() []sdk.ParameterOption {
	return labeledOptions("local", "localStorage", "session", "sessionStorage")
}

func validateChoice(params map[string]any, key string, required bool, values ...string) error {
	value := stringParam(params, key)
	if value == "" && !required {
		return nil
	}
	if value == "" || !contains(values, value) {
		return fmt.Errorf("%s must be one of %s", key, strings.Join(values, ", "))
	}
	return nil
}

func validateRequiredString(params map[string]any, key string, maximum int) error {
	value := stringParam(params, key)
	if value == "" || len(value) > maximum {
		return fmt.Errorf("%s must contain between 1 and %d characters", key, maximum)
	}
	return nil
}

func validateOptionalNumbers(params map[string]any, ranges map[string][2]float64) error {
	for key, bounds := range ranges {
		if _, exists := params[key]; !exists || params[key] == nil || params[key] == "" {
			continue
		}
		if err := validateNumberRange(params, key, bounds[0], bounds[1], false); err != nil {
			return err
		}
	}
	return nil
}

func validateOptionalBooleans(params map[string]any, keys ...string) error {
	for _, key := range keys {
		value, exists := params[key]
		if !exists || value == nil {
			continue
		}
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("%s must be a boolean", key)
		}
	}
	return nil
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
	case "window":
		return 0
	case "tab":
		return 1
	case "page":
		return 2
	case "dom":
		return 3
	case "input":
		return 4
	case "cookie":
		return 5
	case "storage":
		return 6
	default:
		return 7
	}
}

func actionOrder(actionType string) int {
	for index, value := range []string{
		"window.open", "window.focus", "window.state", "window.resize", "window.close",
		"tab.open", "tab.activate", "tab.navigate", "tab.reload", "tab.back", "tab.forward", "tab.duplicate", "tab.move", "tab.pin", "tab.mute", "tab.group", "tab.ungroup", "tab.zoom", "tab.close",
		"page.info", "page.wait", "page.scroll", "page.screenshot",
		"dom.document", "dom.query", "dom.query_all", "dom.focus", "dom.click", "dom.input", "dom.check", "dom.select", "dom.scroll_into_view",
		"input.click", "input.hover", "input.type", "input.key", "input.wheel",
		"cookie.list", "cookie.set", "cookie.delete", "cookie.clear",
		"storage.get", "storage.set", "storage.remove", "storage.clear",
		"runtime.evaluate",
	} {
		if value == actionType {
			return index
		}
	}
	return 100
}
