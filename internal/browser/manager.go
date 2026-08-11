package browser

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/hanxuanyu/meerkit/sdk"
)

const (
	ProtocolVersion        = 1
	ExtensionWebSocketPath = "/api/v1/browser/extension/ws"
	maxActions             = 64
	maxNetworkCaptures     = 32
	maxWireMessageBytes    = 16 << 20
	maxAgentCapabilities   = 128
)

type AgentInfo struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Version      string    `json:"version"`
	Protocol     int       `json:"protocol"`
	Capabilities []string  `json:"capabilities"`
	ConnectedAt  time.Time `json:"connected_at"`
	LastSeenAt   time.Time `json:"last_seen_at"`
}

type wireMessage struct {
	Protocol int             `json:"protocol"`
	Type     string          `json:"type"`
	ID       string          `json:"id,omitempty"`
	Command  string          `json:"command,omitempty"`
	Token    string          `json:"token,omitempty"`
	Payload  json.RawMessage `json:"payload,omitempty"`
	Error    string          `json:"error,omitempty"`
}

type helloPayload struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Version      string   `json:"version"`
	Capabilities []string `json:"capabilities"`
}

type pendingResponse struct {
	message wireMessage
	err     error
}

type agentConnection struct {
	manager   *Manager
	websocket *websocket.Conn
	infoMu    sync.RWMutex
	info      AgentInfo
	sendMu    sync.Mutex
	pendingMu sync.Mutex
	pending   map[string]chan pendingResponse
	done      chan struct{}
	closeOnce sync.Once
}

type Manager struct {
	mu        sync.RWMutex
	rotateMu  sync.Mutex
	token     string
	tokenPath string
	agents    map[string]*agentConnection
	upgrader  websocket.Upgrader
}

func NewManager(pairingToken string) (*Manager, error) {
	if strings.TrimSpace(pairingToken) == "" {
		return nil, errors.New("browser pairing token cannot be empty")
	}
	return &Manager{token: pairingToken, agents: make(map[string]*agentConnection), upgrader: websocket.Upgrader{ReadBufferSize: 32 << 10, WriteBufferSize: 32 << 10, CheckOrigin: allowExtensionOrigin}}, nil
}

func allowExtensionOrigin(request *http.Request) bool {
	origin := request.Header.Get("Origin")
	if origin == "" || strings.HasPrefix(origin, "chrome-extension://") {
		return true
	}
	parsed, err := url.Parse(origin)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && (parsed.Hostname() == "localhost" || parsed.Hostname() == "127.0.0.1")
}

func (m *Manager) PairingToken() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.token
}

func (m *Manager) SetPairingToken(token string) error {
	if strings.TrimSpace(token) == "" {
		return errors.New("browser pairing token cannot be empty")
	}
	m.mu.Lock()
	m.token = token
	connections := make([]*agentConnection, 0, len(m.agents))
	for _, connection := range m.agents {
		connections = append(connections, connection)
	}
	m.mu.Unlock()
	for _, connection := range connections {
		connection.close(errors.New("browser pairing token rotated"))
	}
	return nil
}

func (m *Manager) RotatePairingToken() (string, error) {
	m.rotateMu.Lock()
	defer m.rotateMu.Unlock()
	if m.tokenPath == "" {
		return "", errors.New("browser pairing token is not persisted")
	}
	token, err := GenerateToken()
	if err != nil {
		return "", err
	}
	if err := writeToken(m.tokenPath, token); err != nil {
		return "", err
	}
	if err := m.SetPairingToken(token); err != nil {
		return "", err
	}
	return token, nil
}

