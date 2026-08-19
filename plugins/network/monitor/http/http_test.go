package httpmonitor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/hanxuanyu/meerkit/sdk"
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
	if observation.Result["status_code"] != "200" {
		t.Fatalf("unexpected status: %#v", observation.Result["status_code"])
	}
	if observation.SchemaVersion != resultSchemaVersion {
		t.Fatalf("schema version = %q, want %q", observation.SchemaVersion, resultSchemaVersion)
	}
	body, ok := observation.Result["body_json"].(map[string]any)
	if !ok || body["data"].(map[string]any)["version"] != float64(2) {
		t.Fatalf("unexpected JSON result: %#v", observation.Result["body_json"])
	}
	if _, err := json.Marshal(observation); err != nil {
		t.Fatalf("observation result sets must be serializable: %v", err)
	}
}

func TestExecuteLogsSafeRequestAndResponseSummary(t *testing.T) {
	var logs bytes.Buffer
	module := &Module{Logger: slog.New(slog.NewTextHandler(&logs, nil)), Client: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/plain"}}, Body: io.NopCloser(strings.NewReader("private response")), Request: request}, nil
	})}}
	config, _ := json.Marshal(map[string]any{"url": "http://test.local/health?token=query-secret", "headers": map[string]any{"Authorization": "Bearer header-secret"}})
	if _, err := module.Execute(context.Background(), config); err != nil {
		t.Fatal(err)
	}
	output := logs.String()
	for _, expected := range []string{"http request sending", "target=http://test.local/health", "http response processed", "status_code=200", "body_size=16"} {
		if !strings.Contains(output, expected) {
			t.Fatalf("missing %q in plugin logs:\n%s", expected, output)
		}
	}
	for _, secret := range []string{"query-secret", "header-secret", "private response"} {
		if strings.Contains(output, secret) {
			t.Fatalf("plugin logs leaked %q:\n%s", secret, output)
		}
	}
}

func TestExecuteRedactsQueryFromRequestErrors(t *testing.T) {
	var logs bytes.Buffer
	module := &Module{Logger: slog.New(slog.NewTextHandler(&logs, nil)), Client: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return nil, &url.Error{Op: "Get", URL: request.URL.String(), Err: errors.New("connection refused")}
	})}}
	config, _ := json.Marshal(map[string]any{"url": "http://test.local/health?token=query-secret"})
	_, err := module.Execute(context.Background(), config)
	if err == nil {
		t.Fatal("expected request failure")
	}
	if strings.Contains(logs.String(), "query-secret") || strings.Contains(err.Error(), "query-secret") {
		t.Fatalf("request error leaked query: error=%v logs=%s", err, logs.String())
	}
}

func TestExecuteSummaryIncludesHTTPResultDetails(t *testing.T) {
	module := &Module{Client: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusCreated, Status: "201 Created", Header: http.Header{
			"Content-Type": []string{"application/json"},
			"X-Request-ID": []string{"request-123"},
		}, Body: io.NopCloser(strings.NewReader(`{"ok":true}`)), Request: request}, nil
	})}}
	config, _ := json.Marshal(map[string]any{"url": "http://test.local/health", "method": "POST", "response_mode": "json", "normalize": "json"})

	observation, err := module.Execute(context.Background(), config)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	for _, expected := range []string{
		"HTTP 请求：POST http://test.local/health",
		"响应状态：201 Created",
		"响应体：JSON，application/json，11 字节，未截断",
		"内容哈希：",
	} {
		if !strings.Contains(observation.Summary, expected) {
			t.Fatalf("summary does not contain %q:\n%s", expected, observation.Summary)
		}
	}
	if strings.Contains(observation.Summary, "响应正文") || strings.Contains(observation.Summary, "响应头") || strings.Contains(observation.Summary, `{"ok":true}`) {
		t.Fatalf("summary should omit long response fields:\n%s", observation.Summary)
	}
}

func TestResponseSummaryOmitsResponseBodyAndHeaders(t *testing.T) {
	request, err := http.NewRequest(http.MethodGet, "http://test.local/health", nil)
	if err != nil {
		t.Fatal(err)
	}
	summary := responseSummary(request, &http.Response{StatusCode: http.StatusOK, Header: http.Header{"X-Request-ID": []string{"request-123"}}}, map[string]any{
		"duration_ms": 1, "body_size": 2048, "body_text": "long response body", "body_hash": "hash", "truncated": true,
	})
	if strings.Contains(summary, "long response body") || strings.Contains(summary, "响应头") || strings.Contains(summary, "request-123") {
		t.Fatalf("summary should omit response body and headers:\n%s", summary)
	}
	for _, expected := range []string{"响应体：2048 字节，已截断", "内容哈希：hash"} {
		if !strings.Contains(summary, expected) {
			t.Fatalf("summary does not contain %q:\n%s", expected, summary)
		}
	}
}

