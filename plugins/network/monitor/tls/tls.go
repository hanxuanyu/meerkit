package tlsmonitor

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"math"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/hanxuanyu/meerkit/sdk"
)

const resultSchemaVersion = "1"

type DialFunc func(context.Context, string, string) (net.Conn, error)

type Module struct {
	Dial DialFunc
	Now  func() time.Time
}

func New() *Module { return &Module{} }

func (m *Module) Descriptor() sdk.ModuleDescriptor {
	connectionFields := []sdk.ResultFieldDescriptor{
		boolField("connected", "TCP 已连接"), boolField("handshake_completed", "TLS 握手完成"), stringField("host", "连接主机"), numberField("port", "端口", ""), stringField("remote_addr", "远端地址"), stringField("server_name", "SNI/校验名称"), stringField("tls_version", "TLS 版本"), stringField("cipher_suite", "密码套件"), stringField("negotiated_protocol", "ALPN 协议"), numberField("duration_ms", "连接与握手耗时", "ms"),
	}
	certificateFields := []sdk.ResultFieldDescriptor{
		boolField("present", "存在证书"), boolField("valid", "证书有效"), boolField("hostname_valid", "主机名有效"), boolField("chain_valid", "证书链可信"), boolField("expired", "证书已过期"), boolField("not_yet_valid", "证书尚未生效"), boolField("self_signed", "叶证书自签名"), stringField("subject", "证书 Subject"), stringField("common_name", "Common Name"), stringField("issuer", "证书颁发者"), stringField("serial_number", "序列号"), jsonField("dns_names", "DNS SAN"), jsonField("ip_addresses", "IP SAN"), stringField("not_before", "生效时间"), stringField("not_after", "到期时间"), numberField("seconds_remaining", "剩余秒数", "秒"), numberField("days_remaining", "剩余天数", "天"), stringField("signature_algorithm", "签名算法"), stringField("public_key_algorithm", "公钥算法"), stringField("fingerprint_sha256", "SHA-256 指纹"), numberField("chain_length", "证书链长度", ""), jsonField("chain", "证书链摘要"), stringField("verification_error", "验证错误"),
	}
	properties := map[string]any{
		"host":                map[string]any{"type": "string", "minLength": 1, "maxLength": 253},
		"port":                map[string]any{"type": "integer", "default": 443, "minimum": 1, "maximum": 65535},
		"server_name":         map[string]any{"type": "string", "maxLength": 253},
		"timeout_seconds":     map[string]any{"type": "integer", "default": 10, "minimum": 1, "maximum": 60},
		"verify_certificate":  map[string]any{"type": "boolean", "default": true},
		"minimum_tls_version": map[string]any{"type": "string", "enum": []string{"1.0", "1.1", "1.2", "1.3"}, "default": "1.2"},
		"root_ca_pem":         map[string]any{"type": "string", "maxLength": 1048576},
	}
	return sdk.ModuleDescriptor{
		Type: "tls-certificate", Version: "1", ConfigVersion: "1", ResultSchemaVersion: resultSchemaVersion,
		Name: "TLS 证书", Description: "建立直接 TLS 连接并检查协商参数、证书链、主机名与有效期。",
		ListSummary:  &sdk.ModuleListSummaryDescriptor{Fields: []string{"host", "port"}, Separator: ":"},
		ConfigSchema: map[string]any{"type": "object", "required": []string{"host"}, "properties": properties},
		Parameters: []sdk.ParameterDescriptor{
			{Key: "host", Label: "连接主机", Type: sdk.ParameterString, Required: true, Placeholder: "example.com", Order: 10},
			{Key: "port", Label: "端口", Type: sdk.ParameterInteger, Default: 443, Minimum: sdk.Float64(1), Maximum: sdk.Float64(65535), Order: 20},
			{Key: "server_name", Label: "SNI 与证书名称", Type: sdk.ParameterString, Placeholder: "api.example.com", Description: "留空时使用连接主机；连接 IP 并校验域名时应填写。", Order: 30},
			{Key: "timeout_seconds", Label: "连接超时", Type: sdk.ParameterDuration, Default: 10, Minimum: sdk.Float64(1), Maximum: sdk.Float64(60), Unit: "秒", Order: 40},
			{Key: "verify_certificate", Label: "要求证书验证通过", Type: sdk.ParameterBoolean, Default: true, Description: "关闭后仍返回独立验证结果，但验证失败不会使本次执行失败。", Order: 50},
			{Key: "minimum_tls_version", Label: "最低 TLS 版本", Type: sdk.ParameterList, Default: "1.2", Options: []sdk.ParameterOption{{Value: "1.0", Label: "TLS 1.0"}, {Value: "1.1", Label: "TLS 1.1"}, {Value: "1.2", Label: "TLS 1.2"}, {Value: "1.3", Label: "TLS 1.3"}}, Order: 60},
			{Key: "root_ca_pem", Label: "额外根 CA（PEM）", Type: sdk.ParameterText, FullWidth: true, Rows: 8, Secret: true, Description: "附加到系统根证书池，用于企业或私有 CA。", Order: 70},
		},
		ResultSchema: map[string]any{"type": "object", "properties": map[string]any{"connection": map[string]any{"type": "object"}, "certificate": map[string]any{"type": "object"}}},
		Fields:       append(legacyFields("connection", connectionFields), legacyFields("certificate", certificateFields)...),
		ResultSets:   []sdk.ResultSetDescriptor{{Key: "connection", Label: "TLS 连接", Fields: connectionFields}, {Key: "certificate", Label: "服务端证书", Fields: certificateFields}},
	}
}

