package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hanxuanyu/meerkit/sdk"
	"meerkit/internal/browser"
)

const testToken = "test-mcp-token-with-at-least-32-characters"

type fakeBrowserController struct {
	agents         []browser.AgentInfo
	targets        sdk.BrowserTargets
	actionResult   sdk.BrowserActionResult
	actionRequest  sdk.BrowserActionRequest
	selectorResult sdk.BrowserSelectorCandidates
	captures       []sdk.BrowserNetworkSession
	stopResult     sdk.BrowserNetworkStopResult
}

func (f *fakeBrowserController) Agents() []browser.AgentInfo { return f.agents }
func (f *fakeBrowserController) Targets(context.Context, string) (sdk.BrowserTargets, error) {
	return f.targets, nil
}
func (f *fakeBrowserController) ExecuteAction(_ context.Context, request sdk.BrowserActionRequest) (sdk.BrowserActionResult, error) {
	f.actionRequest = request
	return f.actionResult, nil
}
func (f *fakeBrowserController) SelectorCandidates(context.Context, sdk.BrowserSelectorCandidatesRequest) (sdk.BrowserSelectorCandidates, error) {
	return f.selectorResult, nil
}
func (f *fakeBrowserController) StartNetworkCapture(_ context.Context, request sdk.BrowserNetworkStartRequest) (sdk.BrowserNetworkSession, error) {
	value := sdk.BrowserNetworkSession{ID: "capture-1", Target: request.Target, Status: "active"}
	f.captures = append(f.captures, value)
	return value, nil
}
func (f *fakeBrowserController) StopNetworkCapture(context.Context, string) (sdk.BrowserNetworkStopResult, error) {
	return f.stopResult, nil
}
func (f *fakeBrowserController) Captures() []sdk.BrowserNetworkSession { return f.captures }

func TestHandlerRequiresBearerToken(t *testing.T) {
	handler, err := New(&fakeBrowserController{}, Options{Token: testToken, Version: "test"})
	if err != nil {
		t.Fatal(err)
	}
	for _, authorization := range []string{"", "Bearer wrong-token"} {
		request := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader([]byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`)))
		request.Header.Set("Authorization", authorization)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("authorization %q status = %d, want %d", authorization, response.Code, http.StatusUnauthorized)
		}
	}
}

func TestMCPListsAndCallsBrowserTools(t *testing.T) {
	controller := &fakeBrowserController{agents: []browser.AgentInfo{{ID: "agent-1", Name: "Chrome"}}, actionResult: sdk.BrowserActionResult{Type: "page.info", Success: true, Data: map[string]any{"title": "Example"}}}
	handler, err := New(controller, Options{Token: testToken, Version: "test"})
	if err != nil {
		t.Fatal(err)
	}
	initialize := callMCP(t, handler, "", `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`)
	if initialize.Code != http.StatusOK {
		t.Fatalf("initialize status = %d, body = %s", initialize.Code, initialize.Body.String())
	}
	sessionID := initialize.Header().Get("Mcp-Session-Id")
	if sessionID == "" {
		t.Fatal("initialize did not return an MCP session ID")
	}
	initialized := callMCP(t, handler, sessionID, `{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	if initialized.Code != http.StatusAccepted {
		t.Fatalf("initialized status = %d, body = %s", initialized.Code, initialized.Body.String())
	}
	toolsResponse := callMCP(t, handler, sessionID, `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`)
	var toolsPayload struct {
		Result struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(toolsResponse.Body.Bytes(), &toolsPayload); err != nil {
		t.Fatal(err)
	}
	if len(toolsPayload.Result.Tools) != 8 {
		t.Fatalf("tool count = %d, want 8: %s", len(toolsPayload.Result.Tools), toolsResponse.Body.String())
	}
	callResponse := callMCP(t, handler, sessionID, `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"browser_list_agents","arguments":{}}}`)
	if callResponse.Code != http.StatusOK || !bytes.Contains(callResponse.Body.Bytes(), []byte(`"agent-1"`)) {
		t.Fatalf("list agents response = %d, body = %s", callResponse.Code, callResponse.Body.String())
	}
	actionResponse := callMCP(t, handler, sessionID, `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"browser_execute_action","arguments":{"agent_id":"agent-1","tab_id":21,"action":"page.info"}}}`)
	if actionResponse.Code != http.StatusOK || !bytes.Contains(actionResponse.Body.Bytes(), []byte(`"Example"`)) {
		t.Fatalf("execute action response = %d, body = %s", actionResponse.Code, actionResponse.Body.String())
	}
	if controller.actionRequest.Action.Type != "page.info" || controller.actionRequest.Target.TabID != 21 || controller.actionRequest.TimeoutMS != 60000 {
		t.Fatalf("unexpected browser action request: %#v", controller.actionRequest)
	}
}

func TestScreenshotBecomesMCPImageContent(t *testing.T) {
	result := sdk.BrowserActionResult{Type: "page.screenshot", Success: true, Data: map[string]any{"data_url": "data:image/png;base64,AQID", "format": "png"}}
	content, sanitized := actionContent(result)
	if len(content.Content) != 2 {
		t.Fatalf("content count = %d, want text and image", len(content.Content))
	}
	if _, exists := sanitized.Data["data_url"]; exists {
		t.Fatal("structured screenshot output still contains the data URL")
	}
	if sanitized.Data["image_content"] != true {
		t.Fatalf("image marker = %#v", sanitized.Data["image_content"])
	}
	encoded, err := json.Marshal(content.Content[1])
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(encoded, []byte(`"mimeType":"image/png"`)) || !bytes.Contains(encoded, []byte(`"data":"AQID"`)) {
		t.Fatalf("unexpected MCP image content: %s", encoded)
	}
}

func callMCP(t *testing.T, handler http.Handler, sessionID, payload string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBufferString(payload))
	request.Header.Set("Authorization", "Bearer "+testToken)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")
	if sessionID != "" {
		request.Header.Set("Mcp-Session-Id", sessionID)
		request.Header.Set("Mcp-Protocol-Version", "2025-11-25")
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}