func TestExecuteSupportsHTTPMethods(t *testing.T) {
	for _, method := range []string{"GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "OPTIONS", "TRACE", "CONNECT"} {
		t.Run(method, func(t *testing.T) {
			var got string
			module := captureModule(func(request *http.Request) {
				got = request.Method
			})
			config, _ := json.Marshal(map[string]any{"url": "http://test.local/resource", "method": method})
			if _, err := module.Execute(context.Background(), config); err != nil {
				t.Fatalf("execute failed: %v", err)
			}
			if got != method {
				t.Fatalf("method = %q, want %q", got, method)
			}
		})
	}
}

func TestExecuteBuildsQueryAndHeaders(t *testing.T) {
	var captured *http.Request
	module := captureModule(func(request *http.Request) {
		captured = request
	})
	config, _ := json.Marshal(map[string]any{
		"url":     "http://test.local/resource?existing=value",
		"query":   map[string]any{"page": "2", "tag": []any{"one", "two"}},
		"headers": map[string]any{"X-Request-ID": "abc", "Accept": "application/json", "Authorization": "Bearer token", "Cookie": "session=xyz; theme=dark"},
	})
	if _, err := module.Execute(context.Background(), config); err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if captured.URL.Query().Get("existing") != "value" || captured.URL.Query().Get("page") != "2" {
		t.Fatalf("unexpected query: %s", captured.URL.RawQuery)
	}
	if values := captured.URL.Query()["tag"]; len(values) != 2 || values[0] != "one" || values[1] != "two" {
		t.Fatalf("unexpected repeated query: %#v", values)
	}
	if captured.Header.Get("X-Request-ID") != "abc" || captured.Header.Get("Accept") != "application/json" || captured.Header.Get("Authorization") != "Bearer token" {
		t.Fatalf("unexpected headers: %#v", captured.Header)
	}
	cookies := captured.Cookies()
	if len(cookies) != 2 || cookies[0].Name != "session" || cookies[0].Value != "xyz" || cookies[1].Name != "theme" || cookies[1].Value != "dark" {
		t.Fatalf("unexpected Cookie header: %#v", cookies)
	}
}

func TestExecuteBuildsURLEncodedForm(t *testing.T) {
	var captured *http.Request
	module := captureModule(func(request *http.Request) {
		captured = request
	})
	config, _ := json.Marshal(map[string]any{
		"url":       "http://test.local/submit",
		"method":    "POST",
		"body_mode": "form_urlencoded",
		"form_fields": map[string]any{
			"name": "Meerkit",
			"kind": "monitor",
		},
	})
	if _, err := module.Execute(context.Background(), config); err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if captured.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
		t.Fatalf("content type = %q", captured.Header.Get("Content-Type"))
	}
	body, _ := io.ReadAll(captured.Body)
	if string(body) != "kind=monitor&name=Meerkit" {
		t.Fatalf("body = %q", body)
	}
}

func TestExecuteBuildsMultipartForm(t *testing.T) {
	var captured *http.Request
	module := captureModule(func(request *http.Request) {
		captured = request
	})
	config, _ := json.Marshal(map[string]any{
		"url":       "http://test.local/upload",
		"method":    "POST",
		"body_mode": "multipart_form",
		"form_fields": map[string]any{
			"name": "Meerkit",
		},
	})
	if _, err := module.Execute(context.Background(), config); err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	mediaType, params, err := mime.ParseMediaType(captured.Header.Get("Content-Type"))
	if err != nil || mediaType != "multipart/form-data" {
		t.Fatalf("content type = %q: %v", captured.Header.Get("Content-Type"), err)
	}
	reader := multipartReader(t, captured, params["boundary"])
	part, err := reader.NextPart()
	if err != nil {
		t.Fatalf("read multipart part: %v", err)
	}
	if part.FormName() != "name" {
		t.Fatalf("form name = %q", part.FormName())
	}
	value, _ := io.ReadAll(part)
	if string(value) != "Meerkit" {
		t.Fatalf("form value = %q", value)
	}
}

