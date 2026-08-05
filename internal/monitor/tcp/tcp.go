package tcpmonitor

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"strconv"
	"strings"
	"time"

	"meerkit/internal/core"
)

type Module struct{}

func New() *Module { return &Module{} }

func (m *Module) Descriptor() core.ModuleDescriptor {
	return core.ModuleDescriptor{
		Type: "tcp", Version: "1", Name: "TCP", Description: "连接 TCP 服务并观察可选响应变化。",
		ListSummary: &core.ModuleListSummaryDescriptor{Fields: []string{"host", "port"}, Separator: ":"},
		ConfigSchema: map[string]any{"type": "object", "required": []string{"host", "port"}, "properties": map[string]any{
			"host": map[string]any{"type": "string", "title": "主机"}, "port": map[string]any{"type": "integer", "minimum": 1, "maximum": 65535, "title": "端口"},
			"timeout_seconds": map[string]any{"type": "integer", "default": 10, "minimum": 1, "maximum": 300}, "send": map[string]any{"type": "string", "title": "发送内容"}, "send_base64": map[string]any{"type": "boolean", "default": false},
			"read_response": map[string]any{"type": "boolean", "default": false}, "read_timeout_seconds": map[string]any{"type": "integer", "default": 3, "minimum": 1, "maximum": 60}, "max_read_bytes": map[string]any{"type": "integer", "default": 65536, "minimum": 1, "maximum": 1048576},
		}},
		Parameters: []core.ParameterDescriptor{
			{Key: "host", Label: "主机", Type: core.ParameterString, Required: true, Placeholder: "127.0.0.1", Order: 10},
			{Key: "port", Label: "端口", Type: core.ParameterInteger, Required: true, Minimum: core.Float64(1), Maximum: core.Float64(65535), Placeholder: "8080", Order: 20},
			{Key: "timeout_seconds", Label: "连接超时", Type: core.ParameterDuration, Default: 10, Minimum: core.Float64(1), Maximum: core.Float64(300), Unit: "秒", Order: 30},
			{Key: "send", Label: "发送内容", Type: core.ParameterText, FullWidth: true, Rows: 5, Description: "留空时只建立 TCP 连接。", Order: 40},
			{Key: "read_response", Label: "读取响应", Type: core.ParameterBoolean, Default: false, Order: 200, Description: "连接后读取一次服务端响应。"},
			{Key: "send_base64", Label: "发送内容使用 Base64", Type: core.ParameterBoolean, Default: false, Order: 210, Description: "将发送内容按 Base64 解码后再写入 TCP 连接。"},
			{Key: "read_timeout_seconds", Label: "读取超时", Type: core.ParameterDuration, Default: 3, Minimum: core.Float64(1), Maximum: core.Float64(60), Unit: "秒", Order: 300, VisibleWhen: []core.ParameterCondition{{Field: "read_response", Operator: "equals", Value: true}}},
			{Key: "max_read_bytes", Label: "最大读取字节数", Type: core.ParameterInteger, Default: 65536, Minimum: core.Float64(1), Maximum: core.Float64(1048576), Unit: "字节", Order: 310, VisibleWhen: []core.ParameterCondition{{Field: "read_response", Operator: "equals", Value: true}}},
		},
		ResultSchema: map[string]any{"type": "object", "properties": map[string]any{
			"success": map[string]any{"type": "boolean"}, "connected": map[string]any{"type": "boolean"}, "duration_ms": map[string]any{"type": "number"}, "remote_addr": map[string]any{"type": "string"}, "response_text": map[string]any{"type": "string"}, "response_bytes": map[string]any{"type": "string"}, "response_hash": map[string]any{"type": "string"}, "bytes_read": map[string]any{"type": "integer"},
		}},
		Fields: []core.FieldDescriptor{
			{Name: "success", Label: "执行成功", Type: "boolean", Operators: []string{"is_true", "is_false"}}, {Name: "connected", Label: "连接成功", Type: "boolean", Operators: []string{"is_true", "is_false", "changed"}},
			{Name: "duration_ms", Label: "连接耗时(ms)", Type: "number", Operators: []string{"gt", "gte", "lt", "lte", "changed"}}, {Name: "response_text", Label: "响应文本", Type: "string", Operators: []string{"equals", "not_equals", "contains", "not_contains", "regex", "changed"}},
			{Name: "response_hash", Label: "响应哈希", Type: "string", Operators: []string{"equals", "not_equals", "changed"}}, {Name: "bytes_read", Label: "响应字节数", Type: "number", Operators: []string{"equals", "gt", "gte", "lt", "lte", "changed"}},
		},
		ResultSets: []core.ResultSetDescriptor{{
			Key: "connection", Label: "TCP 连接", Description: "连接探活以及可选的单次收发结果。", Fields: []core.ResultFieldDescriptor{
				{Name: "success", Label: "执行成功", Type: "boolean", Operators: []string{"is_true", "is_false", "changed"}},
				{Name: "connected", Label: "连接成功", Type: "boolean", Operators: []string{"is_true", "is_false", "changed"}},
				{Name: "duration_ms", Label: "连接耗时", Type: "number", Unit: "ms", Operators: []string{"equals", "gt", "gte", "lt", "lte", "changed"}},
				{Name: "remote_addr", Label: "远端地址", Type: "string", Operators: []string{"equals", "not_equals", "contains", "changed"}},
				{Name: "response_text", Label: "响应文本", Type: "text", Operators: []string{"equals", "not_equals", "contains", "not_contains", "regex", "changed"}},
				{Name: "response_bytes", Label: "响应二进制", Type: "binary", Format: "base64", Operators: []string{"exists", "changed"}},
				{Name: "response_hash", Label: "响应哈希", Type: "string", Operators: []string{"equals", "not_equals", "changed"}},
				{Name: "bytes_read", Label: "响应字节数", Type: "number", Unit: "字节", Operators: []string{"equals", "gt", "gte", "lt", "lte", "changed"}},
			},
		}},
	}
}

