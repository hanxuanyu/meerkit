package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"meerkit/internal/app"
	"meerkit/internal/core"
	"meerkit/internal/monitor"
	"meerkit/internal/store"
)

func TestPreviewSchedule(t *testing.T) {
	server := &APIServer{config: app.DefaultConfig()}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/schedules/preview", strings.NewReader(`{"expression":"*/5 * * * *"}`))
	response := httptest.NewRecorder()
	server.previewSchedule(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("preview status = %d, body = %s", response.Code, response.Body.String())
	}
	var payload struct {
		Description string   `json:"description"`
		NextRuns    []string `json:"next_runs"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Description != "每 5 分钟执行一次" || len(payload.NextRuns) != 3 {
		t.Fatalf("unexpected preview: %#v", payload)
	}

	request = httptest.NewRequest(http.MethodPost, "/api/v1/schedules/preview", strings.NewReader(`{"expression":"not-a-cron"}`))
	response = httptest.NewRecorder()
	server.previewSchedule(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("invalid preview status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func TestMonitorListPageAddsNextRun(t *testing.T) {
	registry := monitor.NewRegistry()
	registry.Register(apiTestModule{})
	page := store.PageResult[core.Monitor]{
		Items: []core.Monitor{
			{ID: "monitor-1", ModuleType: "test", ModuleVersion: "1", Enabled: true, Schedules: []string{"*/5 * * * *"}},
			{ID: "monitor-2", ModuleType: "test", ModuleVersion: "1", Enabled: false, Schedules: []string{"*/5 * * * *"}},
			{ID: "monitor-3", ModuleType: "missing", ModuleVersion: "1", Enabled: true, Schedules: []string{"*/5 * * * *"}},
		},
		Page:       2,
		PageSize:   20,
		Total:      21,
		TotalPages: 2,
	}
	result := monitorListPage(page, "Asia/Shanghai", registry)
	if len(result.Items) != 3 || result.Items[0].NextRunAt == nil || !result.Items[0].ModuleAvailable {
		t.Fatalf("next run was not added: %#v", result.Items)
	}
	if result.Items[1].NextRunAt != nil {
		t.Fatalf("disabled monitor should not have a next run: %#v", result.Items[1])
	}
	if result.Items[2].NextRunAt != nil || result.Items[2].ModuleAvailable || result.Items[2].PauseReason != "module_unavailable" {
		t.Fatalf("unavailable monitor should be paused: %#v", result.Items[2])
	}
	if result.Page != page.Page || result.PageSize != page.PageSize || result.Total != page.Total || result.TotalPages != page.TotalPages {
		t.Fatalf("pagination metadata changed: %#v", result)
	}
}

func TestNormalizedConditionConfigAssignsStableRuleIDs(t *testing.T) {
	raw := normalizedConditionConfig(core.ConditionConfig{Logic: "ALL", Rules: []core.ConditionRule{{Field: "summary.success", Operator: "is_true"}, {ID: "existing", Field: "summary.duration_ms", Operator: "gt", Value: 100}}})
	var config core.ConditionConfig
	if err := json.Unmarshal(raw, &config); err != nil {
		t.Fatal(err)
	}
	if config.Rules[0].ID == "" || config.Rules[1].ID != "existing" || config.Rules[0].ID == config.Rules[1].ID {
		t.Fatalf("unexpected rule IDs: %#v", config.Rules)
	}
}

func TestUpdateMonitorCanDisableUnavailableModule(t *testing.T) {
	ctx := context.Background()
	database, err := store.OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	now := time.Now().UTC()
	value := core.Monitor{ID: "unavailable-monitor", Name: "Unavailable", ModuleType: "http", ModuleVersion: "2", ModuleConfigVersion: "1", Schedules: []string{"*/5 * * * *"}, Enabled: true, ModuleConfig: json.RawMessage(`{"url":"https://example.test"}`), ConditionConfig: json.RawMessage(`{"logic":"ALL","rules":[]}`), RuntimeState: json.RawMessage(`{}`), CreatedAt: now, UpdatedAt: now}
	if err := database.CreateMonitor(ctx, value); err != nil {
		t.Fatal(err)
	}

	server := &APIServer{store: database, modules: monitor.NewRegistry(), config: app.DefaultConfig()}
	request := httptest.NewRequest(http.MethodPatch, "/api/v1/monitors/"+value.ID, strings.NewReader(`{"enabled":false}`))
	response := httptest.NewRecorder()
	server.updateMonitor(response, request, value)
	if response.Code != http.StatusOK {
		t.Fatalf("disable unavailable monitor status = %d, body = %s", response.Code, response.Body.String())
	}
	stored, err := database.GetMonitor(ctx, value.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Enabled {
		t.Fatal("unavailable monitor was not disabled")
	}
}

func TestStatusBoardEvaluationConfigIgnoresDisplayColors(t *testing.T) {
	maximum := 100.0
	item := core.StatusBoardItem{
		Source:     core.StatusItemSource{ValueType: core.StatusValueText, DefaultLevel: core.StatusLevelSuccess, DefaultLabel: "正常", DefaultColor: "green", ValueMappings: []core.StatusValueMapping{{Value: "down", Level: core.StatusLevelFailure, Label: "异常", Color: "red"}}},
		Thresholds: []core.StatusThreshold{{Maximum: &maximum, Level: core.StatusLevelSuccess, Label: "正常", Color: "green"}},
	}
	changed := item
	changed.Source.ValueMappings = append([]core.StatusValueMapping(nil), item.Source.ValueMappings...)
	changed.Thresholds = append([]core.StatusThreshold(nil), item.Thresholds...)
	changed.Source.DefaultColor, changed.Source.DefaultLabel = "red", "自定义"
	changed.Source.ValueMappings[0].Color, changed.Source.ValueMappings[0].Label = "green", "自定义"
	changed.Thresholds[0].Color, changed.Thresholds[0].Label = "orange", "自定义"
	if !reflect.DeepEqual(statusBoardEvaluationConfig(item), statusBoardEvaluationConfig(changed)) {
		t.Fatal("display-only color and label changes should not reset trend state")
	}
	changed.Source.ValueMappings[0].Level = core.StatusLevelSuccess
	if reflect.DeepEqual(statusBoardEvaluationConfig(item), statusBoardEvaluationConfig(changed)) {
		t.Fatal("semantic level changes must reset trend state")
	}
}

type apiTestModule struct{}

func (apiTestModule) Descriptor() core.ModuleDescriptor {
	return core.ModuleDescriptor{Type: "test", Version: "1", Name: "Test"}
}
func (apiTestModule) ValidateConfig(json.RawMessage) error { return nil }
func (apiTestModule) Execute(context.Context, json.RawMessage) (core.Observation, error) {
	return core.Observation{Success: true}, nil
}
