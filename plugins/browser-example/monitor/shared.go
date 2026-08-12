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
	URL string `json:"url"`
}
type moduleBase struct{ browser sdk.BrowserClient }
type browserExecution struct {
	Duration int64
	Actions  []sdk.BrowserActionResult
	Network  []sdk.BrowserNetworkResult
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

func (m moduleBase) run(ctx context.Context, config pageConfig, actions []sdk.BrowserAction, captureRule *sdk.BrowserNetworkCaptureRule) (browserExecution, error) {
	started := time.Now()
	executionContext, cancel := context.WithTimeout(ctx, defaultTimeout+5*time.Second)
	defer cancel()
	openURL := strings.TrimSpace(config.URL)
	if captureRule != nil {
		openURL = "about:blank"
	}
	open, err := m.browser.ExecuteAction(executionContext, sdk.BrowserActionRequest{TimeoutMS: int(defaultTimeout.Milliseconds()), Action: sdk.BrowserAction{ID: "open", Type: "tab.open", Params: map[string]any{"url": openURL, "active": false, "wait": true}}})
	if err != nil {
		return browserExecution{}, err
	}
	target := open.Target
	if target.TabID == 0 {
		target.TabID = intValue(open.Data, "tab_id")
		target.WindowID = intValue(open.Data, "window_id")
	}
	if target.TabID == 0 {
		return browserExecution{}, errors.New("browser did not return the opened tab target")
	}
	defer func() {
		closeContext, closeCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer closeCancel()
		_, _ = m.browser.ExecuteAction(closeContext, sdk.BrowserActionRequest{Target: target, TimeoutMS: 5000, Action: sdk.BrowserAction{ID: "close", Type: "tab.close"}})
	}()

	results := []sdk.BrowserActionResult{open}
	group, groupErr := m.browser.ExecuteAction(executionContext, sdk.BrowserActionRequest{Target: target, TimeoutMS: int(defaultTimeout.Milliseconds()), Action: sdk.BrowserAction{ID: "group", Type: "tab.group", Params: map[string]any{"title": "Meerkit", "color": "blue", "collapsed": false, "reuse_group": true}}})
	if groupErr != nil {
		return browserExecution{}, groupErr
	}
	results = append(results, group)

	var capture sdk.BrowserCapture
	var captureDrained chan struct{}
	if captureRule != nil {
		capture, err = m.browser.StartNetworkCapture(executionContext, sdk.BrowserNetworkStartRequest{Target: target, Rules: []sdk.BrowserNetworkCaptureRule{*captureRule}})
		if err != nil {
			return browserExecution{}, err
		}
		captureDrained = make(chan struct{})
		go func() {
			defer close(captureDrained)
			for range capture.Events() {
			}
		}()
		if _, err = m.browser.ExecuteAction(executionContext, sdk.BrowserActionRequest{Target: target, TimeoutMS: int(defaultTimeout.Milliseconds()), Action: sdk.BrowserAction{ID: "navigate", Type: "tab.navigate", Params: map[string]any{"url": strings.TrimSpace(config.URL), "wait": true}}}); err != nil {
			_, _ = capture.Stop(executionContext)
			return browserExecution{}, err
		}
	}

	for _, action := range actions {
		result, actionErr := m.browser.ExecuteAction(executionContext, sdk.BrowserActionRequest{Target: target, TimeoutMS: int(defaultTimeout.Milliseconds()), Action: action})
		results = append(results, result)
		if actionErr != nil {
			if capture != nil {
				_, _ = capture.Stop(executionContext)
			}
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
	return browserExecution{Duration: time.Since(started).Milliseconds(), Actions: results, Network: network}, nil
}

func commonConfigProperties() map[string]any {
	return map[string]any{"url": map[string]any{"type": "string", "format": "uri"}}
}
func commonParameters() []sdk.ParameterDescriptor {
	return []sdk.ParameterDescriptor{{Key: "url", Label: "页面地址", Type: sdk.ParameterURL, Required: true, Order: 10, FullWidth: true, Placeholder: "https://example.com"}}
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
func boolValue(values map[string]any, key string) bool { value, _ := values[key].(bool); return value }
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