func (m *Module) ValidateConfig(raw json.RawMessage) error {
	var config map[string]any
	if err := json.Unmarshal(raw, &config); err != nil {
		return fmt.Errorf("invalid TLS config: %w", err)
	}
	host := strings.TrimSpace(stringValue(config, "host"))
	if host == "" || len(host) > 253 || strings.ContainsAny(host, " \t\r\n") {
		return errors.New("host must be a hostname or IP address")
	}
	if port := intValue(config, "port", 443); port < 1 || port > 65535 {
		return errors.New("port must be between 1 and 65535")
	}
	if name := strings.TrimSpace(stringValue(config, "server_name")); len(name) > 253 || strings.ContainsAny(name, " \t\r\n") {
		return errors.New("server_name must be a hostname or IP address")
	}
	if timeout := intValue(config, "timeout_seconds", 10); timeout < 1 || timeout > 60 {
		return errors.New("timeout_seconds must be between 1 and 60")
	}
	if _, ok := tlsVersions[defaultString(stringValue(config, "minimum_tls_version"), "1.2")]; !ok {
		return errors.New("minimum_tls_version must be 1.0, 1.1, 1.2, or 1.3")
	}
	if roots := stringValue(config, "root_ca_pem"); len(roots) > 1048576 {
		return errors.New("root_ca_pem cannot exceed 1048576 characters")
	} else if roots != "" {
		if _, err := parseExtraRoots(roots); err != nil {
			return err
		}
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
	port := intValue(config, "port", 443)
	serverName := strings.TrimSpace(stringValue(config, "server_name"))
	if serverName == "" {
		serverName = host
	}
	timeout := time.Duration(intValue(config, "timeout_seconds", 10)) * time.Second
	started := time.Now()
	address := net.JoinHostPort(host, strconv.Itoa(port))
	dial := m.Dial
	if dial == nil {
		dialer := &net.Dialer{Timeout: timeout}
		dial = dialer.DialContext
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	connection, err := dial(runCtx, "tcp", address)
	connectionResult := map[string]any{"connected": err == nil, "handshake_completed": false, "host": host, "port": port, "remote_addr": "", "server_name": serverName, "tls_version": "", "cipher_suite": "", "negotiated_protocol": "", "duration_ms": time.Since(started).Milliseconds()}
	if err != nil {
		return failureWithResults(err, "connection_failed", connectionResult, emptyCertificate()), err
	}
	defer connection.Close()
	connectionResult["remote_addr"] = connection.RemoteAddr().String()
	tlsConnection := tls.Client(connection, &tls.Config{ServerName: serverName, InsecureSkipVerify: true, MinVersion: tlsVersions[defaultString(stringValue(config, "minimum_tls_version"), "1.2")]})
	if err = tlsConnection.HandshakeContext(runCtx); err != nil {
		connectionResult["duration_ms"] = time.Since(started).Milliseconds()
		return failureWithResults(err, "handshake_failed", connectionResult, emptyCertificate()), err
	}
	state := tlsConnection.ConnectionState()
	connectionResult["handshake_completed"] = true
	connectionResult["tls_version"] = tlsVersionName(state.Version)
	connectionResult["cipher_suite"] = tls.CipherSuiteName(state.CipherSuite)
	connectionResult["negotiated_protocol"] = state.NegotiatedProtocol
	connectionResult["duration_ms"] = time.Since(started).Milliseconds()
	if len(state.PeerCertificates) == 0 {
		err = errors.New("TLS server did not provide a certificate")
		return failureWithResults(err, "certificate_missing", connectionResult, emptyCertificate()), err
	}
	now := time.Now().UTC()
	if m.Now != nil {
		now = m.Now().UTC()
	}
	roots, rootsErr := parseExtraRoots(stringValue(config, "root_ca_pem"))
	if rootsErr != nil {
		return failureWithResults(rootsErr, "invalid_root_ca", connectionResult, emptyCertificate()), rootsErr
	}
	certificateResult := inspectCertificate(state.PeerCertificates, serverName, roots, now)
	result := map[string]any{"connection": connectionResult, "certificate": certificateResult}
	sets := map[string]map[string]any{"connection": clone(connectionResult), "certificate": clone(certificateResult)}
	if boolValue(config, "verify_certificate", true) && !certificateResult["valid"].(bool) {
		err = errors.New(certificateResult["verification_error"].(string))
		return sdk.Observation{Success: false, SchemaVersion: resultSchemaVersion, Result: result, ResultSets: sets, Summary: "TLS 证书验证失败", ErrorCode: "certificate_invalid", ErrorMessage: err.Error()}, err
	}
	return sdk.Observation{Success: true, SchemaVersion: resultSchemaVersion, Result: result, ResultSets: sets, Summary: fmt.Sprintf("TLS %s，证书剩余 %d 天", connectionResult["tls_version"], certificateResult["days_remaining"])}, nil
}

func inspectCertificate(certificates []*x509.Certificate, serverName string, roots *x509.CertPool, now time.Time) map[string]any {
	leaf := certificates[0]
	intermediates := x509.NewCertPool()
	for _, certificate := range certificates[1:] {
		intermediates.AddCert(certificate)
	}
	chainErr := verifyChain(leaf, roots, intermediates, now)
	hostnameErr := leaf.VerifyHostname(serverName)
	expired := now.After(leaf.NotAfter)
	notYetValid := now.Before(leaf.NotBefore)
	valid := chainErr == nil && hostnameErr == nil && !expired && !notYetValid
	verificationErrors := make([]string, 0, 2)
	if chainErr != nil {
		verificationErrors = append(verificationErrors, chainErr.Error())
	}
	if hostnameErr != nil {
		verificationErrors = append(verificationErrors, hostnameErr.Error())
	}
	seconds := leaf.NotAfter.Sub(now).Seconds()
	ipAddresses := make([]string, 0, len(leaf.IPAddresses))
	for _, address := range leaf.IPAddresses {
		ipAddresses = append(ipAddresses, address.String())
	}
	chain := make([]map[string]any, 0, len(certificates))
	for index, certificate := range certificates {
		chain = append(chain, certificateSummary(index, certificate))
	}
	return map[string]any{"present": true, "valid": valid, "hostname_valid": hostnameErr == nil, "chain_valid": chainErr == nil, "expired": expired, "not_yet_valid": notYetValid, "self_signed": leaf.RawSubject != nil && string(leaf.RawSubject) == string(leaf.RawIssuer) && leaf.CheckSignatureFrom(leaf) == nil, "subject": leaf.Subject.String(), "common_name": leaf.Subject.CommonName, "issuer": leaf.Issuer.String(), "serial_number": strings.ToUpper(leaf.SerialNumber.Text(16)), "dns_names": append([]string(nil), leaf.DNSNames...), "ip_addresses": ipAddresses, "not_before": leaf.NotBefore.UTC().Format(time.RFC3339), "not_after": leaf.NotAfter.UTC().Format(time.RFC3339), "seconds_remaining": int64(math.Floor(seconds)), "days_remaining": int64(math.Floor(seconds / 86400)), "signature_algorithm": leaf.SignatureAlgorithm.String(), "public_key_algorithm": leaf.PublicKeyAlgorithm.String(), "fingerprint_sha256": fingerprint(leaf.Raw), "chain_length": len(certificates), "chain": chain, "verification_error": strings.Join(verificationErrors, "; ")}
}

func verifyChain(leaf *x509.Certificate, roots, intermediates *x509.CertPool, now time.Time) error {
	_, err := leaf.Verify(x509.VerifyOptions{Roots: roots, Intermediates: intermediates, CurrentTime: now, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}})
	return err
}
func parseExtraRoots(value string) (*x509.CertPool, error) {
	roots, err := x509.SystemCertPool()
	if err != nil || roots == nil {
		roots = x509.NewCertPool()
	}
	if strings.TrimSpace(value) == "" {
		return roots, nil
	}
	rest := []byte(value)
	added := false
	for len(rest) > 0 {
		block, remaining := pem.Decode(rest)
		if block == nil {
			break
		}
		rest = remaining
		if block.Type != "CERTIFICATE" {
			continue
		}
		certificate, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("root_ca_pem contains an invalid certificate: %w", err)
		}
		roots.AddCert(certificate)
		added = true
	}
	if !added || len(strings.TrimSpace(string(rest))) > 0 {
		return nil, errors.New("root_ca_pem must contain only valid PEM certificates")
	}
	return roots, nil
}
func certificateSummary(position int, certificate *x509.Certificate) map[string]any {
	return map[string]any{"position": position, "subject": certificate.Subject.String(), "issuer": certificate.Issuer.String(), "not_after": certificate.NotAfter.UTC().Format(time.RFC3339), "fingerprint_sha256": fingerprint(certificate.Raw)}
}
func fingerprint(data []byte) string {
	digest := sha256.Sum256(data)
	encoded := strings.ToUpper(hex.EncodeToString(digest[:]))
	parts := make([]string, 0, len(encoded)/2)
	for index := 0; index < len(encoded); index += 2 {
		parts = append(parts, encoded[index:index+2])
	}
	return strings.Join(parts, ":")
}

