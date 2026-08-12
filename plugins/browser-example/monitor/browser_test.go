package browsermonitor

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hanxuanyu/meerkit/sdk"
)

type fakeBrowserClient struct {
	requests       []sdk.BrowserActionRequest
	events         []sdk.BrowserNetworkResult
	captureRequest sdk.BrowserNetworkStartRequest
}

func (f *fakeBrowserClient) ListTargets(context.Context, string) (sdk.BrowserTargets, error) {
	return sdk.BrowserTargets{}, nil
}
func (f *fakeBrowserClient) ExecuteAction(_ context.Context, request sdk.BrowserActionRequest) (sdk.BrowserActionResult, error) {
	f.requests = append(f.requests, request)
	result := sdk.BrowserActionResult{ID: request.Action.ID, Type: request.Action.Type, Success: true, Target: request.Target, Data: map[string]any{}}
	switch request.Action.Type {
	case "tab.open":
		result.Target = sdk.BrowserTarget{AgentID: "agent", WindowID: 2, TabID: 7}
		result.Data = map[string]any{"window_id": 2, "tab_id": 7}
	case "dom.document":
		result.Data = map[string]any{"html": "<html>ready</html>", "title": "Status", "url": "https://example.com/home", "truncated": false}
	case "dom.query":
		result.Data = map[string]any{"text": "Meerkit ready", "title": "Status", "url": "https://example.com/", "tag_name": "h1"}
	}
	return result, nil
}
func (f *fakeBrowserClient) StartNetworkCapture(_ context.Context, request sdk.BrowserNetworkStartRequest) (sdk.BrowserCapture, error) {
	f.captureRequest = request
	return &fakeCapture{session: sdk.BrowserNetworkSession{ID: "capture", Target: request.Target}, events: f.events}, nil
}

type fakeCapture struct {
	session sdk.BrowserNetworkSession
	events  []sdk.BrowserNetworkResult
}

func (f *fakeCapture) Session() sdk.BrowserNetworkSession { return f.session }
func (f *fakeCapture) Events() <-chan sdk.BrowserNetworkResult {
	channel := make(chan sdk.BrowserNetworkResult)
	close(channel)
	return channel
}
func (f *fakeCapture) Err() error { return nil }
func (f *fakeCapture) Stop(context.Context) (sdk.BrowserNetworkStopResult, error) {
	return sdk.BrowserNetworkStopResult{Session: f.session, Events: f.events}, nil
}

func TestHTMLUsesOpenedTargetForAtomicActions(t *testing.T) {
	client := &fakeBrowserClient{}
	observation, err := NewHTML(client).Execute(t.Context(), json.RawMessage(`{"url":"https://example.com/start"}`))
	if err != nil {
		t.Fatal(err)
	}
	if observation.Result["html"] != "<html>ready</html>" {
		t.Fatalf("unexpected result: %+v", observation.Result)
	}
	if len(client.requests) < 4 || client.requests[2].Target.TabID != 7 || client.requests[2].Action.Type != "dom.document" {
		t.Fatalf("target was not propagated: %+v", client.requests)
	}
}

func TestResponseStartsCaptureBeforeNavigation(t *testing.T) {
	client := &fakeBrowserClient{events: []sdk.BrowserNetworkResult{{CaptureID: "response", URL: "https://example.com/api/status", Status: 200, Body: `{"ready":true}`}}}
	observation, err := NewResponse(client).Execute(t.Context(), json.RawMessage(`{"url":"https://example.com","url_contains":"/api/status"}`))
	if err != nil {
		t.Fatal(err)
	}
	if observation.Result["status"] != 200 || client.captureRequest.Target.TabID != 7 {
		t.Fatalf("unexpected response: %+v", observation.Result)
	}
	foundNavigate := false
	for _, request := range client.requests {
		if request.Action.Type == "tab.navigate" {
			foundNavigate = true
			if request.Target.TabID != 7 {
				t.Fatal("navigate target mismatch")
			}
		}
	}
	if !foundNavigate {
		t.Fatal("capture flow did not navigate the tab")
	}
}

func TestExampleModulesRejectInvalidConfig(t *testing.T) {
	client := &fakeBrowserClient{}
	tests := []struct {
		module sdk.Module
		config string
	}{{NewHTML(client), `{"url":"javascript:alert(1)"}`}, {NewCSSText(client), `{"url":"https://example.com","selector":""}`}, {NewResponse(client), `{"url":"https://example.com","url_contains":""}`}}
	for _, test := range tests {
		if err := test.module.ValidateConfig(json.RawMessage(test.config)); err == nil {
			t.Fatalf("invalid config accepted by %T", test.module)
		}
	}
}
