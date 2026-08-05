package httpmonitor

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"meerkit/internal/core"
)

type Module struct {
	Client *http.Client
}

func New() *Module { return &Module{} }

func (m *Module) Descriptor() core.ModuleDescriptor {
	return core.ModuleDescriptor{
		Type: "http", Version: "1", Name: "HTTP", Description: "请求 HTTP/HTTPS 接口并观察响应变化。",
		ConfigSchema: map[string]any{"type": "object", "required": []string{"url"}, "properties": map[string]any{
			"url":     map[string]any{"type": "string", "title": "URL", "required": true},
			"method":  map[string]any{"type": "string", "enum": []string{"GET", "HEAD", "POST"}, "default": "GET"},
			"headers": map[string]any{"type": "object", "title": "请求头"}, "body": map[string]any{"type": "string", "title": "请求体"},
			"auth_type": map[string]any{"type": "string", "enum": []string{"none", "basic", "bearer"}, "default": "none"},
			"username":  map[string]any{"type": "string"}, "password": map[string]any{"type": "string", "secret": true}, "token": map[string]any{"type": "string", "secret": true},
			"timeout_seconds": map[string]any{"type": "integer", "default": 30, "minimum": 1, "maximum": 300}, "verify_tls": map[string]any{"type": "boolean", "default": true},
			"response_mode": map[string]any{"type": "string", "enum": []string{"auto", "text", "json"}, "default": "auto"}, "normalize": map[string]any{"type": "string", "enum": []string{"raw", "trim", "json"}, "default": "trim"},
			"max_body_bytes": map[string]any{"type": "integer", "default": 262144, "minimum": 1024, "maximum": 1048576},
		}},
		ResultSchema: map[string]any{"type": "object", "properties": map[string]any{
			"success": map[string]any{"type": "boolean"}, "status_code": map[string]any{"type": "integer"}, "duration_ms": map[string]any{"type": "number"}, "response_headers": map[string]any{"type": "object"},
			"body_text": map[string]any{"type": "string"}, "body_json": map[string]any{"type": "object"}, "body_hash": map[string]any{"type": "string"}, "body_size": map[string]any{"type": "integer"}, "truncated": map[string]any{"type": "boolean"},
		}},
		Fields: []core.FieldDescriptor{
			{Name: "success", Label: "请求成功", Type: "boolean", Operators: []string{"is_true", "is_false"}},
			{Name: "status_code", Label: "状态码", Type: "number", Operators: []string{"equals", "not_equals", "gt", "gte", "lt", "lte", "changed"}},
			{Name: "duration_ms", Label: "响应耗时(ms)", Type: "number", Operators: []string{"gt", "gte", "lt", "lte", "changed"}},
			{Name: "body_text", Label: "响应文本", Type: "string", Operators: []string{"equals", "not_equals", "contains", "not_contains", "regex", "changed"}},
			{Name: "body_json", Label: "响应 JSON", Type: "json", Path: true, Operators: []string{"equals", "not_equals", "contains", "gt", "gte", "lt", "lte", "changed"}},
			{Name: "body_hash", Label: "内容哈希", Type: "string", Operators: []string{"equals", "not_equals", "changed"}},
		},
	}
}

func (m *Module) ValidateConfig(raw json.RawMessage) error {
	var config map[string]any
	if err := json.Unmarshal(raw, &config); err != nil {
		return fmt.Errorf("invalid HTTP config: %w", err)
	}
	urlValue, _ := config["url"].(string)
	if urlValue == "" || (!strings.HasPrefix(urlValue, "http://") && !strings.HasPrefix(urlValue, "https://")) {
		return errors.New("url must start with http:// or https://")
	}
	method := valueString(config, "method", "GET")
	if method != "GET" && method != "HEAD" && method != "POST" {
		return fmt.Errorf("unsupported HTTP method %q", method)
	}
	return nil
}