var tlsVersions = map[string]uint16{"1.0": tls.VersionTLS10, "1.1": tls.VersionTLS11, "1.2": tls.VersionTLS12, "1.3": tls.VersionTLS13}

func tlsVersionName(version uint16) string {
	switch version {
	case tls.VersionTLS10:
		return "TLS 1.0"
	case tls.VersionTLS11:
		return "TLS 1.1"
	case tls.VersionTLS12:
		return "TLS 1.2"
	case tls.VersionTLS13:
		return "TLS 1.3"
	default:
		return fmt.Sprintf("0x%04X", version)
	}
}
func emptyCertificate() map[string]any {
	return map[string]any{"present": false, "valid": false, "hostname_valid": false, "chain_valid": false, "expired": false, "not_yet_valid": false, "self_signed": false, "dns_names": []string{}, "ip_addresses": []string{}, "chain": []map[string]any{}, "chain_length": 0, "verification_error": ""}
}
func failureWithResults(err error, code string, connection, certificate map[string]any) sdk.Observation {
	result := map[string]any{"connection": connection, "certificate": certificate}
	return sdk.Observation{Success: false, SchemaVersion: resultSchemaVersion, Result: result, ResultSets: map[string]map[string]any{"connection": clone(connection), "certificate": clone(certificate)}, Summary: "TLS 探测失败", ErrorCode: code, ErrorMessage: err.Error()}
}
func failed(err error, code string) sdk.Observation {
	return failureWithResults(err, code, map[string]any{"connected": false, "handshake_completed": false}, emptyCertificate())
}
func stringField(name, label string) sdk.ResultFieldDescriptor {
	return sdk.ResultFieldDescriptor{Name: name, Label: label, Type: "string", Operators: []string{"equals", "not_equals", "contains", "regex", "changed"}}
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
func legacyFields(prefix string, fields []sdk.ResultFieldDescriptor) []sdk.FieldDescriptor {
	result := make([]sdk.FieldDescriptor, 0, len(fields))
	for _, field := range fields {
		result = append(result, sdk.FieldDescriptor{Name: prefix + "." + field.Name, Label: field.Label, Type: field.Type, Operators: field.Operators, Path: field.Path})
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
