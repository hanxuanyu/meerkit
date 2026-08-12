package sdk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

const BrowserProtocolVersion = 1
const browserSendQueueSize = 256

type BrowserTarget struct {
	AgentID  string `json:"agent_id,omitempty"`
	WindowID int    `json:"window_id,omitempty"`
	TabID    int    `json:"tab_id,omitempty"`
}

type BrowserAction struct {
	ID     string         `json:"id,omitempty"`
	Type   string         `json:"type"`
	Params map[string]any `json:"params,omitempty"`
}

type BrowserActionRequest struct {
	Target    BrowserTarget `json:"target,omitempty"`
	TimeoutMS int           `json:"timeout_ms,omitempty"`
	Action    BrowserAction `json:"action"`
}

type BrowserActionResult struct {
	ID       string         `json:"id,omitempty"`
	Type     string         `json:"type"`
	Success  bool           `json:"success"`
	Target   BrowserTarget  `json:"target,omitempty"`
	Duration int64          `json:"duration_ms,omitempty"`
	Data     map[string]any `json:"data,omitempty"`
	Error    string         `json:"error,omitempty"`
}

type BrowserTargets struct {
	AgentID string          `json:"agent_id,omitempty"`
	Windows []BrowserWindow `json:"windows,omitempty"`
}

type BrowserWindow struct {
	ID      int          `json:"id"`
	Focused bool         `json:"focused,omitempty"`
	Type    string       `json:"type,omitempty"`
	Tabs    []BrowserTab `json:"tabs,omitempty"`
}

type BrowserTab struct {
	ID         int    `json:"id"`
	WindowID   int    `json:"window_id"`
	Index      int    `json:"index,omitempty"`
	Active     bool   `json:"active,omitempty"`
	Status     string `json:"status,omitempty"`
	Title      string `json:"title,omitempty"`
	URL        string `json:"url,omitempty"`
	GroupID    int    `json:"group_id,omitempty"`
	GroupTitle string `json:"group_title,omitempty"`
}

type BrowserNetworkCaptureRule struct {
	ID           string `json:"id,omitempty"`
	URLContains  string `json:"url_contains,omitempty"`
	ResourceType string `json:"resource_type,omitempty"`
	MaxBodyBytes int    `json:"max_body_bytes,omitempty"`
}

type BrowserNetworkStartRequest struct {
	Target BrowserTarget               `json:"target"`
	Rules  []BrowserNetworkCaptureRule `json:"rules,omitempty"`
}

type BrowserNetworkSession struct {
	ID        string        `json:"id"`
	Target    BrowserTarget `json:"target"`
	Status    string        `json:"status,omitempty"`
	StartedAt string        `json:"started_at,omitempty"`
	Count     int           `json:"count,omitempty"`
	Error     string        `json:"error,omitempty"`
}

type BrowserNetworkResult struct {
	SessionID            string             `json:"session_id,omitempty"`
	CaptureID            string             `json:"capture_id,omitempty"`
	URL                  string             `json:"url,omitempty"`
	Method               string             `json:"method,omitempty"`
	Status               int                `json:"status,omitempty"`
	StatusText           string             `json:"status_text,omitempty"`
	ResourceType         string             `json:"resource_type,omitempty"`
	MimeType             string             `json:"mime_type,omitempty"`
	Protocol             string             `json:"protocol,omitempty"`
	RemoteIPAddress      string             `json:"remote_ip_address,omitempty"`
	RemotePort           int                `json:"remote_port,omitempty"`
	InitiatorType        string             `json:"initiator_type,omitempty"`
	Headers              map[string]string  `json:"headers,omitempty"`
	RequestHeaders       map[string]string  `json:"request_headers,omitempty"`
	RequestBody          string             `json:"request_body,omitempty"`
	RequestBodyTruncated bool               `json:"request_body_truncated,omitempty"`
	Body                 string             `json:"body,omitempty"`
	BodyBase64           bool               `json:"body_base64,omitempty"`
	Truncated            bool               `json:"truncated,omitempty"`
	EncodedDataLength    int64              `json:"encoded_data_length,omitempty"`
	Duration             int64              `json:"duration_ms,omitempty"`
	FromDiskCache        bool               `json:"from_disk_cache,omitempty"`
	FromServiceWorker    bool               `json:"from_service_worker,omitempty"`
	Timing               map[string]float64 `json:"timing,omitempty"`
	Error                string             `json:"error,omitempty"`
}

type BrowserNetworkStopResult struct {
	Session BrowserNetworkSession  `json:"session"`
	Events  []BrowserNetworkResult `json:"events,omitempty"`
}

