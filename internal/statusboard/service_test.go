package statusboard

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"meerkit/internal/core"
	"meerkit/internal/monitor"
	"meerkit/internal/store"
)

func TestEvaluateExecutionTransitionsTrendState(t *testing.T) {
	ctx := context.Background()
	database, err := store.OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	registry := monitor.NewRegistry()
	registry.Register(statusTestModule{})
	service := NewService(database, registry, NewHub())
	now := time.Now().UTC()
	monitorValue := core.Monitor{ID: "monitor", Name: "API", ModuleType: "status-test", ModuleVersion: "1", Schedules: []string{"@hourly"}, Enabled: true, ModuleConfig: json.RawMessage(`{}`), ConditionConfig: json.RawMessage(`{"rules":[]}`), RuntimeState: json.RawMessage(`{}`), CreatedAt: now, UpdatedAt: now}
	if err := database.CreateMonitor(ctx, monitorValue); err != nil {
		t.Fatal(err)
	}
	maximum := 100.0
	item := core.StatusBoardItem{ID: "item", Name: "Latency", MonitorID: monitorValue.ID, Enabled: true, Source: core.StatusItemSource{Kind: core.StatusSourceResultField, ResultSet: "response", Field: "duration", ValueType: core.StatusValueNumber}, Thresholds: []core.StatusThreshold{{Maximum: &maximum, Level: core.StatusLevelSuccess, Label: "正常"}, {Level: core.StatusLevelFailure, Label: "过高"}}, HistoryLimit: 60, TrendRules: []core.TrendRule{{ID: "rule", Name: "连续过高", Type: core.TrendConsecutive, Window: 1}}, RuntimeState: core.StatusItemRuntimeState{EvaluationStartedAt: now.Add(-time.Minute), Rules: map[string]core.TrendRuleState{}}, CreatedAt: now, UpdatedAt: now}
	if err := database.CreateStatusBoardItem(ctx, item); err != nil {
		t.Fatal(err)
	}
	failing := core.MonitorRecord{ID: "record-1", MonitorID: monitorValue.ID, StartedAt: now, FinishedAt: now, Result: map[string]any{"response": map[string]any{"duration": 150.0}}}
	triggered, err := service.EvaluateExecution(ctx, monitorValue, failing)
	if err != nil || len(triggered.Events) != 1 || triggered.Events[0].Event.EventType != "trend_triggered" || !triggered.ItemStates[item.ID].Rules["rule"].Active {
		t.Fatalf("triggered=%#v err=%v", triggered, err)
	}
	item.RuntimeState = triggered.ItemStates[item.ID]
	if err := database.UpdateStatusBoardItem(ctx, item); err != nil {
		t.Fatal(err)
	}
	healthy := core.MonitorRecord{ID: "record-2", MonitorID: monitorValue.ID, StartedAt: now.Add(time.Second), FinishedAt: now.Add(time.Second), Result: map[string]any{"response": map[string]any{"duration": 50.0}}}
	recovered, err := service.EvaluateExecution(ctx, monitorValue, healthy)
	if err != nil || len(recovered.Events) != 1 || recovered.Events[0].Event.EventType != "trend_recovered" || recovered.ItemStates[item.ID].Rules["rule"].Active {
		t.Fatalf("recovered=%#v err=%v", recovered, err)
	}
}

type statusTestModule struct{}

func (statusTestModule) Descriptor() core.ModuleDescriptor {
	return core.ModuleDescriptor{Type: "status-test", Version: "1", Name: "Status test", ResultSets: []core.ResultSetDescriptor{{Key: "response", Label: "响应", Fields: []core.ResultFieldDescriptor{{Name: "duration", Label: "耗时", Type: "number"}}}}}
}
func (statusTestModule) ValidateConfig(json.RawMessage) error { return nil }
func (statusTestModule) Execute(context.Context, json.RawMessage) (core.Observation, error) {
	return core.Observation{}, nil
}
