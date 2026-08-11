package browser

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/hanxuanyu/meerkit/sdk"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestOpenManagerPersistsAndRotatesPairingToken(t *testing.T) {
	dataDir := t.TempDir()
	first, err := OpenManager(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	second, err := OpenManager(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	if first.PairingToken() == "" || second.PairingToken() != first.PairingToken() {
		t.Fatalf("pairing token was not persisted: first=%q second=%q", first.PairingToken(), second.PairingToken())
	}
	original := second.PairingToken()
	rotated, err := second.RotatePairingToken()
	if err != nil {
		t.Fatal(err)
	}
	if rotated == "" || rotated == original || second.PairingToken() != rotated {
		t.Fatalf("pairing token was not rotated: original=%q rotated=%q", original, rotated)
	}
	contents, err := os.ReadFile(filepath.Join(dataDir, "browser", pairingTokenFilename))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(contents)) != rotated {
		t.Fatalf("persisted token = %q, want %q", strings.TrimSpace(string(contents)), rotated)
	}
}

func TestManagerRoutesRequestToConnectedAgent(t *testing.T) {
	manager, err := NewManager("pairing-secret")
	if err != nil {
		t.Fatal(err)
	}
	connection := connectTestAgent(t, manager, "pairing-secret", "agent-b", []string{"dom.query"})
	responded := make(chan error, 1)
	go respondToBrowserCommand(connection, responded, "agent-b")

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	result, err := manager.Execute(ctx, sdk.BrowserRunRequest{Actions: []sdk.BrowserAction{{ID: "query", Type: "dom.query", Params: map[string]any{"selector": "main"}}}})
	if err != nil {
		t.Fatal(err)
	}
	if result.AgentID != "agent-b" || len(result.Actions) != 1 || !result.Actions[0].Success {
		t.Fatalf("unexpected browser result: %#v", result)
	}
	if err := <-responded; err != nil {
		t.Fatal(err)
	}
	if agents := manager.Agents(); len(agents) != 1 || agents[0].ID != "agent-b" {
		t.Fatalf("unexpected connected agents: %#v", agents)
	}
}

func TestManagerRejectsUnsupportedAgentCapability(t *testing.T) {
	manager, err := NewManager("pairing-secret")
	if err != nil {
		t.Fatal(err)
	}
	connectTestAgent(t, manager, "pairing-secret", "limited-agent", []string{"tab.open"})
	_, err = manager.Execute(context.Background(), sdk.BrowserRunRequest{Actions: []sdk.BrowserAction{{Type: "dom.query"}}})
	if err == nil || !strings.Contains(err.Error(), "does not support capability") {
		t.Fatalf("unsupported capability error = %v", err)
	}
}

func TestCapabilityServerAuthenticatesAndRoutesRequest(t *testing.T) {
	manager, err := NewManager("pairing-secret")
	if err != nil {
		t.Fatal(err)
	}
	connection := connectTestAgent(t, manager, "pairing-secret", "agent-capability", []string{"page.wait"})
	server, err := StartCapabilityServer(manager, "capability-secret")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(server.Close)

	unauthorized, err := sdk.NewBrowserClient(server.Endpoint, "wrong-secret")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = unauthorized.Close() })
	request := sdk.BrowserRunRequest{Actions: []sdk.BrowserAction{{ID: "wait", Type: "page.wait"}}}
	if _, err := unauthorized.Run(context.Background(), request); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("unauthorized status = %v, want %v (error: %v)", status.Code(err), codes.Unauthenticated, err)
	}

	responded := make(chan error, 1)
	go respondToBrowserCommand(connection, responded, "agent-capability")
	authorized, err := sdk.NewBrowserClient(server.Endpoint, "capability-secret")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = authorized.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	result, err := authorized.Run(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if result.AgentID != "agent-capability" {
		t.Fatalf("agent id = %q, want agent-capability", result.AgentID)
	}
	if err := <-responded; err != nil {
		t.Fatal(err)
	}
}

func connectTestAgent(t *testing.T, manager *Manager, token, id string, capabilities []string) *websocket.Conn {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(manager.HandleExtension))
	t.Cleanup(server.Close)
	websocketURL := "ws" + strings.TrimPrefix(server.URL, "http")
	header := http.Header{"Origin": []string{"chrome-extension://meerkit-test"}}
	connection, response, err := websocket.DefaultDialer.Dial(websocketURL, header)
	if err != nil {
		if response != nil {
			t.Fatalf("connect test agent: %v (status %d)", err, response.StatusCode)
		}
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = connection.Close() })
	payload, err := json.Marshal(helloPayload{ID: id, Name: "Test Chrome", Version: "0.1.0", Capabilities: capabilities})
	if err != nil {
		t.Fatal(err)
	}
	if err := connection.WriteJSON(wireMessage{Protocol: ProtocolVersion, Type: "hello", Token: token, Payload: payload}); err != nil {
		t.Fatal(err)
	}
	var welcome wireMessage
	if err := connection.ReadJSON(&welcome); err != nil {
		t.Fatal(err)
	}
	if welcome.Type != "welcome" {
		t.Fatalf("welcome message = %#v", welcome)
	}
	return connection
}

func respondToBrowserCommand(connection *websocket.Conn, done chan<- error, agentID string) {
	var command wireMessage
	if err := connection.ReadJSON(&command); err != nil {
		done <- err
		return
	}
	var request sdk.BrowserRunRequest
	if err := json.Unmarshal(command.Payload, &request); err != nil {
		done <- err
		return
	}
	result := sdk.BrowserRunResult{AgentID: agentID, Duration: 12, Actions: []sdk.BrowserActionResult{{ID: request.Actions[0].ID, Type: request.Actions[0].Type, Success: true}}}
	payload, err := json.Marshal(result)
	if err == nil {
		err = connection.WriteJSON(wireMessage{Protocol: ProtocolVersion, Type: "response", ID: command.ID, Payload: payload})
	}
	done <- err
}

func TestAllowExtensionOrigin(t *testing.T) {
	for _, value := range []string{"", "chrome-extension://agent", "http://localhost", "http://127.0.0.1", "http://localhost:5173", "https://127.0.0.1:8443"} {
		request := &http.Request{Header: http.Header{"Origin": []string{value}}, URL: &url.URL{}}
		if !allowExtensionOrigin(request) {
			t.Fatalf("origin %q should be accepted", value)
		}
	}
	request := &http.Request{Header: http.Header{"Origin": []string{"https://example.com"}}, URL: &url.URL{}}
	if allowExtensionOrigin(request) {
		t.Fatal("unexpected web origin was accepted")
	}
}
