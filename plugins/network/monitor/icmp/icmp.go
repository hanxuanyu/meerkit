package icmpmonitor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net"
	"strings"
	"time"

	"github.com/hanxuanyu/meerkit/sdk"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

const resultSchemaVersion = "1"

type LookupIPFunc func(context.Context, string, string) ([]net.IP, error)
type ListenPacketFunc func(string, string) (*icmp.PacketConn, error)

type Module struct {
	LookupIP     LookupIPFunc
	ListenPacket ListenPacketFunc
}

func New() *Module { return &Module{} }

func (m *Module) Descriptor() sdk.ModuleDescriptor {
	properties := map[string]any{
		"host":            map[string]any{"type": "string", "minLength": 1, "maxLength": 253},
		"ip_version":      map[string]any{"type": "string", "enum": []string{"auto", "ipv4", "ipv6"}, "default": "auto"},
		"count":           map[string]any{"type": "integer", "default": 4, "minimum": 1, "maximum": 20},
		"interval_ms":     map[string]any{"type": "integer", "default": 250, "minimum": 100, "maximum": 5000},
		"timeout_seconds": map[string]any{"type": "integer", "default": 5, "minimum": 1, "maximum": 60},
		"packet_size":     map[string]any{"type": "integer", "default": 56, "minimum": 0, "maximum": 1400},
		"mode":            map[string]any{"type": "string", "enum": []string{"auto", "unprivileged", "raw"}, "default": "auto"},
	}
	fields := []sdk.ResultFieldDescriptor{boolField("reachable", "目标可达"), stringField("host", "目标主机"), stringField("resolved_ip", "探测 IP"), stringField("ip_version", "IP 版本"), stringField("mode", "探测模式"), numberField("packets_sent", "已发送", "个"), numberField("packets_received", "已接收", "个"), numberField("packet_loss_percent", "丢包率", "%"), numberField("min_rtt_ms", "最小 RTT", "ms"), numberField("avg_rtt_ms", "平均 RTT", "ms"), numberField("max_rtt_ms", "最大 RTT", "ms"), numberField("jitter_ms", "抖动", "ms"), jsonField("rtt_samples_ms", "RTT 样本"), numberField("duration_ms", "总耗时", "ms")}
	return sdk.ModuleDescriptor{
		Type: "icmp", Version: "1", ConfigVersion: "1", ResultSchemaVersion: resultSchemaVersion,
		Name: "ICMP 连通性", Description: "通过 ICMP Echo 请求验证目标连通性、丢包率和往返延迟。",
		ListSummary:  &sdk.ModuleListSummaryDescriptor{Fields: []string{"host", "ip_version"}, Separator: " "},
		ConfigSchema: map[string]any{"type": "object", "required": []string{"host"}, "properties": properties},
		Parameters: []sdk.ParameterDescriptor{
			{Key: "host", Label: "目标主机", Type: sdk.ParameterString, Required: true, Placeholder: "example.com 或 192.0.2.1", Order: 10},
			{Key: "ip_version", Label: "IP 版本", Type: sdk.ParameterList, Default: "auto", Options: []sdk.ParameterOption{{Value: "auto", Label: "自动"}, {Value: "ipv4", Label: "IPv4"}, {Value: "ipv6", Label: "IPv6"}}, Order: 20},
			{Key: "count", Label: "发包数量", Type: sdk.ParameterInteger, Default: 4, Minimum: sdk.Float64(1), Maximum: sdk.Float64(20), Unit: "个", Order: 30},
			{Key: "interval_ms", Label: "发包间隔", Type: sdk.ParameterDuration, Default: 250, Minimum: sdk.Float64(100), Maximum: sdk.Float64(5000), Unit: "ms", Order: 40},
			{Key: "timeout_seconds", Label: "总超时", Type: sdk.ParameterDuration, Default: 5, Minimum: sdk.Float64(1), Maximum: sdk.Float64(60), Unit: "秒", Order: 50},
			{Key: "packet_size", Label: "Payload 大小", Type: sdk.ParameterInteger, Default: 56, Minimum: sdk.Float64(0), Maximum: sdk.Float64(1400), Unit: "字节", Order: 60},
			{Key: "mode", Label: "Socket 模式", Type: sdk.ParameterList, Default: "auto", Options: []sdk.ParameterOption{{Value: "auto", Label: "自动（优先非特权）"}, {Value: "unprivileged", Label: "非特权"}, {Value: "raw", Label: "Raw Socket"}}, Order: 70},
		},
		ResultSchema: map[string]any{"type": "object", "properties": map[string]any{"reachable": map[string]any{"type": "boolean"}, "host": map[string]any{"type": "string"}, "resolved_ip": map[string]any{"type": "string"}, "ip_version": map[string]any{"type": "string"}, "mode": map[string]any{"type": "string"}, "packets_sent": map[string]any{"type": "integer"}, "packets_received": map[string]any{"type": "integer"}, "packet_loss_percent": map[string]any{"type": "number"}, "min_rtt_ms": map[string]any{"type": "number"}, "avg_rtt_ms": map[string]any{"type": "number"}, "max_rtt_ms": map[string]any{"type": "number"}, "jitter_ms": map[string]any{"type": "number"}, "rtt_samples_ms": map[string]any{"type": "array"}, "duration_ms": map[string]any{"type": "integer"}}},
		Fields:       legacyFields(fields), ResultSets: []sdk.ResultSetDescriptor{{Key: "connectivity", Label: "ICMP 连通性", Description: "ICMP Echo 请求的可达性和延迟统计。", Fields: fields}},
	}
}

