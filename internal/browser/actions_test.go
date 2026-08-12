package browser

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/hanxuanyu/meerkit/sdk"
)

func TestActionCatalogExcludesNetworkCapture(t *testing.T) {
	catalog := BrowserActionCatalog()
	if len(catalog.Actions) != 56 {
		t.Fatalf("action catalog count = %d, want 56", len(catalog.Actions))
	}
	seen := make(map[string]struct{}, len(catalog.Actions))
	for _, action := range catalog.Actions {
		if _, exists := seen[action.Type]; exists {
			t.Fatalf("duplicate action %s", action.Type)
		}
		seen[action.Type] = struct{}{}
		if action.Type == "network.capture" {
			t.Fatal("network capture must not be an action")
		}
		if action.TargetMode == "" {
			t.Fatalf("action %s has no target mode", action.Type)
		}
		if action.Parameters == nil {
			t.Fatalf("action %s has nil parameters", action.Type)
		}
		for _, parameter := range action.Parameters {
			if strings.TrimSpace(parameter.Key) == "" || strings.TrimSpace(parameter.Label) == "" || parameter.Type == "" {
				t.Fatalf("action %s contains an incomplete parameter descriptor: %#v", action.Type, parameter)
			}
			if strings.TrimSpace(parameter.Description) == "" {
				t.Fatalf("action %s parameter %s has no description", action.Type, parameter.Key)
			}
		}
	}
}

func TestValidateBrowserActionRequest(t *testing.T) {
	tests := []struct {
		name    string
		request sdk.BrowserActionRequest
		want    string
	}{
		{name: "unknown", request: sdk.BrowserActionRequest{Action: sdk.BrowserAction{Type: "unknown"}}, want: "unsupported"},
		{name: "missing tab", request: sdk.BrowserActionRequest{Action: sdk.BrowserAction{Type: "dom.query", Params: map[string]any{"selector": "main"}}}, want: "tab_id"},
		{name: "missing selector", request: sdk.BrowserActionRequest{Target: sdk.BrowserTarget{TabID: 3}, Action: sdk.BrowserAction{Type: "dom.query"}}, want: "selector"},
		{name: "missing window", request: sdk.BrowserActionRequest{Action: sdk.BrowserAction{Type: "window.close"}}, want: "window_id"},
		{name: "invalid zoom", request: sdk.BrowserActionRequest{Target: sdk.BrowserTarget{TabID: 3}, Action: sdk.BrowserAction{Type: "tab.zoom", Params: map[string]any{"factor": 8}}}, want: "between"},
		{name: "invalid storage area", request: sdk.BrowserActionRequest{Target: sdk.BrowserTarget{TabID: 3}, Action: sdk.BrowserAction{Type: "storage.get", Params: map[string]any{"area": "indexeddb"}}}, want: "area"},
		{name: "invalid boolean", request: sdk.BrowserActionRequest{Target: sdk.BrowserTarget{TabID: 3}, Action: sdk.BrowserAction{Type: "tab.pin", Params: map[string]any{"pinned": "false"}}}, want: "boolean"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := ValidateBrowserActionRequest(test.request); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("got %v, want %q", err, test.want)
			}
		})
	}
	if err := ValidateBrowserActionRequest(sdk.BrowserActionRequest{Target: sdk.BrowserTarget{TabID: 3}, Action: sdk.BrowserAction{Type: "dom.query", Params: map[string]any{"selector": "main"}}}); err != nil {
		t.Fatal(err)
	}
}

func TestValidateBrowserActionRequestAcceptsJSONNumbers(t *testing.T) {
	for _, payload := range []string{
		`{"target":{"tab_id":3},"action":{"type":"tab.zoom","params":{"factor":1.25}}}`,
		`{"target":{"window_id":4},"action":{"type":"window.resize","params":{"width":1280,"height":720}}}`,
		`{"target":{"tab_id":3},"action":{"type":"dom.query_all","params":{"selector":"li","limit":25,"max_length":2048}}}`,
	} {
		var request sdk.BrowserActionRequest
		decoder := json.NewDecoder(bytes.NewBufferString(payload))
		decoder.UseNumber()
		if err := decoder.Decode(&request); err != nil {
			t.Fatal(err)
		}
		if err := ValidateBrowserActionRequest(request); err != nil {
			t.Fatalf("ValidateBrowserActionRequest(%s): %v", payload, err)
		}
	}
}

