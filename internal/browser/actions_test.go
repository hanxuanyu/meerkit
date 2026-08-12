package browser

import (
	"strings"
	"testing"

	"github.com/hanxuanyu/meerkit/sdk"
)

func TestActionCatalogExcludesNetworkCapture(t *testing.T) {
	catalog := BrowserActionCatalog()
	if len(catalog.Actions) == 0 {
		t.Fatal("action catalog is empty")
	}
	for _, action := range catalog.Actions {
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