func (m *Module) ValidateConfig(raw json.RawMessage) error {
	var config map[string]any
	if err := json.Unmarshal(raw, &config); err != nil {
		return err
	}
	if stringValue(config, "host", "") == "" {
		return errors.New("host is required")
	}
	port := intValue(config, "port", 0)
	if port < 1 || port > 65535 {
		return errors.New("port must be between 1 and 65535")
	}
	return nil
}

func (m *Module) Execute(ctx context.Context, raw json.RawMessage) (core.Observation, error) {
	var config map[string]any
	if err := json.Unmarshal(raw, &config); err != nil {
		return failed(), err
	}
	if err := m.ValidateConfig(raw); err != nil {
		return failed(), err
	}
	started := time.Now()
	address := net.JoinHostPort(stringValue(config, "host", ""), strconv.Itoa(intValue(config, "port", 0)))
	connection, err := (&net.Dialer{Timeout: time.Duration(intValue(config, "timeout_seconds", 10)) * time.Second}).DialContext(ctx, "tcp", address)
	if err != nil {
		result := map[string]any{"success": false, "connected": false, "duration_ms": time.Since(started).Milliseconds()}
		return core.Observation{Success: false, SchemaVersion: "1", Result: result, ResultSets: map[string]map[string]any{"connection": copyMap(result)}}, err
	}
	defer connection.Close()
	result := map[string]any{"success": true, "connected": true, "duration_ms": time.Since(started).Milliseconds(), "remote_addr": connection.RemoteAddr().String(), "response_text": "", "response_bytes": "", "response_hash": hash(""), "bytes_read": 0}
	if send := stringValue(config, "send", ""); send != "" {
		data := []byte(send)
		if boolValue(config, "send_base64", false) {
			data, err = base64.StdEncoding.DecodeString(send)
			if err != nil {
				return core.Observation{Success: false, SchemaVersion: "1", Result: result}, err
			}
		}
		if _, err := connection.Write(data); err != nil {
			return core.Observation{Success: false, SchemaVersion: "1", Result: result}, err
		}
	}
	if boolValue(config, "read_response", false) {
		_ = connection.SetReadDeadline(time.Now().Add(time.Duration(intValue(config, "read_timeout_seconds", 3)) * time.Second))
		data := make([]byte, intValue(config, "max_read_bytes", 65536))
		count, readErr := connection.Read(data)
		if readErr != nil && count == 0 {
			return core.Observation{Success: false, SchemaVersion: "1", Result: result}, readErr
		}
		data = data[:count]
		result["response_text"] = string(data)
		result["response_bytes"] = base64.StdEncoding.EncodeToString(data)
		result["response_hash"] = hash(string(data))
		result["bytes_read"] = count
	}
	return core.Observation{Success: true, SchemaVersion: "1", Result: result, ResultSets: map[string]map[string]any{"connection": copyMap(result)}, Summary: "TCP connected"}, nil
}

func copyMap(source map[string]any) map[string]any {
	result := make(map[string]any, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

func failed() core.Observation {
	return core.Observation{Success: false, SchemaVersion: "1", Result: map[string]any{"success": false, "connected": false}}
}
func stringValue(config map[string]any, key, fallback string) string {
	if value, ok := config[key].(string); ok {
		return value
	}
	return fallback
}
func intValue(config map[string]any, key string, fallback int) int {
	if value, ok := config[key].(float64); ok {
		return int(value)
	}
	if value, ok := config[key].(int); ok {
		return value
	}
	return fallback
}
func boolValue(config map[string]any, key string, fallback bool) bool {
	if value, ok := config[key].(bool); ok {
		return value
	}
	return fallback
}
func hash(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

var _ = strings.TrimSpace