type BrowserCapture interface {
	Session() BrowserNetworkSession
	Events() <-chan BrowserNetworkResult
	Err() error
	Stop(context.Context) (BrowserNetworkStopResult, error)
}

type BrowserClient interface {
	ListTargets(context.Context, string) (BrowserTargets, error)
	ExecuteAction(context.Context, BrowserActionRequest) (BrowserActionResult, error)
	StartNetworkCapture(context.Context, BrowserNetworkStartRequest) (BrowserCapture, error)
}

type BrowserBridgeEnvelope struct {
	Type      string          `json:"type"`
	ID        string          `json:"id,omitempty"`
	ReplyTo   string          `json:"reply_to,omitempty"`
	Operation string          `json:"operation,omitempty"`
	Payload   json.RawMessage `json:"payload,omitempty"`
	Error     string          `json:"error,omitempty"`
}

type BrowserBridgeServer interface {
	Session(BrowserBridgeSessionServer) error
}
type BrowserBridgeSessionServer interface {
	Send(*wrapperspb.BytesValue) error
	Recv() (*wrapperspb.BytesValue, error)
	grpc.ServerStream
}
type BrowserBridgeSessionClient interface {
	Send(*wrapperspb.BytesValue) error
	Recv() (*wrapperspb.BytesValue, error)
	grpc.ClientStream
}

var browserBridgeDesc = grpc.ServiceDesc{
	ServiceName: "meerkit.sdk.BrowserBridge",
	HandlerType: (*BrowserBridgeServer)(nil),
	Streams: []grpc.StreamDesc{{StreamName: "Session", ServerStreams: true, ClientStreams: true, Handler: func(server any, stream grpc.ServerStream) error {
		return server.(BrowserBridgeServer).Session(&browserBridgeServerStream{ServerStream: stream})
	}}},
}

type browserBridgeServerStream struct{ grpc.ServerStream }

func (s *browserBridgeServerStream) Send(value *wrapperspb.BytesValue) error { return s.SendMsg(value) }
func (s *browserBridgeServerStream) Recv() (*wrapperspb.BytesValue, error) {
	value := new(wrapperspb.BytesValue)
	if err := s.RecvMsg(value); err != nil {
		return nil, err
	}
	return value, nil
}

type browserBridgeClientStream struct{ grpc.ClientStream }

func (s *browserBridgeClientStream) Send(value *wrapperspb.BytesValue) error { return s.SendMsg(value) }
func (s *browserBridgeClientStream) Recv() (*wrapperspb.BytesValue, error) {
	value := new(wrapperspb.BytesValue)
	if err := s.RecvMsg(value); err != nil {
		return nil, err
	}
	return value, nil
}

func RegisterBrowserBridgeServer(server grpc.ServiceRegistrar, implementation BrowserBridgeServer) {
	server.RegisterService(&browserBridgeDesc, implementation)
}

type BrowserBridgeClient struct{ connection grpc.ClientConnInterface }

func NewBrowserBridgeClient(connection grpc.ClientConnInterface) *BrowserBridgeClient {
	return &BrowserBridgeClient{connection: connection}
}
func (c *BrowserBridgeClient) Session(ctx context.Context, options ...grpc.CallOption) (BrowserBridgeSessionClient, error) {
	stream, err := c.connection.NewStream(ctx, &browserBridgeDesc.Streams[0], "/meerkit.sdk.BrowserBridge/Session", options...)
	if err != nil {
		return nil, err
	}
	return &browserBridgeClientStream{ClientStream: stream}, nil
}

type PluginRuntime struct{ browser *sessionBrowserClient }

func NewPluginRuntime() *PluginRuntime                      { return &PluginRuntime{browser: newSessionBrowserClient()} }
func (r *PluginRuntime) Browser() BrowserClient             { return r.browser }
func (r *PluginRuntime) Serve(provider Provider)            { serveWithRuntime(provider, r) }
func (r *PluginRuntime) browserServer() BrowserBridgeServer { return &pluginBrowserBridge{runtime: r} }

type pendingBrowserResponse struct{ envelope BrowserBridgeEnvelope }
type sessionBrowserClient struct {
	mu           sync.Mutex
	stream       BrowserBridgeSessionServer
	send         chan BrowserBridgeEnvelope
	pending      map[string]chan pendingBrowserResponse
	captures     map[string]*sessionCapture
	orphanEvents map[string][]BrowserNetworkResult
	sequence     atomic.Uint64
	disconnected chan struct{}
}

func newSessionBrowserClient() *sessionBrowserClient {
	return &sessionBrowserClient{pending: make(map[string]chan pendingBrowserResponse), captures: make(map[string]*sessionCapture), orphanEvents: make(map[string][]BrowserNetworkResult)}
}

