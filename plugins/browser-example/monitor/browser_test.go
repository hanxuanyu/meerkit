package browsermonitor

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hanxuanyu/meerkit/sdk"
)

type fakeBrowserClient struct {
	result   sdk.BrowserRunResult
	requests []sdk.BrowserRunRequest
}

func (f *fakeBrowserClient) Run(_ context.Context, request sdk.BrowserRunRequest) (sdk.BrowserRunResult, error) {
	f.requests = append(f.requests, request)
	return f.result, nil
}
func (*fakeBrowserClient) Close() error { return nil }

func TestExampleModulesHaveDistinctExampleDescriptors(t *testing.T) {
	client := &fakeBrowserClient{}
	modules := []sdk.Module{NewHTML(client), NewCSSText(client), NewResponse(client)}
	seen := map[string]bool{}
	for _, module := range modules {
		descriptor := module.Descriptor()
		if descriptor.Type == "" || !containsExample(descriptor.Type) || !containsExample(descriptor.Name) {
			t.Fatalf("module must identify itself as an example: type=%q name=%q", descriptor.Type, descriptor.Name)
		}
		if seen[descriptor.Type] {
			t.Fatalf("duplicate module type %q", descriptor.Type)
		}
		seen[descriptor.Type] = true
		foundBoolean := false
		for _, parameter := range descriptor.Parameters {
			if parameter.Key == "always_new_tab" && parameter.Type == sdk.ParameterBoolean && parameter.Default == false {
				foundBoolean = true
			}
		}
		if !foundBoolean {
			t.Fatalf("module %q does not expose the boolean tab policy", descriptor.Type)
		}
	}
}

func TestHTMLModuleDefaultsToReusableGroupedTab(t *testing.T) {
	client := &fakeBrowserClient{result: sdk.BrowserRunResult{Duration: 42, Actions: []sdk.BrowserActionResult{
		{ID: "open", Type: "tab.open", Success: true, Data: map[string]any{"reused": true}},
		{ID: "document", Type: "dom.document", Success: true, Data: map[string]any{"html": "<html>ready</html>", "title": "Status", "url": "https://example.com/home", "truncated": false}},
	}}}
	observation, err := NewHTML(client).Execute(context.Background(), json.RawMessage(`{"url":"https://example.com/start"}`))
	if err != nil {
		t.Fatal(err)
	}
	if observation.Result["html"] != "<html>ready</html>" || observation.Result["page_url"] != "https://example.com/home" || observation.Result["reused_tab"] != true {
		t.Fatalf("unexpected HTML observation: %+v", observation)
	}
	request := client.requests[0]
	if !request.KeepTab || len(request.Actions) != 3 || request.Actions[0].Type != "tab.open" || request.Actions[1].Type != "tab.group" {
		t.Fatalf("unexpected browser request: %+v", request)
	}
	if request.Actions[0].Params["reuse"] != true || request.Actions[0].Params["reuse_key"] != "browser-example-html:https://example.com/start" || request.Actions[0].Params["group_title"] != "Meerkit" || request.Actions[1].Params["reuse_group"] != true {
		t.Fatalf("reusable grouped tab parameters missing: %+v", request.Actions)
	}
}

func TestReusablePageKeyIgnoresRedirectIrrelevantFragment(t *testing.T) {
	module := NewHTML(&fakeBrowserClient{})
	if got := module.reusablePageKey(pageConfig{URL: "https://example.com/start#login"}); got != "browser-example-html:https://example.com/start" {
		t.Fatalf("reusable page key = %q", got)
	}
	if got := module.reusablePageKey(pageConfig{URL: "https://example.com/start", TabReuseKey: " account-a "}); got != "browser-example-html:account-a" {
		t.Fatalf("explicit reusable page key = %q", got)
	}
}

func TestCSSTextModuleCanAlwaysOpenNewTab(t *testing.T) {
	client := &fakeBrowserClient{result: sdk.BrowserRunResult{Actions: []sdk.BrowserActionResult{
		{ID: "open", Type: "tab.open", Success: true, Data: map[string]any{"reused": false}},
		{ID: "element", Type: "dom.query", Success: true, Data: map[string]any{"text": "Meerkit ready", "title": "Status", "url": "https://example.com/", "tag_name": "h1"}},
	}}}
	config := json.RawMessage(`{"url":"https://example.com","selector":"h1","always_new_tab":true}`)
	observation, err := NewCSSText(client).Execute(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	if observation.Result["text"] != "Meerkit ready" || observation.Result["tag_name"] != "h1" {
		t.Fatalf("unexpected CSS text observation: %+v", observation)
	}
	request := client.requests[0]
	if request.KeepTab || request.Actions[0].Params["reuse"] != false {
		t.Fatalf("always_new_tab was not applied: %+v", request)
	}
}

func TestResponseModuleReturnsLatestMatchingResponse(t *testing.T) {
	client := &fakeBrowserClient{result: sdk.BrowserRunResult{Duration: 12, Actions: []sdk.BrowserActionResult{{ID: "open", Data: map[string]any{"reused": true}}}, Network: []sdk.BrowserNetworkResult{
		{CaptureID: "response", URL: "https://example.com/api/status?seq=1", Status: 202, Body: `{"ready":false}`},
		{CaptureID: "other", URL: "https://example.com/ignored", Status: 500},
		{CaptureID: "response", URL: "https://example.com/api/status?seq=2", Method: "GET", Status: 200, MimeType: "application/json", Body: `{"ready":true}`},
	}}}
	config := json.RawMessage(`{"url":"https://example.com","url_contains":"/api/status"}`)
	observation, err := NewResponse(client).Execute(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	if observation.Result["status"] != 200 || observation.Result["url"] != "https://example.com/api/status?seq=2" {
		t.Fatalf("latest response was not selected: %+v", observation)
	}
	request := client.requests[0]
	if len(request.NetworkCaptures) != 1 || request.NetworkCaptures[0].URLContains != "/api/status" {
		t.Fatalf("unexpected capture rule: %+v", request.NetworkCaptures)
	}
}

func TestExampleModulesRejectInvalidConfig(t *testing.T) {
	client := &fakeBrowserClient{}
	tests := []struct {
		module sdk.Module
		config string
	}{
		{NewHTML(client), `{"url":"javascript:alert(1)"}`},
		{NewCSSText(client), `{"url":"https://example.com","selector":""}`},
		{NewResponse(client), `{"url":"https://example.com","url_contains":""}`},
	}
	for _, test := range tests {
		if err := test.module.ValidateConfig(json.RawMessage(test.config)); err == nil {
			t.Fatalf("invalid config accepted by %T: %s", test.module, test.config)
		}
	}
}

func containsExample(value string) bool {
	for index := 0; index+7 <= len(value); index++ {
		chunk := value[index : index+7]
		if chunk == "example" || chunk == "Example" {
			return true
		}
	}
	return false
}
