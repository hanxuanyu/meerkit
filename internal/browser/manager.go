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
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/wrapperspb"
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
	mu            sync.RWMutex
	rotateMu      sync.Mutex
	token         string
	tokenPath     string
	agents        map[string]*agentConnection
	upgrader      websocket.Upgrader
	capturesMu    sync.RWMutex
	captures      map[string]*captureSession
	subscribersMu sync.RWMutex
	subscribers   map[chan StreamEvent]struct{}
}

type captureSession struct {
	Session sdk.BrowserNetworkSession
	Rules   []sdk.BrowserNetworkCaptureRule
	Events  []sdk.BrowserNetworkResult
	Owner   string
}

type StreamEvent struct {
	Type      string `json:"type"`
	AgentID   string `json:"agent_id,omitempty"`
	SessionID string `json:"session_id,omitempty"`
	Payload   any    `json:"payload,omitempty"`
	Owner     string `json:"-"`
}

func NewManager(pairingToken string) (*Manager, error) {
	if strings.TrimSpace(pairingToken) == "" {
		return nil, errors.New("browser pairing token cannot be empty")
	}
	return &Manager{token: pairingToken, agents: make(map[string]*agentConnection), captures: make(map[string]*captureSession), subscribers: make(map[chan StreamEvent]struct{}), upgrader: websocket.Upgrader{ReadBufferSize: 32 << 10, WriteBufferSize: 32 << 10, CheckOrigin: allowExtensionOrigin}}, nil
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
	m.removeCaptures(func(capture *captureSession) bool {
		return capture.Session.Target.AgentID == agent.info.ID
	}, "browser agent disconnected")
	m.publish(StreamEvent{Type: "browser.targets.changed", AgentID: agent.info.ID, Payload: map[string]any{"available": false}})
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

func (m *Manager) sendCommand(ctx context.Context, agentID, command string, request, output any) error {
	agent := m.selectAgent(agentID)
	if agent == nil {
		if agentID != "" {
			return fmt.Errorf("browser agent %q is not connected", agentID)
		}
		return errors.New("no browser extension is connected")
	}
	data, err := json.Marshal(request)
	if err != nil {
		return err
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
	if err := agent.write(wireMessage{Protocol: ProtocolVersion, Type: "command", ID: requestID, Command: command, Payload: data}); err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case result := <-response:
		if result.err != nil {
			return result.err
		}
		if result.message.Error != "" {
			return errors.New(result.message.Error)
		}
		if output == nil {
			return nil
		}
		if err := json.Unmarshal(result.message.Payload, output); err != nil {
			return fmt.Errorf("decode browser result: %w", err)
		}
		return nil
	}
}

func (m *Manager) ExecuteAction(ctx context.Context, request sdk.BrowserActionRequest) (sdk.BrowserActionResult, error) {
	if err := ValidateBrowserActionRequest(request); err != nil {
		return sdk.BrowserActionResult{}, err
	}
	agent := m.selectAgent(request.Target.AgentID)
	if agent == nil {
		return sdk.BrowserActionResult{}, errors.New("browser extension is not connected")
	}
	if !agent.supportsCapability(actionCapability(request.Action.Type)) {
		return sdk.BrowserActionResult{}, fmt.Errorf("browser agent %q does not support capability %q", agent.info.ID, actionCapability(request.Action.Type))
	}
	var result sdk.BrowserActionResult
	err := m.sendCommand(ctx, request.Target.AgentID, "browser.action", request, &result)
	return result, err
}

func (m *Manager) Targets(ctx context.Context, agentID string) (sdk.BrowserTargets, error) {
	var result sdk.BrowserTargets
	err := m.sendCommand(ctx, agentID, "browser.targets", sdk.BrowserTarget{AgentID: agentID}, &result)
	return result, err
}

func (m *Manager) StartNetworkCapture(ctx context.Context, request sdk.BrowserNetworkStartRequest) (sdk.BrowserNetworkSession, error) {
	return m.startNetworkCapture(ctx, "", request)
}

func (m *Manager) startNetworkCapture(ctx context.Context, owner string, request sdk.BrowserNetworkStartRequest) (sdk.BrowserNetworkSession, error) {
	if request.Target.TabID <= 0 {
		return sdk.BrowserNetworkSession{}, errors.New("network capture requires tab_id")
	}
	if len(request.Rules) == 0 || len(request.Rules) > maxNetworkCaptures {
		return sdk.BrowserNetworkSession{}, fmt.Errorf("network capture rule count must be between 1 and %d", maxNetworkCaptures)
	}
	for _, rule := range request.Rules {
		if len(rule.URLContains) > 4096 || rule.MaxBodyBytes < 0 || rule.MaxBodyBytes > 1048576 {
			return sdk.BrowserNetworkSession{}, errors.New("network capture rule is invalid")
		}
	}
	session := sdk.BrowserNetworkSession{ID: uuid.NewString(), Target: request.Target, Status: "starting", StartedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	payload := struct {
		SessionID string `json:"session_id"`
		sdk.BrowserNetworkStartRequest
	}{SessionID: session.ID, BrowserNetworkStartRequest: request}
	m.capturesMu.Lock()
	m.captures[session.ID] = &captureSession{Session: session, Rules: request.Rules, Owner: owner}
	m.capturesMu.Unlock()
	if err := m.sendCommand(ctx, request.Target.AgentID, "browser.network.start", payload, &session); err != nil {
		m.capturesMu.Lock()
		delete(m.captures, session.ID)
		m.capturesMu.Unlock()
		return sdk.BrowserNetworkSession{}, err
	}
	m.capturesMu.Lock()
	if capture := m.captures[session.ID]; capture != nil {
		capture.Session = session
	}
	m.capturesMu.Unlock()
	m.publish(StreamEvent{Type: "browser.network.status", AgentID: session.Target.AgentID, SessionID: session.ID, Payload: session})
	return session, nil
}

func (m *Manager) StopNetworkCapture(ctx context.Context, id string) (sdk.BrowserNetworkStopResult, error) {
	m.capturesMu.RLock()
	capture := m.captures[id]
	m.capturesMu.RUnlock()
	if capture == nil {
		return sdk.BrowserNetworkStopResult{}, errors.New("browser network capture was not found")
	}
	var session sdk.BrowserNetworkSession
	if err := m.sendCommand(ctx, capture.Session.Target.AgentID, "browser.network.stop", map[string]string{"session_id": id}, &session); err != nil {
		return sdk.BrowserNetworkStopResult{}, err
	}
	m.capturesMu.Lock()
	capture = m.captures[id]
	delete(m.captures, id)
	m.capturesMu.Unlock()
	if capture == nil {
		return sdk.BrowserNetworkStopResult{Session: session}, nil
	}
	if session.Error == "" {
		session.Error = capture.Session.Error
	}
	result := sdk.BrowserNetworkStopResult{Session: session, Events: append([]sdk.BrowserNetworkResult(nil), capture.Events...)}
	m.publish(StreamEvent{Type: "browser.network.status", AgentID: session.Target.AgentID, SessionID: id, Payload: session, Owner: capture.Owner})
	return result, nil
}

func (m *Manager) stopPluginCaptures(pluginID string) {
	m.capturesMu.RLock()
	ids := make([]string, 0)
	for id, capture := range m.captures {
		if capture.Owner == pluginID {
			ids = append(ids, id)
		}
	}
	m.capturesMu.RUnlock()
	for _, id := range ids {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_, err := m.StopNetworkCapture(ctx, id)
		cancel()
		if err != nil {
			m.removeCaptures(func(capture *captureSession) bool { return capture.Owner == pluginID }, "plugin browser session disconnected")
			return
		}
	}
}

func (m *Manager) removeCaptures(match func(*captureSession) bool, reason string) {
	m.capturesMu.Lock()
	removed := make([]StreamEvent, 0)
	for id, capture := range m.captures {
		if !match(capture) {
			continue
		}
		session := capture.Session
		session.Status = "stopped"
		session.Error = reason
		removed = append(removed, StreamEvent{Type: "browser.network.status", AgentID: session.Target.AgentID, SessionID: session.ID, Payload: session, Owner: capture.Owner})
		delete(m.captures, id)
	}
	m.capturesMu.Unlock()
	for _, event := range removed {
		m.publish(event)
	}
}

func (m *Manager) Captures() []sdk.BrowserNetworkSession {
	m.capturesMu.RLock()
	defer m.capturesMu.RUnlock()
	result := make([]sdk.BrowserNetworkSession, 0, len(m.captures))
	for _, capture := range m.captures {
		value := capture.Session
		value.Count = len(capture.Events)
		result = append(result, value)
	}
	return result
}
func (m *Manager) Subscribe() (<-chan StreamEvent, func()) {
	channel := make(chan StreamEvent, 128)
	m.subscribersMu.Lock()
	m.subscribers[channel] = struct{}{}
	m.subscribersMu.Unlock()
	return channel, func() {
		m.subscribersMu.Lock()
		if _, ok := m.subscribers[channel]; ok {
			delete(m.subscribers, channel)
			close(channel)
		}
		m.subscribersMu.Unlock()
	}
}
func (m *Manager) publish(event StreamEvent) {
	m.subscribersMu.RLock()
	defer m.subscribersMu.RUnlock()
	for channel := range m.subscribers {
		select {
		case channel <- event:
		default:
		}
	}
}

// ServePluginBridge accepts browser requests from a plugin over its existing go-plugin gRPC connection.
func (m *Manager) ServePluginBridge(ctx context.Context, pluginID string, conn grpc.ClientConnInterface, ready chan<- error) error {
	sessionCtx, cancelSession := context.WithCancel(ctx)
	defer cancelSession()
	defer m.stopPluginCaptures(pluginID)
	stream, err := sdk.NewBrowserBridgeClient(conn).Session(sessionCtx)
	if err != nil {
		if ready != nil {
			ready <- err
		}
		return err
	}
	first, err := stream.Recv()
	if err != nil {
		if ready != nil {
			ready <- err
		}
		return err
	}
	var hello sdk.BrowserBridgeEnvelope
	if json.Unmarshal(first.Value, &hello) != nil || hello.Type != "ready" {
		err = errors.New("plugin browser bridge did not send ready")
		if ready != nil {
			ready <- err
		}
		return err
	}
	if ready != nil {
		ready <- nil
	}
	send := make(chan sdk.BrowserBridgeEnvelope, 256)
	writerDone := make(chan error, 1)
	go func() {
		for {
			select {
			case <-sessionCtx.Done():
				writerDone <- sessionCtx.Err()
				return
			case envelope := <-send:
				raw, marshalErr := json.Marshal(envelope)
				if marshalErr == nil {
					marshalErr = stream.Send(wrapperspb.Bytes(raw))
				}
				if marshalErr != nil {
					writerDone <- marshalErr
					cancelSession()
					return
				}
			}
		}
	}()
	events, unsubscribe := m.Subscribe()
	defer unsubscribe()
	eventDone := make(chan error, 1)
	go func() {
		dropped := make(map[string]struct{})
		for {
			select {
			case <-sessionCtx.Done():
				eventDone <- sessionCtx.Err()
				return
			case event, ok := <-events:
				if !ok {
					return
				}
				if event.Type != "browser.network" && event.Type != "browser.network.status" && event.Type != "browser.targets.changed" {
					continue
				}
				if event.SessionID != "" {
					allowed := event.Owner == pluginID
					if event.Owner == "" {
						m.capturesMu.RLock()
						capture := m.captures[event.SessionID]
						allowed = capture != nil && capture.Owner == pluginID
						m.capturesMu.RUnlock()
					}
					if !allowed {
						continue
					}
				}
				payload, _ := json.Marshal(event.Payload)
				if event.SessionID != "" && event.Type == "browser.network.status" {
					if _, wasDropped := dropped[event.SessionID]; wasDropped {
						timer := time.NewTimer(time.Second)
						select {
						case send <- sdk.BrowserBridgeEnvelope{Type: "event", Operation: event.Type, Payload: payload}:
							timer.Stop()
							delete(dropped, event.SessionID)
							continue
						case <-timer.C:
							eventDone <- errors.New("plugin browser bridge control queue is full")
							cancelSession()
							return
						case <-sessionCtx.Done():
							timer.Stop()
							return
						}
					}
				}
				select {
				case send <- sdk.BrowserBridgeEnvelope{Type: "event", Operation: event.Type, Payload: payload}:
				default:
					if event.SessionID != "" {
						if _, exists := dropped[event.SessionID]; !exists {
							dropped[event.SessionID] = struct{}{}
							go func(id string) {
								stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
								defer stopCancel()
								m.capturesMu.Lock()
								if capture := m.captures[id]; capture != nil {
									capture.Session.Error = "plugin browser bridge event queue is full"
								}
								m.capturesMu.Unlock()
								_, _ = m.StopNetworkCapture(stopCtx, id)
							}(event.SessionID)
						}
						continue
					}
					eventDone <- errors.New("plugin browser bridge event queue is full")
					cancelSession()
					return
				}
			}
		}
	}()
	var requestsMu sync.Mutex
	requests := make(map[string]context.CancelFunc)
	defer func() {
		requestsMu.Lock()
		for _, cancel := range requests {
			cancel()
		}
		requestsMu.Unlock()
	}()
	for {
		select {
		case writerErr := <-writerDone:
			return writerErr
		case eventErr := <-eventDone:
			return eventErr
		default:
		}
		value, err := stream.Recv()
		if err != nil {
			return err
		}
		var request sdk.BrowserBridgeEnvelope
		if json.Unmarshal(value.Value, &request) != nil {
			continue
		}
		if request.Type == "cancel" && request.ReplyTo != "" {
			requestsMu.Lock()
			cancel := requests[request.ReplyTo]
			delete(requests, request.ReplyTo)
			requestsMu.Unlock()
			if cancel != nil {
				cancel()
			}
			continue
		}
		if request.Type != "request" || request.ID == "" {
			continue
		}
		requestCtx, cancel := context.WithCancel(sessionCtx)
		requestsMu.Lock()
		requests[request.ID] = cancel
		requestsMu.Unlock()
		go func(request sdk.BrowserBridgeEnvelope) {
			defer func() { requestsMu.Lock(); delete(requests, request.ID); requestsMu.Unlock(); cancel() }()
			var result any
			var callErr error
			switch request.Operation {
			case "browser.action":
				var input sdk.BrowserActionRequest
				callErr = json.Unmarshal(request.Payload, &input)
				if callErr == nil {
					result, callErr = m.ExecuteAction(requestCtx, input)
				}
			case "browser.targets":
				var input sdk.BrowserTarget
				callErr = json.Unmarshal(request.Payload, &input)
				if callErr == nil {
					result, callErr = m.Targets(requestCtx, input.AgentID)
				}
			case "browser.network.start":
				var input sdk.BrowserNetworkStartRequest
				callErr = json.Unmarshal(request.Payload, &input)
				if callErr == nil {
					result, callErr = m.startNetworkCapture(requestCtx, pluginID, input)
				}
			case "browser.network.stop":
				var input map[string]string
				callErr = json.Unmarshal(request.Payload, &input)
				if callErr == nil {
					result, callErr = m.StopNetworkCapture(requestCtx, input["id"])
				}
			default:
				callErr = fmt.Errorf("unsupported browser bridge operation %q", request.Operation)
			}
			response := sdk.BrowserBridgeEnvelope{Type: "response", ReplyTo: request.ID, Operation: request.Operation}
			if callErr != nil {
				response.Error = callErr.Error()
			} else {
				response.Payload, _ = json.Marshal(result)
			}
			select {
			case send <- response:
			case <-sessionCtx.Done():
			}
		}(request)
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

func (agent *agentConnection) supportsCapability(required string) bool {
	agent.infoMu.RLock()
	capabilities := make(map[string]struct{}, len(agent.info.Capabilities))
	for _, capability := range agent.info.Capabilities {
		capabilities[capability] = struct{}{}
	}
	agent.infoMu.RUnlock()
	_, ok := capabilities[required]
	return ok
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
		case "event":
			agent.manager.handleEvent(agent, message)
		}
	}
}

func (m *Manager) handleEvent(agent *agentConnection, message wireMessage) {
	switch message.Command {
	case "browser.network":
		var event sdk.BrowserNetworkResult
		if json.Unmarshal(message.Payload, &event) != nil || event.SessionID == "" {
			return
		}
		m.capturesMu.Lock()
		capture := m.captures[event.SessionID]
		if capture != nil && len(capture.Events) < 2000 {
			capture.Events = append(capture.Events, event)
			capture.Session.Count = len(capture.Events)
		}
		m.capturesMu.Unlock()
		if capture != nil {
			m.publish(StreamEvent{Type: message.Command, AgentID: agent.info.ID, SessionID: event.SessionID, Payload: event})
		}
	case "browser.targets.changed":
		m.publish(StreamEvent{Type: message.Command, AgentID: agent.info.ID, Payload: json.RawMessage(message.Payload)})
	case "browser.network.status":
		var session sdk.BrowserNetworkSession
		if json.Unmarshal(message.Payload, &session) != nil || session.ID == "" {
			return
		}
		owner := ""
		if session.Status == "stopped" {
			m.capturesMu.Lock()
			capture := m.captures[session.ID]
			if capture != nil && capture.Session.Target.AgentID == agent.info.ID {
				owner = capture.Owner
				delete(m.captures, session.ID)
			}
			m.capturesMu.Unlock()
			if capture == nil {
				return
			}
		}
		m.publish(StreamEvent{Type: message.Command, AgentID: agent.info.ID, SessionID: session.ID, Payload: json.RawMessage(message.Payload), Owner: owner})
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