func (c *sessionBrowserClient) attach(stream BrowserBridgeSessionServer) error {
	c.mu.Lock()
	if c.stream != nil {
		c.mu.Unlock()
		return errors.New("browser bridge session is already connected")
	}
	c.stream, c.send, c.disconnected = stream, make(chan BrowserBridgeEnvelope, browserSendQueueSize), make(chan struct{})
	c.mu.Unlock()
	writerDone := make(chan error, 1)
	go c.writeLoop(stream, writerDone)
	if err := c.enqueue(BrowserBridgeEnvelope{Type: "ready"}); err != nil {
		c.disconnect(stream)
		return err
	}
	readDone := make(chan error, 1)
	go func() { readDone <- c.readLoop(stream) }()
	var readErr error
	select {
	case readErr = <-readDone:
	case readErr = <-writerDone:
	case <-stream.Context().Done():
		readErr = stream.Context().Err()
	}
	c.disconnect(stream)
	return readErr
}

func (c *sessionBrowserClient) writeLoop(stream BrowserBridgeSessionServer, done chan<- error) {
	for {
		select {
		case <-stream.Context().Done():
			done <- stream.Context().Err()
			return
		case envelope, ok := <-c.send:
			if !ok {
				done <- io.EOF
				return
			}
			data, err := json.Marshal(envelope)
			if err == nil {
				err = stream.Send(wrapperspb.Bytes(data))
			}
			if err != nil {
				done <- err
				return
			}
		}
	}
}

func (c *sessionBrowserClient) readLoop(stream BrowserBridgeSessionServer) error {
	for {
		value, err := stream.Recv()
		if err != nil {
			return err
		}
		var envelope BrowserBridgeEnvelope
		if json.Unmarshal(value.Value, &envelope) != nil {
			continue
		}
		switch envelope.Type {
		case "response":
			c.mu.Lock()
			response := c.pending[envelope.ReplyTo]
			delete(c.pending, envelope.ReplyTo)
			c.mu.Unlock()
			if response != nil {
				response <- pendingBrowserResponse{envelope: envelope}
			}
		case "event":
			switch envelope.Operation {
			case "browser.network":
				var event BrowserNetworkResult
				if json.Unmarshal(envelope.Payload, &event) != nil {
					continue
				}
				c.mu.Lock()
				capture := c.captures[event.SessionID]
				if capture == nil && len(c.orphanEvents[event.SessionID]) < 128 {
					c.orphanEvents[event.SessionID] = append(c.orphanEvents[event.SessionID], event)
				}
				c.mu.Unlock()
				if capture != nil {
					capture.publish(event)
				}
			case "browser.network.status":
				var status struct {
					ID     string `json:"id"`
					Status string `json:"status"`
					Error  string `json:"error"`
				}
				if json.Unmarshal(envelope.Payload, &status) != nil || status.ID == "" || status.Status != "stopped" {
					continue
				}
				c.mu.Lock()
				capture := c.captures[status.ID]
				delete(c.captures, status.ID)
				c.mu.Unlock()
				if capture != nil {
					capture.closeWithError(status.Error)
				}
			}
		}
	}
}

func (c *sessionBrowserClient) disconnect(stream BrowserBridgeSessionServer) {
	c.mu.Lock()
	if c.stream != stream {
		c.mu.Unlock()
		return
	}
	c.stream = nil
	close(c.disconnected)
	for id, response := range c.pending {
		response <- pendingBrowserResponse{envelope: BrowserBridgeEnvelope{Error: "browser bridge disconnected"}}
		delete(c.pending, id)
	}
	for id, capture := range c.captures {
		capture.close()
		delete(c.captures, id)
	}
	clear(c.orphanEvents)
	c.mu.Unlock()
}

func (c *sessionBrowserClient) enqueue(envelope BrowserBridgeEnvelope) error {
	c.mu.Lock()
	send := c.send
	stream := c.stream
	c.mu.Unlock()
	if stream == nil {
		return errors.New("browser bridge is not connected")
	}
	select {
	case send <- envelope:
		return nil
	default:
		return errors.New("browser bridge send queue is full")
	}
}