func (m *Manager) HandleExtension(w http.ResponseWriter, request *http.Request) {
	connection, err := m.upgrader.Upgrade(w, request, nil)
	if err != nil {
		return
	}
	connection.SetReadLimit(maxWireMessageBytes)
	connection.SetReadDeadline(time.Now().Add(10 * time.Second))
	var hello wireMessage
	if err := connection.ReadJSON(&hello); err != nil {
		_ = connection.Close()
		return
	}
	if hello.Type != "hello" || hello.Protocol != ProtocolVersion || !m.validToken(hello.Token) {
		_ = connection.WriteJSON(wireMessage{Protocol: ProtocolVersion, Type: "error", Error: "browser extension pairing failed"})
		_ = connection.Close()
		return
	}
	var payload helloPayload
	if err := json.Unmarshal(hello.Payload, &payload); err != nil || !validAgentHello(payload) {
		_ = connection.Close()
		return
	}
	info := AgentInfo{ID: payload.ID, Name: payload.Name, Version: payload.Version, Protocol: hello.Protocol, Capabilities: payload.Capabilities, ConnectedAt: time.Now().UTC(), LastSeenAt: time.Now().UTC()}
	agent := &agentConnection{manager: m, websocket: connection, info: info, pending: make(map[string]chan pendingResponse), done: make(chan struct{})}
	if err := m.addAgent(agent); err != nil {
		_ = connection.Close()
		return
	}
	connection.SetReadDeadline(time.Now().Add(45 * time.Second))
	connection.SetPongHandler(func(string) error { connection.SetReadDeadline(time.Now().Add(45 * time.Second)); return nil })
	if err := agent.write(wireMessage{Protocol: ProtocolVersion, Type: "welcome", Payload: json.RawMessage(`{"heartbeat_seconds":15}`)}); err != nil {
		agent.close(err)
		return
	}
	go agent.pingLoop()
	agent.readLoop()
}

func (m *Manager) validToken(value string) bool {
	m.mu.RLock()
	token := m.token
	m.mu.RUnlock()
	return subtle.ConstantTimeCompare([]byte(value), []byte(token)) == 1
}

func (m *Manager) addAgent(agent *agentConnection) error {
	m.mu.Lock()
	previous := m.agents[agent.info.ID]
	m.agents[agent.info.ID] = agent
	m.mu.Unlock()
	if previous != nil {
		previous.close(errors.New("browser extension replaced by a newer connection"))
	}
	return nil
}

func (m *Manager) removeAgent(agent *agentConnection) {
	m.mu.Lock()
	if m.agents[agent.info.ID] == agent {
		delete(m.agents, agent.info.ID)
	}
	m.mu.Unlock()
}

