package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/hanxuanyu/meerkit/sdk"
	"meerkit/internal/browser"
)

func TestRunBrowserRoutesRequestToAgent(t *testing.T) {
	manager, err := browser.NewManager("debug-secret")
	if err != nil {
		t.Fatal(err)
	}
	agentServer := httptest.NewServer(http.HandlerFunc(manager.HandleExtension))
	t.Cleanup(agentServer.Close)
	connection, response, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(agentServer.URL, "http"), http.Header{"Origin": []string{"chrome-extension://debug-test"}})
	if err != nil {
		if response != nil {
			t.Fatalf("connect browser agent: %v (status %d)", err, response.StatusCode)
		}
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = connection.Close() })
	if err := connection.WriteJSON(map[string]any{
		"protocol": 1,
		"type":     "hello",
		"token":    "debug-secret",
		"payload": map[string]any{
			"id": "debug-agent", "name": "Debug Chrome", "version": "test", "capabilities": []string{"tab.open", "dom.query"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	var welcome map[string]any
	if err := connection.ReadJSON(&welcome); err != nil {
		t.Fatal(err)
	}

	forwarded := make(chan sdk.BrowserRunRequest, 1)
	agentError := make(chan error, 1)
	go func() {
		var command struct {
			ID      string          `json:"id"`
			Payload json.RawMessage `json:"payload"`
		}
		if readErr := connection.ReadJSON(&command); readErr != nil {
			agentError <- readErr
			return
		}
		var request sdk.BrowserRunRequest
		if decodeErr := json.Unmarshal(command.Payload, &request); decodeErr != nil {
			agentError <- decodeErr
			return
		}
		forwarded <- request
		result := sdk.BrowserRunResult{AgentID: "debug-agent", TabID: 42, Duration: 18, Actions: []sdk.BrowserActionResult{{ID: "open", Type: "tab.open", Success: true}}}
		agentError <- connection.WriteJSON(map[string]any{"protocol": 1, "type": "response", "id": command.ID, "payload": result})
	}()

	server := &APIServer{browser: manager}
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/api/v1/browser/run", strings.NewReader(`{"agent_id":"debug-agent","timeout_ms":500,"keep_tab":true,"actions":[{"id":"open","type":"tab.open","params":{"url":"https://example.com"}},{"id":"query","type":"dom.query","params":{"selector":"main"}}]}`))
	server.runBrowser(context)
	if recorder.Code != http.StatusOK {
		t.Fatalf("run browser status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	request := <-forwarded
	if request.AgentID != "debug-agent" || request.TimeoutMS != 1000 || len(request.Actions) != 2 {
		t.Fatalf("unexpected forwarded request: %#v", request)
	}
	if err := <-agentError; err != nil {
		t.Fatal(err)
	}
	var result sdk.BrowserRunResult
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.AgentID != "debug-agent" || result.TabID != 42 || len(result.Actions) != 1 {
		t.Fatalf("unexpected browser result: %#v", result)
	}
}

func TestRunBrowserRequiresManager(t *testing.T) {
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/api/v1/browser/run", strings.NewReader(`{"actions":[{"type":"tab.open"}]}`))
	new(APIServer).runBrowser(context)
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("run browser status = %d, want %d", recorder.Code, http.StatusServiceUnavailable)
	}
}