func (c *sessionBrowserClient) request(ctx context.Context, operation string, input, output any) error {
	payload, err := json.Marshal(input)
	if err != nil {
		return err
	}
	id := fmt.Sprintf("browser-%d", c.sequence.Add(1))
	response := make(chan pendingBrowserResponse, 1)
	c.mu.Lock()
	if c.stream == nil {
		c.mu.Unlock()
		return errors.New("browser bridge is not connected")
	}
	c.pending[id] = response
	disconnected := c.disconnected
	c.mu.Unlock()
	if err := c.enqueue(BrowserBridgeEnvelope{Type: "request", ID: id, Operation: operation, Payload: payload}); err != nil {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return err
	}
	select {
	case <-ctx.Done():
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		_ = c.enqueue(BrowserBridgeEnvelope{Type: "cancel", ReplyTo: id})
		return ctx.Err()
	case <-disconnected:
		return errors.New("browser bridge disconnected")
	case result := <-response:
		if result.envelope.Error != "" {
			return errors.New(result.envelope.Error)
		}
		if output == nil {
			return nil
		}
		return json.Unmarshal(result.envelope.Payload, output)
	}
}

func (c *sessionBrowserClient) ListTargets(ctx context.Context, agentID string) (BrowserTargets, error) {
	var result BrowserTargets
	err := c.request(ctx, "browser.targets", BrowserTarget{AgentID: agentID}, &result)
	return result, err
}
func (c *sessionBrowserClient) ExecuteAction(ctx context.Context, request BrowserActionRequest) (BrowserActionResult, error) {
	var result BrowserActionResult
	err := c.request(ctx, "browser.action", request, &result)
	return result, err
}
func (c *sessionBrowserClient) StartNetworkCapture(ctx context.Context, request BrowserNetworkStartRequest) (BrowserCapture, error) {
	var session BrowserNetworkSession
	if err := c.request(ctx, "browser.network.start", request, &session); err != nil {
		return nil, err
	}
	capture := &sessionCapture{client: c, session: session, events: make(chan BrowserNetworkResult, 128), stopDone: make(chan struct{})}
	c.mu.Lock()
	c.captures[session.ID] = capture
	orphans := c.orphanEvents[session.ID]
	delete(c.orphanEvents, session.ID)
	c.mu.Unlock()
	for _, event := range orphans {
		capture.publish(event)
	}
	return capture, nil
}

type sessionCapture struct {
	client     *sessionBrowserClient
	session    BrowserNetworkSession
	events     chan BrowserNetworkResult
	once       sync.Once
	stopOnce   sync.Once
	stopDone   chan struct{}
	mu         sync.Mutex
	closed     bool
	err        error
	stopResult BrowserNetworkStopResult
	stopErr    error
}

func (c *sessionCapture) Session() BrowserNetworkSession      { return c.session }
func (c *sessionCapture) Events() <-chan BrowserNetworkResult { return c.events }
func (c *sessionCapture) Err() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.err
}
func (c *sessionCapture) publish(event BrowserNetworkResult) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	select {
	case c.events <- event:
		c.mu.Unlock()
	default:
		queueErr := errors.New("browser network capture event queue is full")
		c.err = queueErr
		c.closed = true
		close(c.events)
		c.mu.Unlock()
		c.beginStop(queueErr)
	}
}
func (c *sessionCapture) close() {
	c.closeWithError("")
}
func (c *sessionCapture) closeWithError(message string) {
	if message != "" {
		c.mu.Lock()
		c.err = errors.New(message)
		c.mu.Unlock()
	}
	c.once.Do(func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		if !c.closed {
			c.closed = true
			close(c.events)
		}
	})
}
func (c *sessionCapture) Stop(ctx context.Context) (BrowserNetworkStopResult, error) {
	c.beginStop(nil)
	select {
	case <-ctx.Done():
		return BrowserNetworkStopResult{}, ctx.Err()
	case <-c.stopDone:
		c.mu.Lock()
		defer c.mu.Unlock()
		return c.stopResult, c.stopErr
	}
}

func (c *sessionCapture) beginStop(reason error) {
	if reason != nil {
		c.mu.Lock()
		if c.err == nil {
			c.err = reason
		}
		c.mu.Unlock()
	}
	c.stopOnce.Do(func() {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			var result BrowserNetworkStopResult
			requestErr := c.client.request(ctx, "browser.network.stop", map[string]string{"id": c.session.ID}, &result)
			c.client.mu.Lock()
			delete(c.client.captures, c.session.ID)
			c.client.mu.Unlock()
			c.close()
			c.mu.Lock()
			c.stopResult = result
			if c.err != nil {
				c.stopErr = c.err
			} else {
				c.stopErr = requestErr
			}
			c.mu.Unlock()
			close(c.stopDone)
		}()
	})
}

type pluginBrowserBridge struct{ runtime *PluginRuntime }

func (s *pluginBrowserBridge) Session(stream BrowserBridgeSessionServer) error {
	return s.runtime.browser.attach(stream)
}
