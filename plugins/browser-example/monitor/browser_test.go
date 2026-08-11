package browsermonitor

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hanxuanyu/meerkit/sdk"
)

type fakeBrowserClient struct{ result sdk.BrowserRunResult }

func (f fakeBrowserClient) Run(context.Context, sdk.BrowserRunRequest) (sdk.BrowserRunResult, error) {
	return f.result, nil
}
func (fakeBrowserClient) Close() error { return nil }

func TestBrowserExampleModule(t *testing.T) {
	client := fakeBrowserClient{result: sdk.BrowserRunResult{Duration: 42, Actions: []sdk.BrowserActionResult{{ID: "element", Type: "dom.query", Success: true, Data: map[string]any{"text": "Meerkit ready", "title": "Status", "url": "https://example.com/", "tag_name": "h1"}}}, Network: []sdk.BrowserNetworkResult{{CaptureID: "api", URL: "https://example.com/api/status", Status: 200, MimeType: "application/json", Body: `{"ok":true}`}}}}
	module := New(client)
	config := json.RawMessage(`{"url":"https://example.com","selector":"h1","api_url_contains":"/api/status"}`)
	if err := module.ValidateConfig(config); err != nil {
		t.Fatal(err)
	}
	observation, err := module.Execute(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	if !observation.Success || observation.Result["text"] != "Meerkit ready" || observation.Result["api_status"] != 200 || observation.ResultSets["page"]["tag_name"] != "h1" {
		t.Fatalf("unexpected observation: %+v", observation)
	}
}

func TestBrowserExampleRejectsInvalidConfig(t *testing.T) {
	module := New(fakeBrowserClient{})
	for _, value := range []string{`{"url":"javascript:alert(1)","selector":"h1"}`, `{"url":"https://example.com","selector":""}`} {
		if err := module.ValidateConfig(json.RawMessage(value)); err == nil {
			t.Fatalf("invalid config accepted: %s", value)
		}
	}
}
