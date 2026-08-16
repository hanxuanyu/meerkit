package browsermonitor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/hanxuanyu/meerkit/sdk"
)

const (
	moduleVersion       = "1"
	configVersion       = "1"
	resultSchemaVersion = "1"
	defaultTimeout      = 60 * time.Second
	retainedGroupTitle  = "Meerkit Examples"
)

type pageConfig struct {
	URL         string `json:"url"`
	KeepTabOpen bool   `json:"keep_tab_open"`
	BypassCache bool   `json:"bypass_cache"`
}

type browserExecution struct {
	Target    sdk.BrowserTarget
	Duration  int64
	Actions   []sdk.BrowserActionResult
	Network   []sdk.BrowserNetworkResult
	Reused    bool
	Refreshed bool
	KeptOpen  bool
}

type browserWorkspace struct {
	browser sdk.BrowserClient
	mu      sync.Mutex
	tabs    map[string]sdk.BrowserTarget
}

type moduleBase struct{ workspace *browserWorkspace }

func NewModules(browser sdk.BrowserClient) []sdk.Module {
	workspace := newBrowserWorkspace(browser)
	return []sdk.Module{newElement(workspace), newResponse(workspace)}
}

func newBrowserWorkspace(browser sdk.BrowserClient) *browserWorkspace {
	return &browserWorkspace{browser: browser, tabs: make(map[string]sdk.BrowserTarget)}
}

func validatePageConfig(config pageConfig, browser sdk.BrowserClient) error {
	parsed, err := url.Parse(strings.TrimSpace(config.URL))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return errors.New("url must be a valid HTTP or HTTPS URL")
	}
	if browser == nil {
		return errors.New("browser capability is unavailable")
	}
	return nil
}

func (w *browserWorkspace) run(ctx context.Context, config pageConfig, actions []sdk.BrowserAction, captureRule *sdk.BrowserNetworkCaptureRule) (browserExecution, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	started := time.Now()
	executionContext, cancel := context.WithTimeout(ctx, defaultTimeout+5*time.Second)
	defer cancel()

	target, results, reused, navigate, err := w.acquireTarget(executionContext, config, captureRule != nil)
	if err != nil {
		return browserExecution{}, err
	}
	if !config.KeepTabOpen {
		defer w.closeTarget(target)
	}

	var capture sdk.BrowserCapture
	var captureDrained chan struct{}
	if captureRule != nil {
		capture, err = w.browser.StartNetworkCapture(executionContext, sdk.BrowserNetworkStartRequest{Target: target, Rules: []sdk.BrowserNetworkCaptureRule{*captureRule}})
		if err != nil {
			return browserExecution{}, err
		}
		captureDrained = make(chan struct{})
		go func() {
			defer close(captureDrained)
			for range capture.Events() {
			}
		}()
	}

	stopCapture := func() {
		if capture != nil {
			_, _ = capture.Stop(executionContext)
		}
	}
	if navigate {
		result, actionErr := w.execute(executionContext, target, sdk.BrowserAction{ID: "navigate", Type: "tab.navigate", Params: map[string]any{"url": strings.TrimSpace(config.URL), "wait": true}})
		results = append(results, result)
		if actionErr != nil {
			stopCapture()
			return browserExecution{}, actionErr
		}
	}

	refreshed := false
	if config.KeepTabOpen {
		result, actionErr := w.execute(executionContext, target, sdk.BrowserAction{ID: "refresh", Type: "tab.reload", Params: map[string]any{"bypass_cache": config.BypassCache, "wait": true}})
		results = append(results, result)
		if actionErr != nil {
			stopCapture()
			return browserExecution{}, actionErr
		}
		refreshed = true
	}

	for _, action := range actions {
		result, actionErr := w.execute(executionContext, target, action)
		results = append(results, result)
		if actionErr != nil {
			stopCapture()
			return browserExecution{}, actionErr
		}
	}

	var network []sdk.BrowserNetworkResult
	if capture != nil {
		stopped, stopErr := capture.Stop(executionContext)
		if stopErr != nil {
			return browserExecution{}, stopErr
		}
		<-captureDrained
		if captureErr := capture.Err(); captureErr != nil {
			return browserExecution{}, captureErr
		}
		network = stopped.Events
	}

	return browserExecution{
		Target: target, Duration: time.Since(started).Milliseconds(), Actions: results, Network: network,
		Reused: reused, Refreshed: refreshed, KeptOpen: config.KeepTabOpen,
	}, nil
}