func (m *Manager) Agents() []AgentInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]AgentInfo, 0, len(m.agents))
	for _, agent := range m.agents {
		agent.infoMu.RLock()
		value := agent.info
		agent.infoMu.RUnlock()
		result = append(result, value)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

func (m *Manager) Execute(ctx context.Context, request sdk.BrowserRunRequest) (sdk.BrowserRunResult, error) {
	if len(request.Actions) == 0 || len(request.Actions) > maxActions {
		return sdk.BrowserRunResult{}, fmt.Errorf("browser action count must be between 1 and %d", maxActions)
	}
	if len(request.NetworkCaptures) > maxNetworkCaptures {
		return sdk.BrowserRunResult{}, fmt.Errorf("browser network capture count cannot exceed %d", maxNetworkCaptures)
	}
	agent := m.selectAgent(request.AgentID)
	if agent == nil {
		if request.AgentID != "" {
			return sdk.BrowserRunResult{}, fmt.Errorf("browser agent %q is not connected", request.AgentID)
		}
		return sdk.BrowserRunResult{}, errors.New("no browser extension is connected")
	}
	if err := agent.supports(request); err != nil {
		return sdk.BrowserRunResult{}, err
	}
	data, err := json.Marshal(request)
	if err != nil {
		return sdk.BrowserRunResult{}, err
	}
	requestID := uuid.NewString()
	response := make(chan pendingResponse, 1)
	agent.pendingMu.Lock()
	agent.pending[requestID] = response
	agent.pendingMu.Unlock()
	defer func() {
		agent.pendingMu.Lock()
		delete(agent.pending, requestID)
		agent.pendingMu.Unlock()
	}()
	if err := agent.write(wireMessage{Protocol: ProtocolVersion, Type: "command", ID: requestID, Command: "browser.run", Payload: data}); err != nil {
		return sdk.BrowserRunResult{}, err
	}
	select {
	case <-ctx.Done():
		return sdk.BrowserRunResult{}, ctx.Err()
	case result := <-response:
		if result.err != nil {
			return sdk.BrowserRunResult{}, result.err
		}
		if result.message.Error != "" {
			return sdk.BrowserRunResult{}, errors.New(result.message.Error)
		}
		var output sdk.BrowserRunResult
		if err := json.Unmarshal(result.message.Payload, &output); err != nil {
			return sdk.BrowserRunResult{}, fmt.Errorf("decode browser result: %w", err)
		}
		return output, nil
	}
}

func (m *Manager) selectAgent(id string) *agentConnection {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if id != "" {
		return m.agents[id]
	}
	ids := make([]string, 0, len(m.agents))
	for agentID := range m.agents {
		ids = append(ids, agentID)
	}
	sort.Strings(ids)
	if len(ids) > 0 {
		return m.agents[ids[0]]
	}
	return nil
}

func validAgentHello(payload helloPayload) bool {
	return len(strings.TrimSpace(payload.ID)) > 0 && len(payload.ID) <= 128 && len(payload.Name) <= 128 && len(payload.Version) <= 64 && len(payload.Capabilities) <= maxAgentCapabilities
}

func (agent *agentConnection) supports(request sdk.BrowserRunRequest) error {
	agent.infoMu.RLock()
	capabilities := make(map[string]struct{}, len(agent.info.Capabilities))
	for _, capability := range agent.info.Capabilities {
		capabilities[capability] = struct{}{}
	}
	agentID := agent.info.ID
	agent.infoMu.RUnlock()
	required := make(map[string]struct{}, len(request.Actions)+1)
	for _, action := range request.Actions {
		if action.Type == "" {
			return errors.New("browser action type cannot be empty")
		}
		required[action.Type] = struct{}{}
	}
	if len(request.NetworkCaptures) > 0 {
		required["network.capture"] = struct{}{}
	}
	for capability := range required {
		if _, ok := capabilities[capability]; !ok {
			return fmt.Errorf("browser agent %q does not support capability %q", agentID, capability)
		}
	}
	return nil
}

func (agent *agentConnection) write(message wireMessage) error {
	agent.sendMu.Lock()
	defer agent.sendMu.Unlock()
	return agent.websocket.WriteJSON(message)
}

func (agent *agentConnection) readLoop() {
	defer agent.close(errors.New("browser extension disconnected"))
	for {
		var message wireMessage
		if err := agent.websocket.ReadJSON(&message); err != nil {
			return
		}
		agent.infoMu.Lock()
		agent.info.LastSeenAt = time.Now().UTC()
		agent.infoMu.Unlock()
		switch message.Type {
		case "response":
			if message.Protocol != ProtocolVersion || message.ID == "" {
				continue
			}
			agent.pendingMu.Lock()
			response := agent.pending[message.ID]
			delete(agent.pending, message.ID)
			agent.pendingMu.Unlock()
			if response != nil {
				select {
				case response <- pendingResponse{message: message}:
				default:
				}
			}
		case "ping":
			_ = agent.write(wireMessage{Protocol: ProtocolVersion, Type: "pong"})
		}
	}
}

func (agent *agentConnection) pingLoop() {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			agent.sendMu.Lock()
			err := agent.websocket.WriteControl(websocket.PingMessage, nil, time.Now().Add(5*time.Second))
			agent.sendMu.Unlock()
			if err != nil {
				agent.close(err)
				return
			}
		case <-agent.done:
			return
		}
	}
}

func (agent *agentConnection) close(cause error) {
	agent.closeOnce.Do(func() {
		close(agent.done)
		agent.manager.removeAgent(agent)
		_ = agent.websocket.Close()
		agent.pendingMu.Lock()
		for _, response := range agent.pending {
			select {
			case response <- pendingResponse{err: cause}:
			default:
			}
		}
		agent.pending = make(map[string]chan pendingResponse)
		agent.pendingMu.Unlock()
	})
}
