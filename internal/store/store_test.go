package store

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"meerkit/internal/core"
)

func TestListMonitorsPageFiltersAndPaginates(t *testing.T) {
	ctx := context.Background()
	database, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	for index, item := range []struct {
		id, name, module, state string
	}{
		{"monitor-1", "Production API", "http", `{"condition_active":true,"last_success":true}`},
		{"monitor-2", "Internal TCP", "tcp", `{"condition_active":false,"last_success":true}`},
		{"monitor-3", "Staging API", "http", `{}`},
	} {
		now := time.Date(2026, 1, 1, 0, 0, index, 0, time.UTC)
		monitor := core.Monitor{ID: item.id, Name: item.name, ModuleType: item.module, ModuleVersion: "1", Schedules: []string{"@hourly"}, Enabled: item.id != "monitor-2", ModuleConfig: json.RawMessage(`{"url":"https://example.test"}`), ConditionConfig: json.RawMessage(`{"rules":[]}`), RuntimeState: json.RawMessage(item.state), CreatedAt: now, UpdatedAt: now}
		if err := database.CreateMonitor(ctx, monitor); err != nil {
			t.Fatal(err)
		}
	}

	page, err := database.ListMonitorsPage(ctx, MonitorListOptions{Page: 2, PageSize: 1})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 3 || page.TotalPages != 3 || len(page.Items) != 1 || page.Page != 2 {
		t.Fatalf("unexpected page metadata: %+v", page)
	}

	triggered, err := database.ListMonitorsPage(ctx, MonitorListOptions{PageSize: 20, Status: "triggered", AvailableModuleTypes: []string{"http"}})
	if err != nil {
		t.Fatal(err)
	}
	if triggered.Total != 1 || triggered.Items[0].ID != "monitor-1" {
		t.Fatalf("unexpected triggered monitors: %+v", triggered)
	}

	unavailable, err := database.ListMonitorsPage(ctx, MonitorListOptions{PageSize: 20, Status: "unavailable", AvailableModuleTypes: []string{"tcp"}})
	if err != nil {
		t.Fatal(err)
	}
	if unavailable.Total != 2 {
		t.Fatalf("unexpected unavailable monitors: %+v", unavailable)
	}

	search, err := database.ListMonitorsPage(ctx, MonitorListOptions{PageSize: 20, Search: "staging"})
	if err != nil {
		t.Fatal(err)
	}
	if search.Total != 1 || search.Items[0].ID != "monitor-3" {
		t.Fatalf("unexpected search result: %+v", search)
	}
}

func TestListRecordsPageFiltersAndPaginates(t *testing.T) {
	ctx := context.Background()
	database, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	now := time.Now().UTC()
	monitor := core.Monitor{ID: "monitor-1", Name: "API", ModuleType: "http", ModuleVersion: "1", Schedules: []string{"@hourly"}, Enabled: true, ModuleConfig: json.RawMessage(`{}`), ConditionConfig: json.RawMessage(`{}`), RuntimeState: json.RawMessage(`{}`), CreatedAt: now, UpdatedAt: now}
	if err := database.CreateMonitor(ctx, monitor); err != nil {
		t.Fatal(err)
	}
	for index, item := range []struct {
		success bool
		event   string
	}{
		{true, "none"},
		{false, "triggered"},
		{true, "recovered"},
	} {
		record := core.MonitorRecord{ID: "record-" + string(rune('1'+index)), MonitorID: monitor.ID, StartedAt: now.Add(time.Duration(index) * time.Minute), FinishedAt: now.Add(time.Duration(index)*time.Minute + time.Second), Success: item.success, DurationMS: int64(index + 1), ResultSchemaVersion: "1", Result: map[string]any{"body": "response-" + string(rune('1'+index))}, ResultHash: "hash-" + string(rune('1'+index)), ConditionState: "false", EventType: item.event, NotificationEvents: []core.RecordNotificationEvent{}, ErrorMessage: ""}
		if err := database.AddRecord(ctx, record); err != nil {
			t.Fatal(err)
		}
	}

	page, err := database.ListRecordsPage(ctx, monitor.ID, RecordListOptions{Page: 2, PageSize: 2})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 3 || page.TotalPages != 2 || len(page.Items) != 1 {
		t.Fatalf("unexpected record page: %+v", page)
	}

	failed, err := database.ListRecordsPage(ctx, monitor.ID, RecordListOptions{PageSize: 20, Status: "failed"})
	if err != nil {
		t.Fatal(err)
	}
	if failed.Total != 1 || failed.Items[0].EventType != "triggered" {
		t.Fatalf("unexpected failed records: %+v", failed)
	}
	trendEvents := []core.RecordNotificationEvent{{ID: "trend-event", Source: "status_trend", EventType: "trend_triggered", Deliveries: map[string]core.NotificationDelivery{}}}
	if err := database.UpdateRecordNotificationEvents(ctx, "record-1", trendEvents); err != nil {
		t.Fatal(err)
	}
	trends, err := database.ListRecordsPage(ctx, monitor.ID, RecordListOptions{PageSize: 20, EventType: "trend_triggered"})
	if err != nil || trends.Total != 1 || trends.Items[0].ID != "record-1" {
		t.Fatalf("unexpected trend records: %+v err=%v", trends, err)
	}

	search, err := database.ListRecordsPage(ctx, monitor.ID, RecordListOptions{PageSize: 20, Search: "response-3"})
	if err != nil {
		t.Fatal(err)
	}
	if search.Total != 1 || search.Items[0].EventType != "recovered" {
		t.Fatalf("unexpected record search: %+v", search)
	}
	deleted, err := database.DeleteMonitorRecords(ctx, monitor.ID)
	if err != nil || deleted != 3 {
		t.Fatalf("delete records deleted=%d err=%v", deleted, err)
	}
	empty, err := database.ListRecordsPage(ctx, monitor.ID, RecordListOptions{PageSize: 20})
	if err != nil || empty.Total != 0 {
		t.Fatalf("records after delete: %+v err=%v", empty, err)
	}
}

