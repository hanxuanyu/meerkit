package inapp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"meerkit/internal/core"
	templateutil "meerkit/internal/template"
)

const (
	defaultTitleTemplate = "{{monitor.name}} · {{event.type}}"
	defaultBodyTemplate  = "{{event.summary}}"
)

type notificationStore interface {
	CreateInAppNotification(context.Context, core.InAppNotification) error
	CountUnreadInAppNotifications(context.Context) (int, error)
}

type StreamEvent struct {
	Type         string                  `json:"type"`
	Notification *core.InAppNotification `json:"notification,omitempty"`
	UnreadCount  int                     `json:"unread_count"`
}

type Hub struct {
	mu          sync.RWMutex
	subscribers map[chan StreamEvent]struct{}
}

func NewHub() *Hub {
	return &Hub{subscribers: make(map[chan StreamEvent]struct{})}
}

func (h *Hub) Subscribe() (<-chan StreamEvent, func()) {
	stream := make(chan StreamEvent, 16)
	h.mu.Lock()
	h.subscribers[stream] = struct{}{}
	h.mu.Unlock()
	var once sync.Once
	return stream, func() {
		once.Do(func() {
			h.mu.Lock()
			delete(h.subscribers, stream)
			close(stream)
			h.mu.Unlock()
		})
	}
}

func (h *Hub) Publish(event StreamEvent) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for subscriber := range h.subscribers {
		select {
		case subscriber <- event:
		default:
		}
	}
}

type Notifier struct {
	store notificationStore
	hub   *Hub
}

func New(store notificationStore, hub *Hub) *Notifier {
	return &Notifier{store: store, hub: hub}
}

func (n *Notifier) Descriptor() core.NotifierDescriptor {
	return core.NotifierDescriptor{
		Type: "inapp", Name: "站内通知", Description: "内置站内通知渠道。",
		ConfigSchema: map[string]any{"type": "object", "properties": map[string]any{
			"title_template": map[string]any{"type": "string", "default": defaultTitleTemplate},
			"body_template":  map[string]any{"type": "string", "multiline": true, "default": defaultBodyTemplate},
		}},
		Parameters: []core.ParameterDescriptor{
			{Key: "title_template", Label: "通知标题", Type: core.ParameterString, Order: 10, Default: defaultTitleTemplate},
			{Key: "body_template", Label: "通知内容", Type: core.ParameterText, FullWidth: true, Rows: 6, Order: 20, Default: defaultBodyTemplate},
		},
	}
}

func (n *Notifier) ValidateConfig(raw json.RawMessage) error {
	var config map[string]any
	if len(raw) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, &config); err != nil {
		return fmt.Errorf("decode in-app notification config: %w", err)
	}
	for _, key := range []string{"title_template", "body_template"} {
		if value, ok := config[key]; ok {
			if _, ok := value.(string); !ok {
				return fmt.Errorf("%s must be a string", key)
			}
		}
	}
	return nil
}

func (n *Notifier) Send(ctx context.Context, raw json.RawMessage, event core.NotificationEvent) error {
	var config map[string]any
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &config); err != nil {
			return fmt.Errorf("decode in-app notification config: %w", err)
		}
	}
	templateContext := templateutil.NewContext(event)
	title, err := templateutil.MustRenderString(stringValue(config, "title_template", defaultTitleTemplate), templateContext)
	if err != nil {
		return err
	}
	content, err := templateutil.MustRenderString(stringValue(config, "body_template", defaultBodyTemplate), templateContext)
	if err != nil {
		return err
	}
	notification := core.InAppNotification{
		ID: core.NewID(), ChannelID: core.BuiltInNotificationChannelID, MonitorID: event.MonitorID, RecordID: event.RecordID,
		EventType: event.EventType, Title: strings.TrimSpace(title), Content: strings.TrimSpace(content), CreatedAt: time.Now().UTC(),
	}
	if err := n.store.CreateInAppNotification(ctx, notification); err != nil {
		return err
	}
	unreadCount, _ := n.store.CountUnreadInAppNotifications(ctx)
	if n.hub != nil {
		n.hub.Publish(StreamEvent{Type: "created", Notification: &notification, UnreadCount: unreadCount})
	}
	return nil
}

func stringValue(config map[string]any, key, fallback string) string {
	if value, ok := config[key].(string); ok && value != "" {
		return value
	}
	return fallback
}