func TestActionCatalogSensitiveAndDestructiveMetadata(t *testing.T) {
	actions := map[string]ActionDefinition{}
	for _, action := range BrowserActionCatalog().Actions {
		actions[action.Type] = action
	}
	for _, actionType := range []string{"cookie.list", "cookie.set", "cookie.delete", "cookie.clear", "storage.get", "storage.set", "storage.remove", "storage.clear", "runtime.evaluate"} {
		if !actions[actionType].Sensitive {
			t.Fatalf("action %s must be sensitive", actionType)
		}
	}
	for _, actionType := range []string{"window.close", "tab.close", "cookie.set", "cookie.delete", "cookie.clear", "storage.set", "storage.remove", "storage.clear", "runtime.evaluate"} {
		if !actions[actionType].Destructive {
			t.Fatalf("action %s must be destructive", actionType)
		}
	}
}

func TestNormalizeBrowserActionRequestAppliesCatalogDefaults(t *testing.T) {
	tests := []struct {
		actionType string
		key        string
		want       any
	}{
		{actionType: "tab.pin", key: "pinned", want: true},
		{actionType: "tab.mute", key: "muted", want: true},
		{actionType: "dom.check", key: "checked", want: true},
		{actionType: "tab.auto_discardable", key: "auto_discardable", want: true},
		{actionType: "dom.dispatch_event", key: "event", want: "change"},
	}
	for _, test := range tests {
		t.Run(test.actionType, func(t *testing.T) {
			request := normalizeBrowserActionRequest(sdk.BrowserActionRequest{Action: sdk.BrowserAction{Type: test.actionType}})
			if got := request.Action.Params[test.key]; got != test.want {
				t.Fatalf("default %s = %#v, want %#v", test.key, got, test.want)
			}
		})
	}

	request := normalizeBrowserActionRequest(sdk.BrowserActionRequest{Action: sdk.BrowserAction{Type: "tab.pin", Params: map[string]any{"pinned": false}}})
	if request.Action.Params["pinned"] != false {
		t.Fatal("explicit false must not be replaced by the catalog default")
	}

	selectorWait := normalizeBrowserActionRequest(sdk.BrowserActionRequest{Action: sdk.BrowserAction{Type: "page.wait", Params: map[string]any{"selector": "main"}}})
	if selectorWait.Action.Params["mode"] != "selector" {
		t.Fatalf("selector-only wait mode = %#v, want selector", selectorWait.Action.Params["mode"])
	}
	durationWait := normalizeBrowserActionRequest(sdk.BrowserActionRequest{Action: sdk.BrowserAction{Type: "page.wait", Params: map[string]any{"duration_ms": 250}}})
	if durationWait.Action.Params["mode"] != "duration" {
		t.Fatalf("duration-only wait mode = %#v, want duration", durationWait.Action.Params["mode"])
	}
}

func TestValidateExpandedBrowserActions(t *testing.T) {
	valid := []sdk.BrowserActionRequest{
		{Target: sdk.BrowserTarget{TabID: 3}, Action: sdk.BrowserAction{Type: "tab.discard"}},
		{Target: sdk.BrowserTarget{TabID: 3}, Action: sdk.BrowserAction{Type: "page.performance"}},
		{Target: sdk.BrowserTarget{TabID: 3}, Action: sdk.BrowserAction{Type: "page.wait", Params: map[string]any{"mode": "visible", "selector": "main"}}},
		{Target: sdk.BrowserTarget{TabID: 3}, Action: sdk.BrowserAction{Type: "page.wait", Params: map[string]any{"mode": "title", "value": "Meerkit"}}},
		{Target: sdk.BrowserTarget{TabID: 3}, Action: sdk.BrowserAction{Type: "dom.set_attribute", Params: map[string]any{"selector": "main", "name": "data-state", "value": "ready"}}},
		{Target: sdk.BrowserTarget{TabID: 3}, Action: sdk.BrowserAction{Type: "dom.dispatch_event", Params: map[string]any{"selector": "input", "event": "change"}}},
	}
	for _, request := range valid {
		if err := ValidateBrowserActionRequest(request); err != nil {
			t.Fatalf("ValidateBrowserActionRequest(%s): %v", request.Action.Type, err)
		}
	}

	invalid := sdk.BrowserActionRequest{Target: sdk.BrowserTarget{TabID: 3}, Action: sdk.BrowserAction{Type: "dom.dispatch_event", Params: map[string]any{"selector": "input", "event": "click"}}}
	if err := ValidateBrowserActionRequest(invalid); err == nil || !strings.Contains(err.Error(), "event") {
		t.Fatalf("invalid event error = %v", err)
	}
}

