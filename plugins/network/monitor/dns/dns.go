package dnsmonitor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/hanxuanyu/meerkit/sdk"
	"github.com/miekg/dns"
)

const resultSchemaVersion = "1"

var recordTypes = map[string]uint16{
	"A": dns.TypeA, "AAAA": dns.TypeAAAA, "CNAME": dns.TypeCNAME, "MX": dns.TypeMX,
	"TXT": dns.TypeTXT, "NS": dns.TypeNS, "SRV": dns.TypeSRV, "CAA": dns.TypeCAA, "PTR": dns.TypePTR,
}

type ExchangeFunc func(context.Context, string, string, *dns.Msg) (*dns.Msg, error)

type Module struct{ Exchange ExchangeFunc }

func New() *Module { return &Module{} }

func (m *Module) Descriptor() sdk.ModuleDescriptor {
	properties := map[string]any{
		"name":              map[string]any{"type": "string", "minLength": 1, "maxLength": 253},
		"record_type":       map[string]any{"type": "string", "enum": sortedRecordTypes(), "default": "A"},
		"server":            map[string]any{"type": "string", "minLength": 1, "maxLength": 253},
		"port":              map[string]any{"type": "integer", "default": 53, "minimum": 1, "maximum": 65535},
		"transport":         map[string]any{"type": "string", "enum": []string{"auto", "udp", "tcp"}, "default": "auto"},
		"timeout_seconds":   map[string]any{"type": "integer", "default": 5, "minimum": 1, "maximum": 60},
		"recursion_desired": map[string]any{"type": "boolean", "default": true},
		"dnssec":            map[string]any{"type": "boolean", "default": false},
	}
	fields := dnsFields()
	return sdk.ModuleDescriptor{
		Type: "dns", Version: "1", ConfigVersion: "1", ResultSchemaVersion: resultSchemaVersion,
		Name: "DNS", Description: "向指定 DNS 服务器查询记录并观察解析结果。",
		ListSummary:  &sdk.ModuleListSummaryDescriptor{Fields: []string{"name", "record_type", "server"}, Separator: " "},
		ConfigSchema: map[string]any{"type": "object", "required": []string{"name", "server"}, "properties": properties},
		Parameters: []sdk.ParameterDescriptor{
			{Key: "name", Label: "查询名称", Type: sdk.ParameterString, Required: true, Order: 10, Placeholder: "example.com"},
			{Key: "record_type", Label: "记录类型", Type: sdk.ParameterList, Default: "A", Order: 20, Options: recordTypeOptions()},
			{Key: "server", Label: "DNS 服务器", Type: sdk.ParameterString, Required: true, Order: 30, Placeholder: "1.1.1.1"},
			{Key: "port", Label: "端口", Type: sdk.ParameterInteger, Default: 53, Minimum: sdk.Float64(1), Maximum: sdk.Float64(65535), Order: 40},
			{Key: "transport", Label: "传输协议", Type: sdk.ParameterList, Default: "auto", Options: []sdk.ParameterOption{{Value: "auto", Label: "自动（UDP/TCP）"}, {Value: "udp", Label: "UDP"}, {Value: "tcp", Label: "TCP"}}, Order: 50},
			{Key: "timeout_seconds", Label: "查询超时", Type: sdk.ParameterDuration, Default: 5, Minimum: sdk.Float64(1), Maximum: sdk.Float64(60), Unit: "秒", Order: 60},
			{Key: "recursion_desired", Label: "请求递归解析", Type: sdk.ParameterBoolean, Default: true, Order: 70},
			{Key: "dnssec", Label: "请求 DNSSEC 数据", Type: sdk.ParameterBoolean, Default: false, Order: 80},
		},
		ResultSchema: map[string]any{"type": "object", "properties": resultProperties()},
		Fields:       legacyFields(fields),
		ResultSets:   []sdk.ResultSetDescriptor{{Key: "query", Label: "DNS 查询", Description: "DNS 响应、记录和解析耗时。", Fields: fields}},
	}
}