func (w *browserWorkspace) acquireTarget(ctx context.Context, config pageConfig, capture bool) (sdk.BrowserTarget, []sdk.BrowserActionResult, bool, bool, error) {
	pageURL := strings.TrimSpace(config.URL)
	if config.KeepTabOpen {
		target, tabURL, found, err := w.findRetainedTarget(ctx, pageURL)
		if err != nil {
			return sdk.BrowserTarget{}, nil, false, false, err
		}
		if found {
			return target, nil, true, !samePageURL(tabURL, pageURL), nil
		}
	}

	openURL := pageURL
	if capture && !config.KeepTabOpen {
		openURL = "about:blank"
	}
	open, err := w.browser.ExecuteAction(ctx, sdk.BrowserActionRequest{
		TimeoutMS: int(defaultTimeout.Milliseconds()),
		Action:    sdk.BrowserAction{ID: "open", Type: "tab.open", Params: map[string]any{"url": openURL, "active": false, "wait": true}},
	})
	if err != nil {
		return sdk.BrowserTarget{}, nil, false, false, err
	}
	target := targetFromResult(open)
	if target.TabID == 0 {
		return sdk.BrowserTarget{}, nil, false, false, errors.New("browser did not return the opened tab target")
	}

	group, err := w.execute(ctx, target, sdk.BrowserAction{ID: "group", Type: "tab.group", Params: map[string]any{"title": retainedGroupTitle, "color": "blue", "collapsed": false, "reuse_group": true}})
	if err != nil {
		w.closeTarget(target)
		return sdk.BrowserTarget{}, nil, false, false, err
	}
	if config.KeepTabOpen {
		w.tabs[pageURL] = target
	}
	return target, []sdk.BrowserActionResult{open, group}, false, capture && !config.KeepTabOpen, nil
}

func (w *browserWorkspace) findRetainedTarget(ctx context.Context, pageURL string) (sdk.BrowserTarget, string, bool, error) {
	targets, err := w.browser.ListTargets(ctx, "")
	if err != nil {
		return sdk.BrowserTarget{}, "", false, err
	}
	if cached, ok := w.tabs[pageURL]; ok {
		if tab, found := findTargetTab(targets, cached.TabID); found {
			return sdk.BrowserTarget{AgentID: targets.AgentID, WindowID: tab.WindowID, TabID: tab.ID}, tab.URL, true, nil
		}
		delete(w.tabs, pageURL)
	}
	for _, window := range targets.Windows {
		for _, tab := range window.Tabs {
			if tab.GroupTitle == retainedGroupTitle && samePageURL(tab.URL, pageURL) {
				target := sdk.BrowserTarget{AgentID: targets.AgentID, WindowID: window.ID, TabID: tab.ID}
				w.tabs[pageURL] = target
				return target, tab.URL, true, nil
			}
		}
	}
	return sdk.BrowserTarget{}, "", false, nil
}

func (w *browserWorkspace) execute(ctx context.Context, target sdk.BrowserTarget, action sdk.BrowserAction) (sdk.BrowserActionResult, error) {
	return w.browser.ExecuteAction(ctx, sdk.BrowserActionRequest{Target: target, TimeoutMS: int(defaultTimeout.Milliseconds()), Action: action})
}

func (w *browserWorkspace) closeTarget(target sdk.BrowserTarget) {
	closeContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _ = w.browser.ExecuteAction(closeContext, sdk.BrowserActionRequest{Target: target, TimeoutMS: 5000, Action: sdk.BrowserAction{ID: "close", Type: "tab.close"}})
}

func targetFromResult(result sdk.BrowserActionResult) sdk.BrowserTarget {
	target := result.Target
	if target.TabID == 0 {
		target.TabID = intValue(result.Data, "tab_id")
		target.WindowID = intValue(result.Data, "window_id")
	}
	return target
}

func findTargetTab(targets sdk.BrowserTargets, tabID int) (sdk.BrowserTab, bool) {
	for _, window := range targets.Windows {
		for _, tab := range window.Tabs {
			if tab.ID == tabID {
				return tab, true
			}
		}
	}
	return sdk.BrowserTab{}, false
}

func samePageURL(left, right string) bool {
	normalize := func(raw string) string {
		parsed, err := url.Parse(strings.TrimSpace(raw))
		if err != nil {
			return strings.TrimSpace(raw)
		}
		parsed.Fragment = ""
		return parsed.String()
	}
	return normalize(left) == normalize(right)
}

func commonConfigProperties() map[string]any {
	return map[string]any{
		"url":           map[string]any{"type": "string", "format": "uri"},
		"keep_tab_open": map[string]any{"type": "boolean", "default": false},
		"bypass_cache":  map[string]any{"type": "boolean", "default": false},
	}
}

func commonParameters() []sdk.ParameterDescriptor {
	return []sdk.ParameterDescriptor{
		{Key: "url", Label: "页面地址", Type: sdk.ParameterURL, Required: true, Order: 10, FullWidth: true, Placeholder: "https://example.com"},
		{Key: "keep_tab_open", Label: "保持标签页", Type: sdk.ParameterBoolean, Default: false, Order: 20, Description: "复用 Meerkit 示例标签页，并在每次采集前刷新页面。"},
		{Key: "bypass_cache", Label: "刷新时绕过缓存", Type: sdk.ParameterBoolean, Default: false, Order: 30, Description: "保持标签页时，刷新页面忽略浏览器缓存。", VisibleWhen: []sdk.ParameterCondition{{Field: "keep_tab_open", Operator: "equals", Value: true}}},
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

func mapValue(values map[string]any, key string) map[string]any {
	value, _ := values[key].(map[string]any)
	if value == nil {
		return map[string]any{}
	}
	return value
}

func intValue(values map[string]any, key string) int {
	switch value := values[key].(type) {
	case int:
		return value
	case float64:
		return int(value)
	default:
		return 0
	}
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
	return sdk.Observation{Success: false, SchemaVersion: resultSchemaVersion, Result: result, ResultSets: map[string]map[string]any{resultSet: result}, Summary: message, ErrorCode: "browser_example_execution_failed", ErrorMessage: message}
}