func TestBuiltInChannelAndInAppNotificationLifecycle(t *testing.T) {
	ctx := context.Background()
	database, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	channel, err := database.GetChannel(ctx, core.BuiltInNotificationChannelID)
	if err != nil {
		t.Fatal(err)
	}
	if !channel.BuiltIn || channel.NotifierType != "inapp" || !channel.Enabled {
		t.Fatalf("unexpected built-in channel: %+v", channel)
	}

	now := time.Now().UTC()
	for index := 0; index < 3; index++ {
		notification := core.InAppNotification{
			ID: "notification-" + string(rune('1'+index)), ChannelID: channel.ID, MonitorID: "monitor-1", RecordID: "record-1",
			EventType: "triggered", Title: "Alert", Content: "body", CreatedAt: now.Add(time.Duration(index) * time.Minute),
		}
		if err := database.CreateInAppNotification(ctx, notification); err != nil {
			t.Fatal(err)
		}
	}

	page, err := database.ListInAppNotificationsPage(ctx, NotificationListOptions{Page: 1, PageSize: 2})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 3 || page.TotalPages != 2 || len(page.Items) != 2 || page.Items[0].ID != "notification-3" {
		t.Fatalf("unexpected notification page: %+v", page)
	}
	count, err := database.CountUnreadInAppNotifications(ctx)
	if err != nil || count != 3 {
		t.Fatalf("unread count=%d err=%v", count, err)
	}
	read, err := database.MarkInAppNotificationRead(ctx, "notification-3")
	if err != nil || !read.Read || read.ReadAt == nil {
		t.Fatalf("mark read returned %+v err=%v", read, err)
	}
	updated, err := database.MarkAllInAppNotificationsRead(ctx)
	if err != nil || updated != 2 {
		t.Fatalf("mark all updated=%d err=%v", updated, err)
	}
	count, _ = database.CountUnreadInAppNotifications(ctx)
	if count != 0 {
		t.Fatalf("unread count after mark all=%d", count)
	}
	deleted, err := database.DeleteReadInAppNotifications(ctx)
	if err != nil || deleted != 3 {
		t.Fatalf("delete read notifications deleted=%d err=%v", deleted, err)
	}
	page, err = database.ListInAppNotificationsPage(ctx, NotificationListOptions{PageSize: 20})
	if err != nil || page.Total != 0 {
		t.Fatalf("notifications after delete: %+v err=%v", page, err)
	}
}

func TestUnifiedNotificationDeliveryPersistence(t *testing.T) {
	ctx := context.Background()
	database, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	createdAt := time.Now().UTC().Add(-time.Hour)
	for _, delivery := range []core.NotificationDeliveryRecord{
		{ID: "delivery-inapp", EventID: "event-1", Source: "status_trend", EventType: "trend_triggered", ChannelID: core.BuiltInNotificationChannelID, NotifierType: "inapp", Title: "Alert", Content: "Threshold reached", Status: "sent", Attempts: 1, CreatedAt: createdAt, UpdatedAt: createdAt},
		{ID: "delivery-webhook", EventID: "event-1", Source: "status_trend", EventType: "trend_triggered", ChannelID: "webhook-1", NotifierType: "webhook", Content: "Threshold reached", Payload: json.RawMessage(`{"value":12}`), Status: "error", Attempts: 3, Message: "timeout", CreatedAt: createdAt, UpdatedAt: createdAt},
	} {
		if err := database.CreateNotificationDelivery(ctx, delivery); err != nil {
			t.Fatal(err)
		}
	}
	count, err := database.orm.NewSelect().Model((*notificationDeliveryModel)(nil)).Count(ctx)
	if err != nil || count != 2 {
		t.Fatalf("delivery count=%d err=%v", count, err)
	}
	page, err := database.ListInAppNotificationsPage(ctx, NotificationListOptions{PageSize: 20})
	if err != nil || page.Total != 1 || page.Items[0].ID != "delivery-inapp" {
		t.Fatalf("in-app projection = %+v err=%v", page, err)
	}
	if err := database.UpdateNotificationDeliveryResult(ctx, "event-1", "webhook-1", core.NotificationDelivery{Status: "sent", Attempts: 4}); err != nil {
		t.Fatal(err)
	}
	var status string
	if err := database.orm.NewSelect().Model((*notificationDeliveryModel)(nil)).Column("status").Where("id = ?", "delivery-webhook").Scan(ctx, &status); err != nil || status != "sent" {
		t.Fatalf("updated delivery status=%q err=%v", status, err)
	}
}