func (m *Module) ValidateConfig(raw json.RawMessage) error {
	var config map[string]any
	if err := json.Unmarshal(raw, &config); err != nil {
		return fmt.Errorf("invalid DNS config: %w", err)
	}
	name := strings.TrimSpace(stringValue(config, "name"))
	if name == "" || len(name) > 253 {
		return errors.New("name must contain between 1 and 253 characters")
	}
	typeName := strings.ToUpper(stringValue(config, "record_type"))
	if typeName == "" {
		typeName = "A"
	}
	if _, ok := recordTypes[typeName]; !ok {
		return fmt.Errorf("unsupported DNS record type %q", typeName)
	}
	server := strings.TrimSpace(stringValue(config, "server"))
	if server == "" || strings.ContainsAny(server, " \t\r\n") {
		return errors.New("server must be a hostname or IP address")
	}
	if port := intValue(config, "port", 53); port < 1 || port > 65535 {
		return errors.New("port must be between 1 and 65535")
	}
	transport := strings.ToLower(stringValue(config, "transport"))
	if transport == "" {
		transport = "auto"
	}
	if transport != "auto" && transport != "udp" && transport != "tcp" {
		return errors.New("transport must be auto, udp, or tcp")
	}
	if timeout := intValue(config, "timeout_seconds", 5); timeout < 1 || timeout > 60 {
		return errors.New("timeout_seconds must be between 1 and 60")
	}
	return nil
}

func (m *Module) Execute(ctx context.Context, raw json.RawMessage) (sdk.Observation, error) {
	var config map[string]any
	if err := json.Unmarshal(raw, &config); err != nil {
		return failedObservation(err), err
	}
	if err := m.ValidateConfig(raw); err != nil {
		return failedObservation(err), err
	}
	name := strings.TrimSpace(stringValue(config, "name"))
	recordType := strings.ToUpper(stringValue(config, "record_type"))
	if recordType == "" {
		recordType = "A"
	}
	server := net.JoinHostPort(strings.TrimSpace(stringValue(config, "server")), strconv.Itoa(intValue(config, "port", 53)))
	transport := strings.ToLower(stringValue(config, "transport"))
	if transport == "" {
		transport = "auto"
	}
	started := time.Now()
	message := new(dns.Msg)
	message.SetQuestion(questionName(name, recordType), recordTypes[recordType])
	message.RecursionDesired = boolValue(config, "recursion_desired", true)
	if boolValue(config, "dnssec", false) {
		message.SetEdns0(1232, true)
	}
	exchange := m.Exchange
	if exchange == nil {
		exchange = defaultExchange
	}
	response, usedTransport, err := query(ctx, exchange, server, transport, message, time.Duration(intValue(config, "timeout_seconds", 5))*time.Second)
	duration := time.Since(started).Milliseconds()
	if err != nil {
		return failedResult(err, name, recordType, server, duration)
	}
	result := responseResult(response, questionName(name, recordType), recordType, server, usedTransport, duration)
	return sdk.Observation{Success: true, SchemaVersion: resultSchemaVersion, Result: result, ResultSets: map[string]map[string]any{"query": clone(result)}, Summary: fmt.Sprintf("DNS %s %s：%d 条记录", recordType, name, result["answer_count"])}, nil
}

func query(ctx context.Context, exchange ExchangeFunc, server, transport string, message *dns.Msg, timeout time.Duration) (*dns.Msg, string, error) {
	if transport == "udp" || transport == "tcp" {
		response, err := exchangeWithTimeout(ctx, exchange, timeout, transport, server, message)
		return response, transport, err
	}
	response, err := exchangeWithTimeout(ctx, exchange, timeout, "udp", server, message)
	if err == nil && !response.Truncated {
		return response, "udp", nil
	}
	if err != nil {
		return nil, "udp", err
	}
	response, err = exchangeWithTimeout(ctx, exchange, timeout, "tcp", server, message)
	return response, "tcp", err
}

func defaultExchange(ctx context.Context, network, server string, message *dns.Msg) (*dns.Msg, error) {
	client := &dns.Client{Net: network}
	response, _, err := client.ExchangeContext(ctx, message, server)
	return response, err
}

