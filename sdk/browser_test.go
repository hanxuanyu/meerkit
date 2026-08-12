package sdk

import (
	"context"
	"encoding/json"
	"io"
	"testing"
	"time"

	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type browserStreamReceive struct {
	value *wrapperspb.BytesValue
	err   error
}

type browserTestStream struct {
	ctx     context.Context
	cancel  context.CancelFunc
	receive chan browserStreamReceive
	sent    chan BrowserBridgeEnvelope
}

func newBrowserTestStream() *browserTestStream {
	ctx, cancel := context.WithCancel(context.Background())
	return &browserTestStream{ctx: ctx, cancel: cancel, receive: make(chan browserStreamReceive, 8), sent: make(chan BrowserBridgeEnvelope, 8)}
}

func (s *browserTestStream) Send(value *wrapperspb.BytesValue) error {
	var envelope BrowserBridgeEnvelope
	if err := json.Unmarshal(value.Value, &envelope); err != nil {
		return err
	}
	select {
	case s.sent <- envelope:
		return nil
	case <-s.ctx.Done():
		return s.ctx.Err()
	}
}

func (s *browserTestStream) Recv() (*wrapperspb.BytesValue, error) {
	select {
	case value := <-s.receive:
		return value.value, value.err
	case <-s.ctx.Done():
		return nil, s.ctx.Err()
	}
}

func (s *browserTestStream) SetHeader(metadata.MD) error  { return nil }
func (s *browserTestStream) SendHeader(metadata.MD) error { return nil }
func (s *browserTestStream) SetTrailer(metadata.MD)       {}
func (s *browserTestStream) Context() context.Context     { return s.ctx }
func (s *browserTestStream) SendMsg(any) error            { return nil }
func (s *browserTestStream) RecvMsg(any) error            { return nil }

func (s *browserTestStream) push(envelope BrowserBridgeEnvelope) {
	raw, _ := json.Marshal(envelope)
	s.receive <- browserStreamReceive{value: wrapperspb.Bytes(raw)}
}

func waitBrowserEnvelope(t *testing.T, channel <-chan BrowserBridgeEnvelope) BrowserBridgeEnvelope {
	t.Helper()
	select {
	case envelope := <-channel:
		return envelope
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for browser bridge envelope")
		return BrowserBridgeEnvelope{}
	}
}

func startBridgeSession(t *testing.T) (*sessionBrowserClient, *browserTestStream) {
	t.Helper()
	client := newSessionBrowserClient()
	stream := newBrowserTestStream()
	done := make(chan error, 1)
	go func() { done <- client.attach(stream) }()
	if ready := waitBrowserEnvelope(t, stream.sent); ready.Type != "ready" {
		t.Fatalf("first envelope = %#v, want ready", ready)
	}
	t.Cleanup(func() {
		stream.cancel()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("browser bridge did not stop")
		}
	})
	return client, stream
}

func TestBrowserBridgeMapsRequestResponse(t *testing.T) {
	client, stream := startBridgeSession(t)
	result := make(chan BrowserTargets, 1)
	errors := make(chan error, 1)
	go func() {
		value, err := client.ListTargets(t.Context(), "agent-1")
		result <- value
		errors <- err
	}()
	request := waitBrowserEnvelope(t, stream.sent)
	if request.Type != "request" || request.Operation != "browser.targets" || request.ID == "" {
		t.Fatalf("unexpected request: %#v", request)
	}
	payload, _ := json.Marshal(BrowserTargets{AgentID: "agent-1", Windows: []BrowserWindow{{ID: 7}}})
	stream.push(BrowserBridgeEnvelope{Type: "response", ReplyTo: request.ID, Payload: payload})
	if err := <-errors; err != nil {
		t.Fatal(err)
	}
	if value := <-result; value.AgentID != "agent-1" || len(value.Windows) != 1 || value.Windows[0].ID != 7 {
		t.Fatalf("unexpected targets: %#v", value)
	}
}

func TestBrowserBridgeSendsCancel(t *testing.T) {
	client, stream := startBridgeSession(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := client.ListTargets(ctx, "agent-1")
		done <- err
	}()
	request := waitBrowserEnvelope(t, stream.sent)
	cancel()
	if err := <-done; err != context.Canceled {
		t.Fatalf("request error = %v, want context canceled", err)
	}
	cancelEnvelope := waitBrowserEnvelope(t, stream.sent)
	if cancelEnvelope.Type != "cancel" || cancelEnvelope.ReplyTo != request.ID {
		t.Fatalf("unexpected cancel: %#v", cancelEnvelope)
	}
}

func TestBrowserBridgeBuffersEarlyNetworkEvent(t *testing.T) {
	client, stream := startBridgeSession(t)
	captures := make(chan BrowserCapture, 1)
	errors := make(chan error, 1)
	go func() {
		capture, err := client.StartNetworkCapture(t.Context(), BrowserNetworkStartRequest{Target: BrowserTarget{TabID: 3}})
		captures <- capture
		errors <- err
	}()
	request := waitBrowserEnvelope(t, stream.sent)
	eventPayload, _ := json.Marshal(BrowserNetworkResult{SessionID: "capture-1", URL: "https://example.com/data"})
	stream.push(BrowserBridgeEnvelope{Type: "event", Operation: "browser.network", Payload: eventPayload})
	sessionPayload, _ := json.Marshal(BrowserNetworkSession{ID: "capture-1", Target: BrowserTarget{TabID: 3}, Status: "running"})
	stream.push(BrowserBridgeEnvelope{Type: "response", ReplyTo: request.ID, Payload: sessionPayload})
	if err := <-errors; err != nil {
		t.Fatal(err)
	}
	capture := <-captures
	select {
	case event := <-capture.Events():
		if event.URL != "https://example.com/data" {
			t.Fatalf("unexpected event: %#v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("early network event was not delivered")
	}
	stream.receive <- browserStreamReceive{err: io.EOF}
}
