package browser

import (
	"strings"
	"testing"

	"github.com/hanxuanyu/meerkit/sdk"
)

func TestBrowserActionCatalogDefinesExecutableActions(t *testing.T) {
	catalog := BrowserActionCatalog()
	if len(catalog.Actions) != 12 {
		t.Fatalf("action definitions = %d, want 12", len(catalog.Actions))
	}
	seen := make(map[string]bool, len(catalog.Actions))
	for _, definition := range catalog.Actions {
		if definition.Type == "" || definition.Label == "" || definition.Category == "" || definition.Capability == "" || definition.ResultType == "" {
			t.Fatalf("incomplete action definition: %#v", definition)
		}
		if seen[definition.Type] {
			t.Fatalf("duplicate action definition %q", definition.Type)
		}
		seen[definition.Type] = true
	}
	if !seen["network.capture"] {
		t.Fatal("network.capture is not exposed as an action")
	}
	if err := ValidateBrowserRunRequest(sdk.BrowserRunRequest{Actions: catalog.StarterFlow}); err != nil {
		t.Fatalf("starter flow is invalid: %v", err)
	}
}

func TestValidateBrowserRunRequestUsesActionValidators(t *testing.T) {
	tests := []struct {
		name    string
		request sdk.BrowserRunRequest
		want    string
	}{
		{name: "unknown action", request: sdk.BrowserRunRequest{Actions: []sdk.BrowserAction{{Type: "unknown"}}}, want: "unsupported type"},
		{name: "missing selector", request: sdk.BrowserRunRequest{Actions: []sdk.BrowserAction{{Type: "dom.query"}}}, want: "selector"},
		{name: "duplicate id", request: sdk.BrowserRunRequest{Actions: []sdk.BrowserAction{{ID: "same", Type: "page.wait"}, {ID: "same", Type: "page.wait"}}}, want: "duplicated"},
		{name: "invalid context", request: sdk.BrowserRunRequest{TabID: -1, Actions: []sdk.BrowserAction{{Type: "page.wait"}}}, want: "cannot be negative"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateBrowserRunRequest(test.request)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("validation error = %v, want text %q", err, test.want)
			}
		})
	}
}

func TestValidateBrowserRunRequestAcceptsAtomicNetworkCapture(t *testing.T) {
	request := sdk.BrowserRunRequest{TabID: 12, WindowID: 3, Actions: []sdk.BrowserAction{
		{ID: "capture", Type: "network.capture", Params: map[string]any{"capture_id": "api", "url_contains": "/api/", "resource_type": "Fetch", "max_body_bytes": 4096}},
		{ID: "navigate", Type: "tab.navigate", Params: map[string]any{"url": "https://example.com"}},
	}}
	if err := ValidateBrowserRunRequest(request); err != nil {
		t.Fatal(err)
	}
}