func responseResult(response *dns.Msg, name, recordType, server, transport string, duration int64) map[string]any {
	answers := make([]map[string]any, 0, len(response.Answer))
	values := make([]string, 0, len(response.Answer))
	minTTL := 0
	for _, rr := range response.Answer {
		item, value := normalizeRR(rr)
		answers = append(answers, item)
		values = append(values, value)
		if ttl := rr.Header().Ttl; minTTL == 0 || ttl < uint32(minTTL) {
			minTTL = int(ttl)
		}
	}
	return map[string]any{"responded": true, "query_name": dns.Fqdn(name), "query_type": recordType, "server": server, "transport": transport, "rcode": dns.RcodeToString[response.Rcode], "authoritative": response.Authoritative, "truncated": response.Truncated, "recursion_available": response.RecursionAvailable, "authenticated_data": response.AuthenticatedData, "answer_count": len(response.Answer), "authority_count": len(response.Ns), "additional_count": len(response.Extra), "min_ttl": minTTL, "answers": answers, "answer_values": values, "duration_ms": duration}
}

func normalizeRR(rr dns.RR) (map[string]any, string) {
	item := map[string]any{"name": rr.Header().Name, "type": dns.TypeToString[rr.Header().Rrtype], "ttl": int(rr.Header().Ttl)}
	value := ""
	switch record := rr.(type) {
	case *dns.A:
		value = record.A.String()
	case *dns.AAAA:
		value = record.AAAA.String()
	case *dns.CNAME:
		value = record.Target
		item["target"] = value
	case *dns.NS:
		value = record.Ns
		item["target"] = value
	case *dns.PTR:
		value = record.Ptr
		item["target"] = value
	case *dns.MX:
		value = record.Mx
		item["priority"] = int(record.Preference)
		item["target"] = value
	case *dns.SRV:
		value = record.Target
		item["priority"] = int(record.Priority)
		item["weight"] = int(record.Weight)
		item["port"] = int(record.Port)
		item["target"] = value
	case *dns.TXT:
		value = strings.Join(record.Txt, "")
		item["value"] = value
	case *dns.CAA:
		value = record.Value
		item["flags"] = int(record.Flag)
		item["tag"] = record.Tag
		item["value"] = value
	default:
		value = rr.String()
		item["value"] = value
	}
	item["value"] = value
	return item, value
}

func dnsFields() []sdk.ResultFieldDescriptor {
	stringOps := []string{"equals", "not_equals", "contains", "regex", "changed"}
	numberOps := []string{"equals", "gt", "gte", "lt", "lte", "changed"}
	boolOps := []string{"is_true", "is_false", "changed"}
	return []sdk.ResultFieldDescriptor{
		{Name: "responded", Label: "收到响应", Type: "boolean", Operators: boolOps}, {Name: "query_name", Label: "查询名称", Type: "string", Operators: stringOps}, {Name: "query_type", Label: "记录类型", Type: "string", Operators: stringOps}, {Name: "server", Label: "DNS 服务器", Type: "string", Operators: stringOps}, {Name: "transport", Label: "传输协议", Type: "string", Operators: stringOps}, {Name: "rcode", Label: "响应码", Type: "string", Operators: stringOps}, {Name: "authoritative", Label: "权威响应", Type: "boolean", Operators: boolOps}, {Name: "truncated", Label: "响应截断", Type: "boolean", Operators: boolOps}, {Name: "recursion_available", Label: "支持递归", Type: "boolean", Operators: boolOps}, {Name: "authenticated_data", Label: "DNSSEC 已验证标志", Type: "boolean", Operators: boolOps}, {Name: "answer_count", Label: "Answer 数量", Type: "number", Operators: numberOps}, {Name: "authority_count", Label: "Authority 数量", Type: "number", Operators: numberOps}, {Name: "additional_count", Label: "Additional 数量", Type: "number", Operators: numberOps}, {Name: "min_ttl", Label: "最小 TTL", Type: "number", Unit: "秒", Operators: numberOps}, {Name: "answers", Label: "Answer 记录", Type: "json", Path: true, Operators: []string{"exists", "contains", "changed"}}, {Name: "answer_values", Label: "Answer 值", Type: "json", Path: true, Operators: []string{"exists", "contains", "changed"}}, {Name: "duration_ms", Label: "查询耗时", Type: "number", Unit: "ms", Operators: numberOps},
	}
}