func TestExecuteBuildsRawBodies(t *testing.T) {
	tests := []struct {
		name        string
		mode        string
		field       string
		value       string
		contentType string
	}{
		{name: "json", mode: "raw_json", field: "json_body", value: `{"healthy":true}`, contentType: "application/json"},
		{name: "text", mode: "raw_text", field: "raw_body", value: "PING\r\n", contentType: "text/plain; charset=utf-8"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var captured *http.Request
			module := captureModule(func(request *http.Request) {
				captured = request
			})
			config := map[string]any{"url": "http://test.local/raw", "method": "POST", "body_mode": test.mode, test.field: test.value}
			raw, _ := json.Marshal(config)
			if _, err := module.Execute(context.Background(), raw); err != nil {
				t.Fatalf("execute failed: %v", err)
			}
			if captured.Header.Get("Content-Type") != test.contentType {
				t.Fatalf("content type = %q", captured.Header.Get("Content-Type"))
			}
			body, _ := io.ReadAll(captured.Body)
			if string(body) != test.value {
				t.Fatalf("body = %q", body)
			}
		})
	}
}

func TestValidateConfigRejectsInvalidBodyConfiguration(t *testing.T) {
	tests := []struct {
		name   string
		config map[string]any
	}{
		{name: "method", config: map[string]any{"url": "http://test.local", "method": "INVALID"}},
		{name: "body on GET", config: map[string]any{"url": "http://test.local", "method": "GET", "body_mode": "raw_json", "json_body": `{"value":1}`}},
		{name: "body mode", config: map[string]any{"url": "http://test.local", "body_mode": "xml"}},
		{name: "invalid json", config: map[string]any{"url": "http://test.local", "body_mode": "raw_json", "json_body": "{"}},
		{name: "invalid proxy", config: map[string]any{"url": "http://test.local", "proxy_url": "unix:///tmp/proxy"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			raw, _ := json.Marshal(test.config)
			if err := (&Module{}).ValidateConfig(raw); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestDescriptorDeclaresMethodDependentBodyParameters(t *testing.T) {
	descriptor := (&Module{}).Descriptor()
	if descriptor.ListSummary == nil || len(descriptor.ListSummary.Fields) != 1 || descriptor.ListSummary.Fields[0] != "url" {
		t.Fatalf("HTTP list summary should use url: %#v", descriptor.ListSummary)
	}
	var bodyMode *sdk.ParameterDescriptor
	for index := range descriptor.Parameters {
		if descriptor.Parameters[index].Key == "body_mode" {
			bodyMode = &descriptor.Parameters[index]
			break
		}
	}
	if bodyMode == nil {
		t.Fatal("body_mode parameter is missing")
	}
	if len(bodyMode.VisibleWhen) != 1 || bodyMode.VisibleWhen[0].Operator != "in" {
		t.Fatalf("body_mode should depend on method: %#v", bodyMode.VisibleWhen)
	}
	if len(bodyMode.OptionsWhen) != 1 || len(bodyMode.OptionsWhen[0].Options) != len(supportedBodyModes) {
		t.Fatalf("body_mode options should be dynamic: %#v", bodyMode.OptionsWhen)
	}
}

func TestDescriptorDeclaresStatusCodeAsString(t *testing.T) {
	descriptor := (&Module{}).Descriptor()
	if descriptor.ResultSchemaVersion != resultSchemaVersion {
		t.Fatalf("result schema version = %q, want %q", descriptor.ResultSchemaVersion, resultSchemaVersion)
	}
	properties, ok := descriptor.ResultSchema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("result schema properties are missing: %#v", descriptor.ResultSchema)
	}
	statusSchema, ok := properties["status_code"].(map[string]any)
	if !ok || statusSchema["type"] != "string" {
		t.Fatalf("status_code schema = %#v, want string", properties["status_code"])
	}
	for _, field := range descriptor.Fields {
		if field.Name == "status_code" && field.Type != "string" {
			t.Fatalf("legacy status_code field type = %q, want string", field.Type)
		}
	}
	if len(descriptor.ResultSets) != 1 {
		t.Fatalf("result sets = %#v, want one response set", descriptor.ResultSets)
	}
	for _, field := range descriptor.ResultSets[0].Fields {
		if field.Name == "status_code" {
			if field.Type != "string" {
				t.Fatalf("status_code result field type = %q, want string", field.Type)
			}
			for _, operator := range field.Operators {
				if operator == "gt" || operator == "gte" || operator == "lt" || operator == "lte" {
					t.Fatalf("status_code should not expose numeric operator %q", operator)
				}
			}
			return
		}
	}
	t.Fatal("status_code result field is missing")
}

func captureModule(capture func(*http.Request)) *Module {
	return &Module{Client: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		capture(request)
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/plain"}}, Body: io.NopCloser(strings.NewReader("ok")), Request: request}, nil
	})}}
}

func multipartReader(t *testing.T, request *http.Request, boundary string) *multipart.Reader {
	t.Helper()
	return multipart.NewReader(request.Body, boundary)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return fn(request) }