func (m *Module) ValidateConfig(raw json.RawMessage) error {
	var config map[string]any
	if err := json.Unmarshal(raw, &config); err != nil {
		return fmt.Errorf("invalid ICMP config: %w", err)
	}
	host := strings.TrimSpace(stringValue(config, "host"))
	if host == "" || len(host) > 253 {
		return errors.New("host must contain between 1 and 253 characters")
	}
	version := defaultString(stringValue(config, "ip_version"), "auto")
	if version != "auto" && version != "ipv4" && version != "ipv6" {
		return errors.New("ip_version must be auto, ipv4, or ipv6")
	}
	if count := intValue(config, "count", 4); count < 1 || count > 20 {
		return errors.New("count must be between 1 and 20")
	}
	if interval := intValue(config, "interval_ms", 250); interval < 100 || interval > 5000 {
		return errors.New("interval_ms must be between 100 and 5000")
	}
	if timeout := intValue(config, "timeout_seconds", 5); timeout < 1 || timeout > 60 {
		return errors.New("timeout_seconds must be between 1 and 60")
	}
	if size := intValue(config, "packet_size", 56); size < 0 || size > 1400 {
		return errors.New("packet_size must be between 0 and 1400")
	}
	mode := defaultString(stringValue(config, "mode"), "auto")
	if mode != "auto" && mode != "unprivileged" && mode != "raw" {
		return errors.New("mode must be auto, unprivileged, or raw")
	}
	return nil
}