func legacyFields(fields []sdk.ResultFieldDescriptor) []sdk.FieldDescriptor {
	result := make([]sdk.FieldDescriptor, 0, len(fields))
	for _, field := range fields {
		result = append(result, sdk.FieldDescriptor{Name: field.Name, Label: field.Label, Type: field.Type, Operators: field.Operators, Path: field.Path})
	}
	return result
}
func resultProperties() map[string]any {
	return map[string]any{"responded": map[string]any{"type": "boolean"}, "query_name": map[string]any{"type": "string"}, "query_type": map[string]any{"type": "string"}, "server": map[string]any{"type": "string"}, "transport": map[string]any{"type": "string"}, "rcode": map[string]any{"type": "string"}, "authoritative": map[string]any{"type": "boolean"}, "truncated": map[string]any{"type": "boolean"}, "recursion_available": map[string]any{"type": "boolean"}, "authenticated_data": map[string]any{"type": "boolean"}, "answer_count": map[string]any{"type": "integer"}, "authority_count": map[string]any{"type": "integer"}, "additional_count": map[string]any{"type": "integer"}, "min_ttl": map[string]any{"type": "integer"}, "answers": map[string]any{"type": "array"}, "answer_values": map[string]any{"type": "array"}, "duration_ms": map[string]any{"type": "integer"}}
}
func sortedRecordTypes() []string {
	return []string{"A", "AAAA", "CNAME", "MX", "TXT", "NS", "SRV", "CAA", "PTR"}
}
func recordTypeOptions() []sdk.ParameterOption {
	result := make([]sdk.ParameterOption, 0, len(sortedRecordTypes()))
	for _, value := range sortedRecordTypes() {
		result = append(result, sdk.ParameterOption{Value: value, Label: value})
	}
	return result
}
func questionName(name, recordType string) string {
	if recordType == "PTR" && net.ParseIP(name) != nil {
		if reverse, err := dns.ReverseAddr(name); err == nil {
			return reverse
		}
	}
	return dns.Fqdn(name)
}
func stringValue(config map[string]any, key string) string {
	value, _ := config[key].(string)
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
func boolValue(config map[string]any, key string, fallback bool) bool {
	value, ok := config[key].(bool)
	if !ok {
		return fallback
	}
	return value
}
func clone(value map[string]any) map[string]any {
	result := make(map[string]any, len(value))
	for key, item := range value {
		result[key] = item
	}
	return result
}
func exchangeWithTimeout(ctx context.Context, exchange ExchangeFunc, timeout time.Duration, network, server string, message *dns.Msg) (*dns.Msg, error) {
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return exchange(runCtx, network, server, message)
}
func failedObservation(err error) sdk.Observation {
	result := map[string]any{"responded": false, "answer_count": 0}
	return sdk.Observation{Success: false, SchemaVersion: resultSchemaVersion, Result: result, ResultSets: map[string]map[string]any{"query": clone(result)}, ErrorCode: "dns_error", ErrorMessage: err.Error()}
}
func failedResult(err error, name, recordType, server string, duration int64) (sdk.Observation, error) {
	result := map[string]any{"responded": false, "query_name": dns.Fqdn(name), "query_type": recordType, "server": server, "transport": "", "rcode": "", "answer_count": 0, "answer_values": []string{}, "answers": []map[string]any{}, "duration_ms": duration}
	return sdk.Observation{Success: false, SchemaVersion: resultSchemaVersion, Result: result, ResultSets: map[string]map[string]any{"query": clone(result)}, ErrorCode: "dns_error", ErrorMessage: err.Error()}, err
}
