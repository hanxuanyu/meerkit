package store

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"meerkit/internal/core"
)

func TestStatusBoardItemAndNotificationEventPersistence(t *testing.T) {
	ctx := context.Background()
	database, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	now := time.Now().UTC()
	monitor := core.Monitor{ID: "monitor", Name: "API", ModuleType: "http", ModuleVersion: "1", Schedules: []string{"@hourly"}, Enabled: true, ModuleConfig: json.RawMessage(`{}`), ConditionConfig: json.RawMessage(`{"rules":[]}`), RuntimeState: json.RawMessage(`{}`), CreatedAt: now, UpdatedAt: now}
	if err := database.CreateMonitor(ctx, monitor); err != nil {
		t.Fatal(err)
	}
	item := core.StatusBoardItem{ID: "item", Name: "Latency", MonitorID: monitor.ID, Enabled: true, Source: core.StatusItemSource{Kind: core.StatusSourceResultField, ResultSet: "response", Field: "duration", ValueType: core.StatusValueNumber}, HistoryLimit: 60, RuntimeState: core.StatusItemRuntimeState{EvaluationStartedAt: now, Rules: map[string]core.TrendRuleState{}}, CreatedAt: now, UpdatedAt: now}
	if err := database.CreateStatusBoardItem(ctx, item); err != nil {
		t.Fatal(err)
	}
	loaded, err := database.GetStatusBoardItem(ctx, item.ID)
	if err != nil || loaded.Source.Field != "duration" || loaded.HistoryLimit != 60 {
		t.Fatalf("loaded=%#v err=%v", loaded, err)
	}
	record := core.MonitorRecord{ID: "record", MonitorID: monitor.ID, ModuleType: "http", ModuleVersion: "1", StartedAt: now, FinishedAt: now, Success: true, ResultSchemaVersion: "1", Result: map[string]any{}, ResultHash: "hash", ConditionState: "false", EventType: "none", NotificationEvents: []core.RecordNotificationEvent{{ID: "event", Source: "status_trend", EventType: "trend_triggered", Deliveries: map[string]core.NotificationDelivery{"channel": {Status: "pending"}}}}}
	state := core.RuntimeState{LastRecordID: record.ID}
	itemState := loaded.RuntimeState
	itemState.Rules["rule"] = core.TrendRuleState{Active: true, LastRecordID: record.ID}
	if err := database.CommitMonitorExecution(ctx, record, monitor.ID, state, map[string]core.StatusItemRuntimeState{item.ID: itemState}); err != nil {
		t.Fatal(err)
	}
	stored, err := database.GetRecord(ctx, monitor.ID, record.ID)
	if err != nil || len(stored.NotificationEvents) != 1 || stored.NotificationEvents[0].Deliveries["channel"].Status != "pending" {
		t.Fatalf("stored=%#v err=%v", stored, err)
	}
	stored.NotificationEvents[0].Deliveries["channel"] = core.NotificationDelivery{Status: "sent", Attempts: 1}
	if err := database.UpdateRecordNotificationEvents(ctx, record.ID, stored.NotificationEvents); err != nil {
		t.Fatal(err)
	}
	stored, _ = database.GetRecord(ctx, monitor.ID, record.ID)
	if stored.NotificationEvents[0].Deliveries["channel"].Status != "sent" {
		t.Fatalf("events=%#v", stored.NotificationEvents)
	}
	rollbackRecord := record
	rollbackRecord.ID = "rollback-record"
	if err := database.CommitMonitorExecution(ctx, rollbackRecord, monitor.ID, state, map[string]core.StatusItemRuntimeState{"missing-item": itemState}); err == nil {
		t.Fatal("expected missing status item to roll back execution")
	}
	if _, err := database.GetRecord(ctx, monitor.ID, rollbackRecord.ID); err == nil {
		t.Fatal("record remained after transaction rollback")
	}
}