func (m *Module) Execute(ctx context.Context, raw json.RawMessage) (sdk.Observation, error) {
	var config map[string]any
	if err := json.Unmarshal(raw, &config); err != nil {
		return failed(err, "invalid_config"), err
	}
	if err := m.ValidateConfig(raw); err != nil {
		return failed(err, "invalid_config"), err
	}
	host := strings.TrimSpace(stringValue(config, "host"))
	version := defaultString(stringValue(config, "ip_version"), "auto")
	lookup := m.LookupIP
	if lookup == nil {
		lookup = net.DefaultResolver.LookupIP
	}
	addresses, err := lookup(ctx, "ip", host)
	if err != nil || len(addresses) == 0 {
		if err == nil {
			err = errors.New("host did not resolve to an IP address")
		}
		return failed(err, "resolve_failed"), err
	}
	address, _, ipVersion := selectAddress(addresses, version)
	if address == nil {
		err = fmt.Errorf("no %s address was found for %s", version, host)
		return failed(err, "resolve_failed"), err
	}
	mode := defaultString(stringValue(config, "mode"), "auto")
	listen := m.ListenPacket
	if listen == nil {
		listen = icmp.ListenPacket
	}
	protocol := 1
	if ipVersion == "ipv6" {
		protocol = 58
	}
	packet, actualMode, err := listenICMP(listen, ipVersion, mode)
	if err != nil {
		return failed(err, "permission_denied"), err
	}
	defer packet.Close()
	count := intValue(config, "count", 4)
	interval := time.Duration(intValue(config, "interval_ms", 250)) * time.Millisecond
	timeout := time.Duration(intValue(config, "timeout_seconds", 5)) * time.Second
	payloadSize := intValue(config, "packet_size", 56)
	identifier := int(time.Now().UnixNano() & 0xffff)
	samples := make([]float64, 0, count)
	started := time.Now()
	sent := 0
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	perPacketTimeout := timeout / time.Duration(count)
	if perPacketTimeout < 100*time.Millisecond {
		perPacketTimeout = 100 * time.Millisecond
	}
	for sequence := 0; sequence < count; sequence++ {
		if sequence > 0 {
			timer := time.NewTimer(interval)
			select {
			case <-runCtx.Done():
				timer.Stop()
				result := observation(host, address.String(), ipVersion, actualMode, sent, samples, started, runCtx.Err())
				return result, runCtx.Err()
			case <-timer.C:
			}
		}
		body := &icmp.Echo{ID: identifier, Seq: sequence, Data: make([]byte, payloadSize)}
		message := &icmp.Message{Type: echoRequestType(ipVersion), Code: 0, Body: body}
		bytes, marshalErr := message.Marshal(nil)
		if marshalErr != nil {
			return failed(marshalErr, "icmp_error"), marshalErr
		}
		destination := net.Addr(&net.UDPAddr{IP: address})
		if actualMode == "raw" {
			destination = &net.IPAddr{IP: address}
		}
		sentAt := time.Now()
		sent++
		if _, err = packet.WriteTo(bytes, destination); err != nil {
			result := observation(host, address.String(), ipVersion, actualMode, sent, samples, started, err)
			return result, err
		}
		deadline := sentAt.Add(perPacketTimeout)
		if ctxDeadline, ok := runCtx.Deadline(); ok && ctxDeadline.Before(deadline) {
			deadline = ctxDeadline
		}
		_ = packet.SetReadDeadline(deadline)
		buffer := make([]byte, 1500+payloadSize)
		for {
			readCount, _, readErr := packet.ReadFrom(buffer)
			if readErr != nil {
				break
			}
			response, parseErr := icmp.ParseMessage(protocol, buffer[:readCount])
			if parseErr != nil {
				continue
			}
			echo, ok := response.Body.(*icmp.Echo)
			if !ok || echo.Seq != sequence || (actualMode == "raw" && echo.ID != identifier) || response.Type != echoReplyType(ipVersion) {
				continue
			}
			samples = append(samples, float64(time.Since(sentAt).Microseconds())/1000)
			break
		}
	}
	result := observation(host, address.String(), ipVersion, actualMode, sent, samples, started, nil)
	if len(samples) == 0 {
		err = errors.New("no ICMP echo reply was received")
		result.ErrorCode = "timeout"
		result.ErrorMessage = err.Error()
		return result, err
	}
	return result, nil
}

func observation(host, resolved, version, mode string, sent int, samples []float64, started time.Time, executionErr error) sdk.Observation {
	received := len(samples)
	loss := 100.0
	if sent > 0 {
		loss = float64(sent-received) * 100 / float64(sent)
	}
	min, avg, max, jitter := 0.0, 0.0, 0.0, 0.0
	if received > 0 {
		min, max = samples[0], samples[0]
		sum := 0.0
		for _, sample := range samples {
			if sample < min {
				min = sample
			}
			if sample > max {
				max = sample
			}
			sum += sample
		}
		avg = sum / float64(received)
		if received > 1 {
			for index := 1; index < received; index++ {
				jitter += math.Abs(samples[index] - samples[index-1])
			}
			jitter /= float64(received - 1)
		}
	}
	result := map[string]any{"reachable": received > 0, "host": host, "resolved_ip": resolved, "ip_version": version, "mode": mode, "packets_sent": sent, "packets_received": received, "packet_loss_percent": loss, "min_rtt_ms": min, "avg_rtt_ms": avg, "max_rtt_ms": max, "jitter_ms": jitter, "rtt_samples_ms": samples, "duration_ms": time.Since(started).Milliseconds()}
	success := received > 0
	if executionErr != nil && !errors.Is(executionErr, context.DeadlineExceeded) {
		success = false
	}
	observation := sdk.Observation{Success: success, SchemaVersion: resultSchemaVersion, Result: result, ResultSets: map[string]map[string]any{"connectivity": clone(result)}, Summary: fmt.Sprintf("ICMP %s：%d/%d 个响应", host, received, sent)}
	if executionErr != nil {
		observation.ErrorCode = "icmp_error"
		observation.ErrorMessage = executionErr.Error()
	}
	return observation
}

