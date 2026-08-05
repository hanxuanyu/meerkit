package runtime

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"meerkit/internal/app"
	"meerkit/internal/core"
	"meerkit/internal/store"
)

func TestCleanupWorkerPrunesExpiredData(t *testing.T) {
	ctx := context.Background()
	database, err := store.OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	now := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
	monitor := core.Monitor{ID: "monitor-1", Name: "API", ModuleType: "http", ModuleVersion: "1", Schedules: []string{"@hourly"}, Enabled: true, ModuleConfig: json.RawMessage(`{}`), ConditionConfig: json.RawMessage(`{}`), RuntimeState: json.RawMessage(`{}`), CreatedAt: now, UpdatedAt: now}
	if err := database.CreateMonitor(ctx, monitor); err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct {
		id       string
		finished time.Time
	}{{"old-record", now.Add(-2 * time.Hour)}, {"new-record", now.Add(-30 * time.Minute)}} {
		record := core.MonitorRecord{ID: item.id, MonitorID: monitor.ID, StartedAt: item.finished.Add(-time.Second), FinishedAt: item.finished, Success: true, ResultSchemaVersion: "1", Result: map[string]any{}, ResultHash: item.id, ConditionState: "false", EventType: "none", NotificationResult: map[string]any{}}
		if err := database.AddRecord(ctx, record); err != nil {
			t.Fatal(err)
		}
	}
	for _, item := range []struct {
		id      string
		created time.Time
	}{{"old-notification", now.Add(-2 * time.Hour)}, {"new-notification", now.Add(-30 * time.Minute)}} {
		notification := core.InAppNotification{ID: item.id, ChannelID: core.BuiltInNotificationChannelID, MonitorID: monitor.ID, EventType: "triggered", Title: item.id, Content: "body", CreatedAt: item.created}
		if err := database.CreateInAppNotification(ctx, notification); err != nil {
			t.Fatal(err)
		}
	}

	config := app.DefaultConfig()
	config.Storage.Retention = "1h"
	config.Storage.NotificationRetention = "1h"
	var publishedUnread int
	worker := NewCleanupWorker(database, config, nil, func(count int) { publishedUnread = count })
	worker.run(ctx, now)

	records, err := database.ListRecordsPage(ctx, monitor.ID, store.RecordListOptions{PageSize: 20})
	if err != nil || records.Total != 1 || records.Items[0].ID != "new-record" {
		t.Fatalf("records after cleanup: %+v err=%v", records, err)
	}
	notifications, err := database.ListInAppNotificationsPage(ctx, store.NotificationListOptions{PageSize: 20})
	if err != nil || notifications.Total != 1 || notifications.Items[0].ID != "new-notification" {
		t.Fatalf("notifications after cleanup: %+v err=%v", notifications, err)
	}
	if publishedUnread != 1 {
		t.Fatalf("published unread count=%d, want 1", publishedUnread)
	}
}