func (m *Module) Execute(ctx context.Context, raw json.RawMessage) (core.Observation, error) {
	var config map[string]any
	if err := json.Unmarshal(raw, &config); err != nil {
		return failedObservation(), err
	}
	if err := m.ValidateConfig(raw); err != nil {
		return failedObservation(), err
	}
	timeout := time.Duration(valueInt(config, "timeout_seconds", 30)) * time.Second
	requestContext, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	method := valueString(config, "method", "GET")
	request, err := http.NewRequestWithContext(requestContext, method, valueString(config, "url", ""), strings.NewReader(valueString(config, "body", "")))
	if err != nil {
		return failedObservation(), err
	}
	if headers, ok := config["headers"].(map[string]any); ok {
		for key, value := range headers {
			request.Header.Set(key, fmt.Sprint(value))
		}
	}
	switch strings.ToLower(valueString(config, "auth_type", "none")) {
	case "basic":
		request.SetBasicAuth(valueString(config, "username", ""), valueString(config, "password", ""))
	case "bearer":
		request.Header.Set("Authorization", "Bearer "+valueString(config, "token", ""))
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: !valueBool(config, "verify_tls", true)} //nolint:gosec -- explicitly configured by the user.
	client := m.Client
	if client == nil {
		client = &http.Client{Transport: transport}
	}
	started := time.Now()
	response, err := client.Do(request)
	if err != nil {
		return core.Observation{Success: false, SchemaVersion: "1", Result: map[string]any{"success": false, "status_code": 0, "duration_ms": time.Since(started).Milliseconds()}}, err
	}
	defer response.Body.Close()
	maxBytes := int64(valueInt(config, "max_body_bytes", 262144))
	data, readErr := io.ReadAll(io.LimitReader(response.Body, maxBytes+1))
	if readErr != nil {
		return core.Observation{Success: false, SchemaVersion: "1", Result: map[string]any{"success": false, "status_code": response.StatusCode}}, readErr
	}
	truncated := int64(len(data)) > maxBytes
	if truncated {
		data = data[:maxBytes]
	}
	bodyText := normalize(string(data), valueString(config, "normalize", "trim"))
	result := map[string]any{"success": true, "status_code": response.StatusCode, "duration_ms": time.Since(started).Milliseconds(), "response_headers": flatten(response.Header), "body_text": bodyText, "body_hash": hash(bodyText), "body_size": len(data), "truncated": truncated}
	mode := strings.ToLower(valueString(config, "response_mode", "auto"))
	if mode == "json" || (mode == "auto" && strings.Contains(response.Header.Get("Content-Type"), "json")) {
		var parsed any
		if json.Unmarshal(data, &parsed) == nil {
			result["body_json"] = parsed
		}
	}
	return core.Observation{Success: true, SchemaVersion: "1", Result: result, Summary: fmt.Sprintf("HTTP %d", response.StatusCode)}, nil
}

func failedObservation() core.Observation {
	return core.Observation{Success: false, SchemaVersion: "1", Result: map[string]any{"success": false}}
}

func valueString(config map[string]any, key, fallback string) string {
	if value, ok := config[key].(string); ok {
		return value
	}
	return fallback
}
func valueInt(config map[string]any, key string, fallback int) int {
	if value, ok := config[key].(float64); ok {
		return int(value)
	}
	if value, ok := config[key].(int); ok {
		return value
	}
	return fallback
}
func valueBool(config map[string]any, key string, fallback bool) bool {
	if value, ok := config[key].(bool); ok {
		return value
	}
	return fallback
}

func normalize(value, mode string) string {
	if mode == "trim" {
		return strings.TrimSpace(value)
	}
	if mode == "json" {
		var parsed any
		if json.Unmarshal([]byte(value), &parsed) == nil {
			data, _ := json.Marshal(parsed)
			return string(data)
		}
	}
	return value
}

func flatten(headers http.Header) map[string]any {
	result := make(map[string]any, len(headers))
	for key, values := range headers {
		if len(values) > 0 {
			result[strings.ToLower(key)] = values[0]
		}
	}
	return result
}
func hash(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

var _ = strconv.Itoa