func selectAddress(addresses []net.IP, version string) (net.IP, string, string) {
	for _, address := range addresses {
		if version == "ipv4" && address.To4() != nil {
			return address, "udp4", "ipv4"
		}
		if version == "ipv6" && address.To4() == nil {
			return address, "udp6", "ipv6"
		}
	}
	if version == "auto" && len(addresses) > 0 {
		if address := addresses[0].To4(); address != nil {
			return address, "udp4", "ipv4"
		}
		return addresses[0], "udp6", "ipv6"
	}
	return nil, "", ""
}
func echoRequestType(version string) icmp.Type {
	if version == "ipv6" {
		return ipv6.ICMPTypeEchoRequest
	}
	return ipv4.ICMPTypeEcho
}
func echoReplyType(version string) icmp.Type {
	if version == "ipv6" {
		return ipv6.ICMPTypeEchoReply
	}
	return ipv4.ICMPTypeEchoReply
}
func listenICMP(listen ListenPacketFunc, version, mode string) (*icmp.PacketConn, string, error) {
	unprivilegedNetwork, rawNetwork, address := "udp4", "ip4:icmp", "0.0.0.0"
	if version == "ipv6" {
		unprivilegedNetwork, rawNetwork, address = "udp6", "ip6:ipv6-icmp", "::"
	}
	if mode == "unprivileged" {
		connection, err := listen(unprivilegedNetwork, address)
		return connection, "unprivileged", err
	}
	if mode == "raw" {
		connection, err := listen(rawNetwork, address)
		return connection, "raw", err
	}
	connection, err := listen(unprivilegedNetwork, address)
	if err == nil {
		return connection, "unprivileged", nil
	}
	rawConnection, rawErr := listen(rawNetwork, address)
	if rawErr == nil {
		return rawConnection, "raw", nil
	}
	return nil, "", fmt.Errorf("unprivileged ICMP failed: %v; raw ICMP failed: %w", err, rawErr)
}
func failed(err error, code string) sdk.Observation {
	result := map[string]any{"reachable": false, "packets_sent": 0, "packets_received": 0, "packet_loss_percent": 100.0, "rtt_samples_ms": []float64{}}
	return sdk.Observation{Success: false, SchemaVersion: resultSchemaVersion, Result: result, ResultSets: map[string]map[string]any{"connectivity": clone(result)}, ErrorCode: code, ErrorMessage: err.Error()}
}
func stringField(name, label string) sdk.ResultFieldDescriptor {
	return sdk.ResultFieldDescriptor{Name: name, Label: label, Type: "string", Operators: []string{"equals", "not_equals", "contains", "changed"}}
}
func boolField(name, label string) sdk.ResultFieldDescriptor {
	return sdk.ResultFieldDescriptor{Name: name, Label: label, Type: "boolean", Operators: []string{"is_true", "is_false", "changed"}}
}
func numberField(name, label, unit string) sdk.ResultFieldDescriptor {
	return sdk.ResultFieldDescriptor{Name: name, Label: label, Type: "number", Unit: unit, Operators: []string{"equals", "gt", "gte", "lt", "lte", "changed"}}
}
func jsonField(name, label string) sdk.ResultFieldDescriptor {
	return sdk.ResultFieldDescriptor{Name: name, Label: label, Type: "json", Path: true, Operators: []string{"exists", "contains", "changed"}}
}
func legacyFields(fields []sdk.ResultFieldDescriptor) []sdk.FieldDescriptor {
	result := make([]sdk.FieldDescriptor, 0, len(fields))
	for _, field := range fields {
		result = append(result, sdk.FieldDescriptor{Name: field.Name, Label: field.Label, Type: field.Type, Operators: field.Operators, Path: field.Path})
	}
	return result
}
func stringValue(config map[string]any, key string) string {
	value, _ := config[key].(string)
	return value
}
func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
func intValue(config map[string]any, key string, fallback int) int {
	switch value := config[key].(type) {
	case float64:
		return int(value)
	case int:
		return value
	default:
		return fallback
	}
}
func clone(value map[string]any) map[string]any {
	result := make(map[string]any, len(value))
	for key, item := range value {
		result[key] = item
	}
	return result
}
