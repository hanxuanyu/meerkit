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
	if len(catalog.Actions) != 46 {
		t.Fatalf("action catalog count = %d, want 46", len(catalog.Actions))
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
	for _, actionType := range []string{"cookie.list", "cookie.set", "cookie.delete", "cookie.clear", "storage.get", "storage.set", "storage.remove", "storage.clear"} {
		if !actions[actionType].Sensitive {
			t.Fatalf("action %s must be sensitive", actionType)
		}
	}
	for _, actionType := range []string{"window.close", "tab.close", "cookie.set", "cookie.delete", "cookie.clear", "storage.set", "storage.remove", "storage.clear"} {
		if !actions[actionType].Destructive {
			t.Fatalf("action %s must be destructive", actionType)
		}
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
