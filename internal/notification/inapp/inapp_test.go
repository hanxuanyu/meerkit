package inapp

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"meerkit/internal/core"
)

type fakeStore struct {
	notification core.InAppNotification
}

func (s *fakeStore) CreateInAppNotification(_ context.Context, notification core.InAppNotification) error {
	s.notification = notification
	return nil
}

func (*fakeStore) CountUnreadInAppNotifications(context.Context) (int, error) { return 1, nil }

func TestNotifierRendersAndPublishesNotification(t *testing.T) {
	store := &fakeStore{}
	hub := NewHub()
	stream, cancel := hub.Subscribe()
	defer cancel()
	notifier := New(store, hub)
	event := core.NotificationEvent{
		EventType: "triggered", MonitorID: "monitor-1", RecordID: "record-1", MonitorName: "Production API", ModuleType: "http",
		TriggeredAt: time.Now().UTC(), Summary: "status changed", CurrentResult: map[string]any{"summary": map[string]any{"duration_ms": 120}},
	}
	config := json.RawMessage(`{"title_template":"{{monitor.name}} 告警","body_template":"{{event.summary}} · {{result.summary.duration_ms}}ms"}`)
	if err := notifier.Send(context.Background(), config, event); err != nil {
		t.Fatal(err)
	}
	if store.notification.Title != "Production API 告警" || store.notification.Content != "status changed · 120ms" || store.notification.RecordID != "record-1" {
		t.Fatalf("unexpected notification: %+v", store.notification)
	}
	select {
	case update := <-stream:
		if update.Type != "created" || update.Notification == nil || update.Notification.ID != store.notification.ID || update.UnreadCount != 1 {
			t.Fatalf("unexpected stream event: %+v", update)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for notification event")
	}
}
