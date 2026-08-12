package browser

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/hanxuanyu/meerkit/sdk"
)

func TestManagerRejectsMissingExtension(t *testing.T) {
	manager, err := NewManager("secret")
	if err != nil {
		t.Fatal(err)
	}
	_, err = manager.ExecuteAction(t.Context(), sdk.BrowserActionRequest{Target: sdk.BrowserTarget{TabID: 1}, Action: sdk.BrowserAction{Type: "page.wait"}})
	if err == nil {
		t.Fatal("expected unavailable extension error")
	}
}

func TestAgentAssemblesChunkedResponse(t *testing.T) {
	response := make(chan pendingResponse, 1)
	agent := &agentConnection{
		pending: map[string]chan pendingResponse{"request": response},
		chunks:  make(map[string]*responseAssembly),
	}
	payload := `{"id":"shot","type":"page.screenshot","success":true,"data":{"data_url":"data:image/png;base64,AAAA"}}`
	split := len(payload) / 2
	agent.handleResponseChunk(wireMessage{Protocol: ProtocolVersion, Type: "response_chunk", ID: "request", Sequence: 0, Total: 2, Chunk: payload[:split]})
	select {
	case <-response:
		t.Fatal("response completed before the final chunk")
	default:
	}
	agent.handleResponseChunk(wireMessage{Protocol: ProtocolVersion, Type: "response_chunk", ID: "request", Sequence: 1, Total: 2, Chunk: payload[split:]})
	result := <-response
	if result.err != nil {
		t.Fatal(result.err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(result.message.Payload, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["type"] != "page.screenshot" {
		t.Fatalf("unexpected response: %#v", decoded)
	}
}

func TestAgentRejectsOversizedChunkedResponse(t *testing.T) {
	response := make(chan pendingResponse, 1)
	agent := &agentConnection{
		pending: map[string]chan pendingResponse{"request": response},
		chunks: map[string]*responseAssembly{
			"request": {total: 1, size: maxChunkedResponseBytes},
		},
	}
	agent.handleResponseChunk(wireMessage{Protocol: ProtocolVersion, Type: "response_chunk", ID: "request", Sequence: 0, Total: 1, Chunk: "a"})
	result := <-response
	if result.err == nil || !strings.Contains(result.err.Error(), "exceeded limits") {
		t.Fatalf("unexpected error: %v", result.err)
	}
}

func TestTargetsRejectMissingExtension(t *testing.T) {
	manager, err := NewManager("secret")
	if err != nil {
		t.Fatal(err)
	}
	_, err = manager.Targets(t.Context(), "agent")
	if err == nil {
		t.Fatal("expected unavailable extension error")
	}
}