func TestActionCatalogBrowserTargetParameterTypes(t *testing.T) {
	for _, action := range BrowserActionCatalog().Actions {
		if action.Type != "tab.move" {
			continue
		}
		for _, parameter := range action.Parameters {
			if parameter.Key == "destination_window_id" {
				if parameter.Type != sdk.ParameterBrowserWindow {
					t.Fatalf("destination_window_id type = %q, want %q", parameter.Type, sdk.ParameterBrowserWindow)
				}
				return
			}
		}
	}
	t.Fatal("tab.move destination_window_id parameter was not found")
}

func TestActionCatalogCSSSelectorParameters(t *testing.T) {
	configured := map[string]bool{
		"dom.click": true, "dom.input": true, "dom.check": true, "dom.select": true,
		"dom.submit": true, "input.click": true, "input.hover": true, "input.type": true,
	}
	foundConfigured := make(map[string]bool, len(configured))
	for _, action := range BrowserActionCatalog().Actions {
		for _, parameter := range action.Parameters {
			if parameter.Key != "selector" {
				continue
			}
			if parameter.Type != sdk.ParameterCSSSelector {
				t.Fatalf("action %s selector type = %q, want %q", action.Type, parameter.Type, sdk.ParameterCSSSelector)
			}
			if configured[action.Type] {
				if parameter.SelectorCandidates == nil || len(parameter.SelectorCandidates.Queries) == 0 {
					t.Fatalf("action %s must configure selector candidates", action.Type)
				}
				foundConfigured[action.Type] = true
			} else if parameter.SelectorCandidates != nil {
				t.Fatalf("action %s unexpectedly configures selector candidates", action.Type)
			}
		}
	}
	for actionType := range configured {
		if !foundConfigured[actionType] {
			t.Fatalf("action %s configured selector parameter was not found", actionType)
		}
	}
}

func TestValidateBrowserSelectorCandidatesRequest(t *testing.T) {
	valid := sdk.BrowserSelectorCandidatesRequest{Target: sdk.BrowserTarget{TabID: 3}, Queries: []string{"button", "a[href]"}, Limit: 50}
	if err := ValidateBrowserSelectorCandidatesRequest(valid); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		request sdk.BrowserSelectorCandidatesRequest
	}{
		{name: "missing tab", request: sdk.BrowserSelectorCandidatesRequest{Queries: []string{"button"}}},
		{name: "missing queries", request: sdk.BrowserSelectorCandidatesRequest{Target: sdk.BrowserTarget{TabID: 3}}},
		{name: "empty query", request: sdk.BrowserSelectorCandidatesRequest{Target: sdk.BrowserTarget{TabID: 3}, Queries: []string{" "}}},
		{name: "limit too large", request: sdk.BrowserSelectorCandidatesRequest{Target: sdk.BrowserTarget{TabID: 3}, Queries: []string{"button"}, Limit: 201}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := ValidateBrowserSelectorCandidatesRequest(test.request); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestBuildActionSpecMapRejectsDuplicateTypes(t *testing.T) {
	defer func() {
		if recovered := recover(); recovered == nil || !strings.Contains(recovered.(string), "duplicate browser action type") {
			t.Fatalf("panic = %v, want duplicate action error", recovered)
		}
	}()
	buildActionSpecMap([]actionSpec{
		newAction("test.action", "Test", "Test action.", "test", "Test", "test", "status", "none", false, nil, noValidation),
		newAction("test.action", "Test", "Test action.", "test", "Test", "test", "status", "none", false, nil, noValidation),
	})
}
