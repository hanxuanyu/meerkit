package httpmonitor

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestExecuteCapturesJSONResponse(t *testing.T) {
	module := &Module{Client: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"data":{"version":2}}`)), Request: request}, nil
	})}}
	config, _ := json.Marshal(map[string]any{"url": "http://test.local/health", "method": "GET", "response_mode": "json", "normalize": "json"})
	observation, err := module.Execute(context.Background(), config)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if observation.Result["status_code"] != 200 {
		t.Fatalf("unexpected status: %#v", observation.Result["status_code"])
	}
	body, ok := observation.Result["body_json"].(map[string]any)
	if !ok || body["data"].(map[string]any)["version"] != float64(2) {
		t.Fatalf("unexpected JSON result: %#v", observation.Result["body_json"])
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return fn(request) }
