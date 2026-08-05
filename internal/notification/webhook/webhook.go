package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"meerkit/internal/core"
)

type Notifier struct{}

func New() *Notifier { return &Notifier{} }

func (n *Notifier) Descriptor() core.NotifierDescriptor {
	return core.NotifierDescriptor{Type: "webhook", Name: "Webhook", Description: "发送 JSON 通知到自定义地址。", ConfigSchema: map[string]any{"type": "object", "required": []string{"url"}, "properties": map[string]any{
		"url": map[string]any{"type": "string", "title": "URL"}, "method": map[string]any{"type": "string", "enum": []string{"POST", "PUT"}, "default": "POST"}, "headers": map[string]any{"type": "object"},
	}}}
}

func (n *Notifier) ValidateConfig(raw json.RawMessage) error {
	var config map[string]any
	if err := json.Unmarshal(raw, &config); err != nil {
		return err
	}
	if stringValue(config, "url", "") == "" {
		return fmt.Errorf("webhook url is required")
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
	body, _ := json.Marshal(event)
	request, err := http.NewRequestWithContext(ctx, stringValue(config, "method", "POST"), stringValue(config, "url", ""), strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	if headers, ok := config["headers"].(map[string]any); ok {
		for key, value := range headers {
			request.Header.Set(key, fmt.Sprint(value))
		}
	}
	response, err := (&http.Client{}).Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("webhook returned HTTP %d", response.StatusCode)
	}
	return nil
}

func stringValue(config map[string]any, key, fallback string) string {
	if value, ok := config[key].(string); ok {
		return value
	}
	return fallback
}
