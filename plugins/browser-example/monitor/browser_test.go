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
	tabs           map[int]sdk.BrowserTab
	nextTabID      int
}

func newFakeBrowserClient() *fakeBrowserClient {
	return &fakeBrowserClient{tabs: make(map[int]sdk.BrowserTab), nextTabID: 7}
}

func (f *fakeBrowserClient) ListTargets(context.Context, string) (sdk.BrowserTargets, error) {
	windows := map[int][]sdk.BrowserTab{}
	for _, tab := range f.tabs {
		windows[tab.WindowID] = append(windows[tab.WindowID], tab)
	}
	result := sdk.BrowserTargets{AgentID: "agent"}
	for windowID, tabs := range windows {
		result.Windows = append(result.Windows, sdk.BrowserWindow{ID: windowID, Tabs: tabs})
	}
	return result, nil
}

func (f *fakeBrowserClient) ExecuteAction(_ context.Context, request sdk.BrowserActionRequest) (sdk.BrowserActionResult, error) {
	f.requests = append(f.requests, request)
	result := sdk.BrowserActionResult{ID: request.Action.ID, Type: request.Action.Type, Success: true, Target: request.Target, Data: map[string]any{}}
	switch request.Action.Type {
	case "tab.open":
		tabID := f.nextTabID
		f.nextTabID++
		pageURL, _ := request.Action.Params["url"].(string)
		f.tabs[tabID] = sdk.BrowserTab{ID: tabID, WindowID: 2, URL: pageURL}
		result.Target = sdk.BrowserTarget{AgentID: "agent", WindowID: 2, TabID: tabID}
		result.Data = map[string]any{"window_id": 2, "tab_id": tabID}
	case "tab.group":
		tab := f.tabs[request.Target.TabID]
		tab.GroupTitle, _ = request.Action.Params["title"].(string)
		f.tabs[request.Target.TabID] = tab
	case "tab.navigate":
		tab := f.tabs[request.Target.TabID]
		tab.URL, _ = request.Action.Params["url"].(string)
		f.tabs[request.Target.TabID] = tab
	case "tab.close":
		delete(f.tabs, request.Target.TabID)
	case "dom.query":
		result.Data = map[string]any{
			"text": "Meerkit ready", "html": "<h1 data-state=\"ready\">Meerkit ready</h1>", "value": "",
			"attributes": map[string]any{"data-state": "ready"}, "title": "Status", "url": "https://example.com/", "tag_name": "h1", "visible": true, "truncated": false,
		}
	}
	return result, nil
}

func (f *fakeBrowserClient) StartNetworkCapture(_ context.Context, request sdk.BrowserNetworkStartRequest) (sdk.BrowserCapture, error) {
	f.captureRequest = request
	return &fakeCapture{session: sdk.BrowserNetworkSession{ID: "capture", Target: request.Target}, events: f.events}, nil
}

