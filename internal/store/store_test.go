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

	triggered, err := database.ListMonitorsPage(ctx, MonitorListOptions{PageSize: 20, Status: "triggered"})
	if err != nil {
		t.Fatal(err)
	}
	if triggered.Total != 1 || triggered.Items[0].ID != "monitor-1" {
		t.Fatalf("unexpected triggered monitors: %+v", triggered)
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
		record := core.MonitorRecord{ID: "record-" + string(rune('1'+index)), MonitorID: monitor.ID, StartedAt: now.Add(time.Duration(index) * time.Minute), FinishedAt: now.Add(time.Duration(index)*time.Minute + time.Second), Success: item.success, DurationMS: int64(index + 1), ResultSchemaVersion: "1", Result: map[string]any{"body": "response-" + string(rune('1'+index))}, ResultHash: "hash-" + string(rune('1'+index)), ConditionState: "false", EventType: item.event, NotificationResult: map[string]any{}, ErrorMessage: ""}
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

	search, err := database.ListRecordsPage(ctx, monitor.ID, RecordListOptions{PageSize: 20, Search: "response-3"})
	if err != nil {
		t.Fatal(err)
	}
	if search.Total != 1 || search.Items[0].EventType != "recovered" {
		t.Fatalf("unexpected record search: %+v", search)
	}
}
