package webhook

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"meerkit/internal/core"
)

func TestSendSupportsGetAndPostBodies(t *testing.T) {
	type captured struct {
		method      string
		contentType string
		query       url.Values
		headers     http.Header
		body        string
	}
	requests := make(chan captured, 4)
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(request.Body)
		requests <- captured{method: request.Method, contentType: request.Header.Get("Content-Type"), query: request.URL.Query(), headers: request.Header, body: string(body)}
		return &http.Response{StatusCode: http.StatusAccepted, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("")), Request: request}, nil
	})

	notifier := &Notifier{Client: &http.Client{Transport: transport}}
	event := core.NotificationEvent{EventType: "triggered", MonitorName: "API", ModuleType: "http", TriggeredAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Summary: "changed", CurrentResult: map[string]any{"response": map[string]any{"status_code": 503}}}
	webhookURL := "http://webhook.test/notify"

	tests := []struct {
		name        string
		config      map[string]any
		wantMethod  string
		wantBody    string
		wantContain string
	}{
		{name: "get", config: map[string]any{"url": webhookURL, "method": "GET", "query": map[string]any{"token": "abc"}, "headers": map[string]any{"Authorization": "Bearer test"}}, wantMethod: "GET", wantContain: "event_type=triggered"},
		{name: "event json", config: map[string]any{"url": webhookURL, "method": "POST", "body_mode": "event_json"}, wantMethod: "POST", wantContain: "\"event_type\":\"triggered\""},
		{name: "form", config: map[string]any{"url": webhookURL, "method": "POST", "body_mode": "form_urlencoded", "form_fields": map[string]any{"kind": "alert"}}, wantMethod: "POST", wantContain: "kind=alert"},
		{name: "raw json", config: map[string]any{"url": webhookURL, "method": "POST", "body_mode": "raw_json", "json_body": "{\"hello\":\"world\"}"}, wantMethod: "POST", wantBody: "{\"hello\":\"world\"}"},
		{name: "templated json", config: map[string]any{"url": webhookURL, "method": "POST", "body_mode": "raw_json", "json_body": "{\"status\":{{result.response.status_code}}}"}, wantMethod: "POST", wantBody: "{\"status\":503}"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			raw, err := json.Marshal(test.config)
			if err != nil {
				t.Fatal(err)
			}
			if err := notifier.Send(context.Background(), raw, event); err != nil {
				t.Fatal(err)
			}
			request := <-requests
			if request.method != test.wantMethod {
				t.Fatalf("method = %q, want %q", request.method, test.wantMethod)
			}
			if test.wantBody != "" && request.body != test.wantBody {
				t.Fatalf("body = %q, want %q", request.body, test.wantBody)
			}
			if test.wantContain != "" && !strings.Contains(request.body+request.query.Encode(), test.wantContain) {
				t.Fatalf("request does not contain %q: body=%q query=%q", test.wantContain, request.body, request.query.Encode())
			}
		})
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestValidateConfigRejectsUnsupportedWebhookConfig(t *testing.T) {
	tests := []json.RawMessage{
		json.RawMessage("{\"url\":\"https://example.com\",\"method\":\"PUT\"}"),
		json.RawMessage("{\"url\":\"https://example.com\",\"method\":\"POST\",\"body_mode\":\"raw_json\",\"json_body\":\"{\"}"),
		json.RawMessage("{\"url\":\"https://example.com\",\"method\":\"POST\",\"body_mode\":\"form_urlencoded\",\"form_fields\":[]}"),
	}
	for _, raw := range tests {
		if err := (&Notifier{}).ValidateConfig(raw); err == nil {
			t.Fatalf("ValidateConfig(%s) unexpectedly succeeded", raw)
		}
	}
}