func (f *fakeBrowserClient) actionCount(actionType string) int {
	count := 0
	for _, request := range f.requests {
		if request.Action.Type == actionType {
			count++
		}
	}
	return count
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

func TestElementReadsContentAndClosesTemporaryTab(t *testing.T) {
	client := newFakeBrowserClient()
	observation, err := NewElement(client).Execute(t.Context(), json.RawMessage(`{"url":"https://example.com","selector":"main h1"}`))
	if err != nil {
		t.Fatal(err)
	}
	if observation.Result["text"] != "Meerkit ready" || observation.Result["html"] == "" || observation.Result["visible"] != true {
		t.Fatalf("unexpected element result: %+v", observation.Result)
	}
	if client.actionCount("tab.open") != 1 || client.actionCount("tab.close") != 1 || client.actionCount("tab.reload") != 0 {
		t.Fatalf("unexpected temporary tab lifecycle: %+v", client.requests)
	}
}

func TestElementKeepsReusesAndRefreshesTab(t *testing.T) {
	client := newFakeBrowserClient()
	module := NewElement(client)
	config := json.RawMessage(`{"url":"https://example.com","selector":"main h1","keep_tab_open":true,"bypass_cache":true}`)
	first, err := module.Execute(t.Context(), config)
	if err != nil {
		t.Fatal(err)
	}
	second, err := module.Execute(t.Context(), config)
	if err != nil {
		t.Fatal(err)
	}
	if client.actionCount("tab.open") != 1 || client.actionCount("tab.reload") != 2 || client.actionCount("tab.close") != 0 {
		t.Fatalf("persistent tab was not reused and refreshed: %+v", client.requests)
	}
	if first.Result["tab_reused"] != false || second.Result["tab_reused"] != true || second.Result["tab_kept_open"] != true {
		t.Fatalf("unexpected tab metadata: first=%+v second=%+v", first.Result, second.Result)
	}
	for _, request := range client.requests {
		if request.Action.Type == "tab.reload" && request.Action.Params["bypass_cache"] != true {
			t.Fatalf("reload did not preserve bypass_cache: %+v", request)
		}
	}
}

func TestElementDescriptorUsesCSSSelectorField(t *testing.T) {
	descriptor := NewElement(newFakeBrowserClient()).Descriptor()
	for _, parameter := range descriptor.Parameters {
		if parameter.Key == "selector" {
			if parameter.Type != sdk.ParameterCSSSelector || parameter.SelectorCandidates == nil {
				t.Fatalf("selector parameter does not expose browser discovery: %+v", parameter)
			}
			return
		}
	}
	t.Fatal("selector parameter was not found")
}

func TestModuleDescriptorsAreSelfConsistent(t *testing.T) {
	provider := sdk.NewProvider(NewModules(newFakeBrowserClient())...)
	descriptors, err := provider.ListModules()
	if err != nil {
		t.Fatal(err)
	}
	if len(descriptors) != 2 {
		t.Fatalf("unexpected module count: %d", len(descriptors))
	}
	types := map[string]bool{}
	for _, descriptor := range descriptors {
		types[descriptor.Type] = true
	}
	if !types[elementModuleType] || !types[responseModuleType] {
		t.Fatalf("unexpected module descriptors: %+v", descriptors)
	}
}

func TestResponseStartsCaptureBeforeNavigation(t *testing.T) {
	client := newFakeBrowserClient()
	client.events = []sdk.BrowserNetworkResult{{CaptureID: "response", URL: "https://example.com/api/status", Status: 200, Body: `{"ready":true}`}}
	observation, err := NewResponse(client).Execute(t.Context(), json.RawMessage(`{"url":"https://example.com","url_contains":"/api/status"}`))
	if err != nil {
		t.Fatal(err)
	}
	if observation.Result["status"] != 200 || client.captureRequest.Target.TabID != 7 {
		t.Fatalf("unexpected response: %+v", observation.Result)
	}
	if client.actionCount("tab.navigate") != 1 || client.actionCount("tab.close") != 1 {
		t.Fatalf("capture flow did not navigate and close its temporary tab: %+v", client.requests)
	}
}

func TestResponseRefreshesPersistentTabAfterCaptureStarts(t *testing.T) {
	client := newFakeBrowserClient()
	client.events = []sdk.BrowserNetworkResult{{CaptureID: "response", URL: "https://example.com/api/status", Status: 204}}
	observation, err := NewResponse(client).Execute(t.Context(), json.RawMessage(`{"url":"https://example.com","url_contains":"/api/status","keep_tab_open":true}`))
	if err != nil {
		t.Fatal(err)
	}
	if client.actionCount("tab.reload") != 1 || client.actionCount("tab.navigate") != 0 || client.actionCount("tab.close") != 0 {
		t.Fatalf("persistent capture did not use refresh lifecycle: %+v", client.requests)
	}
	if observation.Result["tab_kept_open"] != true || observation.Result["tab_refreshed"] != true {
		t.Fatalf("persistent result metadata is missing: %+v", observation.Result)
	}
}

func TestExampleModulesRejectInvalidConfig(t *testing.T) {
	client := newFakeBrowserClient()
	tests := []struct {
		module sdk.Module
		config string
	}{
		{NewElement(client), `{"url":"javascript:alert(1)","selector":"main"}`},
		{NewElement(client), `{"url":"https://example.com","selector":""}`},
		{NewResponse(client), `{"url":"https://example.com","url_contains":""}`},
	}
	for _, test := range tests {
		if err := test.module.ValidateConfig(json.RawMessage(test.config)); err == nil {
			t.Fatalf("invalid config accepted by %T", test.module)
		}
	}
}
