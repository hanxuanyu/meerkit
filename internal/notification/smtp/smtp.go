package smtp

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/smtp"
	"strconv"
	"strings"

	"meerkit/internal/core"
	templateutil "meerkit/internal/template"
)

type Notifier struct{}

func New() *Notifier { return &Notifier{} }

func (n *Notifier) Descriptor() core.NotifierDescriptor {
	return core.NotifierDescriptor{Type: "smtp", Name: "SMTP 邮件", Description: "通过 SMTP 发送告警和恢复邮件，主题和正文支持结果占位符。", ConfigSchema: map[string]any{"type": "object", "required": []string{"host", "from", "to"}, "properties": map[string]any{
		"host": map[string]any{"type": "string"}, "port": map[string]any{"type": "integer", "default": 587}, "username": map[string]any{"type": "string"}, "password": map[string]any{"type": "string", "secret": true}, "from": map[string]any{"type": "string"}, "to": map[string]any{"type": "string", "description": "多个地址使用逗号分隔"}, "subject_template": map[string]any{"type": "string", "default": "{{event.type}} · {{monitor.name}}"}, "body_template": map[string]any{"type": "string", "multiline": true, "default": "监控项：{{monitor.name}}\n模块：{{monitor.module_type}}\n状态：{{event.type}}\n时间：{{event.triggered_at}}\n摘要：{{event.summary}}\n\n当前结果：\n{{result}}"},
	}}, Parameters: []core.ParameterDescriptor{
		{Key: "host", Label: "SMTP 主机", Type: core.ParameterString, Required: true, Placeholder: "smtp.example.com", Order: 10},
		{Key: "port", Label: "端口", Type: core.ParameterInteger, Default: 587, Minimum: core.Float64(1), Maximum: core.Float64(65535), Order: 20},
		{Key: "from", Label: "发件人", Type: core.ParameterEmail, Required: true, Placeholder: "alert@example.com", Order: 30},
		{Key: "username", Label: "用户名", Type: core.ParameterString, Order: 40},
		{Key: "password", Label: "密码", Type: core.ParameterString, Secret: true, Order: 50},
		{Key: "subject_template", Label: "主题模板", Type: core.ParameterString, Default: "{{event.type}} · {{monitor.name}}", Order: 60, Description: "支持 {{monitor.name}}、{{event.type}} 和 {{result.field}}。"},
		{Key: "to", Label: "收件人", Type: core.ParameterText, FullWidth: true, Required: true, Rows: 3, Order: 100, Description: "多个地址使用逗号分隔。"},
		{Key: "body_template", Label: "正文模板", Type: core.ParameterText, FullWidth: true, Rows: 9, Order: 110, Default: "监控项：{{monitor.name}}\n模块：{{monitor.module_type}}\n状态：{{event.type}}\n时间：{{event.triggered_at}}\n摘要：{{event.summary}}\n\n当前结果：\n{{result}}", Description: "支持从当前结果集中取值的占位符。"},
	}}
}

func (n *Notifier) ValidateConfig(raw json.RawMessage) error {
	var config map[string]any
	if err := json.Unmarshal(raw, &config); err != nil {
		return err
	}
	for _, key := range []string{"host", "from", "to"} {
		if stringValue(config, key, "") == "" {
			return fmt.Errorf("smtp %s is required", key)
		}
	}
	return nil
}

func (n *Notifier) Send(ctx context.Context, raw json.RawMessage, event core.NotificationEvent) error {
	var config map[string]any
	if err := json.Unmarshal(raw, &config); err != nil {
		return err
	}
	if err := n.ValidateConfig(raw); err != nil {
		return err
	}
	rendered, missing, err := templateutil.Render(config, templateutil.NewContext(event))
	if err != nil {
		return err
	}
	if len(missing) > 0 {
		return fmt.Errorf("smtp template placeholders not found: %s", strings.Join(missing, ", "))
	}
	config, ok := rendered.(map[string]any)
	if !ok {
		return fmt.Errorf("invalid SMTP configuration")
	}
	host, port := stringValue(config, "host", ""), intValue(config, "port", 587)
	from, recipients := stringValue(config, "from", ""), splitRecipients(stringValue(config, "to", ""))
	templateContext := templateutil.NewContext(event)
	subject, err := templateutil.MustRenderString(stringValue(config, "subject_template", "{{event.type}} · {{monitor.name}}"), templateContext)
	if err != nil {
		return err
	}
	body, err := templateutil.MustRenderString(stringValue(config, "body_template", "当前结果：\n{{result}}"), templateContext)
	if err != nil {
		return err
	}
	message := []byte("From: " + from + "\r\nTo: " + strings.Join(recipients, ",") + "\r\nSubject: " + subject + "\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n" + body)
	address := host + ":" + strconv.Itoa(port)
	var auth smtp.Auth
	if username := stringValue(config, "username", ""); username != "" {
		auth = smtp.PlainAuth("", username, stringValue(config, "password", ""), host)
	}
	if port == 465 {
		return sendTLS(address, host, auth, from, recipients, message)
	}
	return smtp.SendMail(address, auth, from, recipients, message)
}

func sendTLS(address, host string, auth smtp.Auth, from string, recipients []string, message []byte) error {
	connection, err := tls.Dial("tcp", address, &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12})
	if err != nil {
		return err
	}
	client, err := smtp.NewClient(connection, host)
	if err != nil {
		return err
	}
	defer client.Close()
	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return err
		}
	}
	if err := client.Mail(from); err != nil {
		return err
	}
	for _, recipient := range recipients {
		if err := client.Rcpt(recipient); err != nil {
			return err
		}
	}
	writer, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := writer.Write(message); err != nil {
		return err
	}
	return writer.Close()
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
func splitRecipients(value string) []string {
	var result []string
	for _, part := range strings.Split(value, ",") {
		if item := strings.TrimSpace(part); item != "" {
			result = append(result, item)
		}
	}
	return result
}
