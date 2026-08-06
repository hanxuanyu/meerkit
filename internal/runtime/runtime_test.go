package runtime

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"meerkit/internal/app"
	"meerkit/internal/core"
	"meerkit/internal/monitor"
	"meerkit/internal/store"
)

func TestRunPersistsComposedExecutionSummary(t *testing.T) {
	ctx := context.Background()
	database, err := store.OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	now := time.Now().UTC()
	monitorConfig := core.Monitor{
		ID: "summary-monitor", Name: "Summary monitor", ModuleType: "summary-test", ModuleVersion: "1",
		Schedules: []string{"@hourly"}, Enabled: true, ModuleConfig: json.RawMessage(`{}`),
		ConditionConfig: json.RawMessage(`{"logic":"ALL","rules":[{"field":"value","operator":"equals","value":"ready"}]}`),
		RuntimeState:    json.RawMessage(`{}`), CreatedAt: now, UpdatedAt: now,
	}
	if err := database.CreateMonitor(ctx, monitorConfig); err != nil {
		t.Fatal(err)
	}

	modules := monitor.NewRegistry()
	modules.Register(summaryTestModule{})
	record, err := NewRunner(database, modules, nil, nil).Run(ctx, monitorConfig.ID)
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if record.EventType != "triggered" {
		t.Fatalf("event type = %q, want triggered", record.EventType)
	}
	summarySet, ok := record.Result["summary"].(map[string]any)
	if !ok {
		t.Fatalf("summary result set has unexpected type: %#v", record.Result["summary"])
	}
	summary, ok := summarySet["summary"].(string)
	if !ok {
		t.Fatalf("summary field has unexpected type: %#v", summarySet["summary"])
	}
	for _, expected := range []string{"模块结果：ready", "事件类型：已触发", "条件状态：满足（ALL 逻辑，满足 1/1 条）", "条件详情：", "状态值（value）", "实际值：ready"} {
		if !strings.Contains(summary, expected) {
			t.Fatalf("persisted summary does not contain %q:\n%s", expected, summary)
		}
	}

	storedMonitor, err := database.GetMonitor(ctx, monitorConfig.ID)
	if err != nil {
		t.Fatal(err)
	}
	var state core.RuntimeState
	if err := json.Unmarshal(storedMonitor.RuntimeState, &state); err != nil {
		t.Fatal(err)
	}
	if state.LastSummary != summary {
		t.Fatalf("last summary does not match persisted record: %q != %q", state.LastSummary, summary)
	}
}

type summaryTestModule struct{}

func (summaryTestModule) Descriptor() core.ModuleDescriptor {
	return core.ModuleDescriptor{Type: "summary-test", Version: "1", Name: "Summary test", ResultSchema: map[string]any{"type": "object"}, Fields: []core.FieldDescriptor{{Name: "value", Label: "状态值"}}}
}

func (summaryTestModule) ValidateConfig(json.RawMessage) error { return nil }

func (summaryTestModule) Execute(context.Context, json.RawMessage) (core.Observation, error) {
	return core.Observation{Success: true, SchemaVersion: "1", Result: map[string]any{"value": "ready"}, Summary: "模块结果：ready"}, nil
}

func TestSchedulerPausesMonitorWhileModuleIsUnavailable(t *testing.T) {
	ctx := context.Background()
	database, err := store.OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	now := time.Date(2026, time.August, 6, 23, 50, 0, 0, time.UTC)
	value := core.Monitor{ID: "paused-monitor", Name: "Paused monitor", ModuleType: "summary-test", ModuleVersion: "1", Schedules: []string{"* * * * * *"}, Enabled: true, ModuleConfig: json.RawMessage(`{}`), ConditionConfig: json.RawMessage(`{}`), RuntimeState: json.RawMessage(`{}`), CreatedAt: now, UpdatedAt: now}
	if err := database.CreateMonitor(ctx, value); err != nil {
		t.Fatal(err)
	}

	modules := monitor.NewRegistry()
	scheduler := NewScheduler(NewRunner(database, modules, nil, nil), database, app.DefaultConfig(), nil)
	scheduler.syncAndRun(ctx, now)
	if len(scheduler.tasks) != 0 || scheduler.pausedMonitors[value.ID] == "" {
		t.Fatalf("unavailable monitor was not paused: tasks=%#v paused=%#v", scheduler.tasks, scheduler.pausedMonitors)
	}

	modules.Register(summaryTestModule{})
	scheduler.syncAndRun(ctx, now.Add(time.Second))
	if scheduler.tasks[value.ID] == nil {
		t.Fatalf("available monitor did not resume: %#v", scheduler.tasks)
	}
	if _, paused := scheduler.pausedMonitors[value.ID]; paused {
		t.Fatalf("resumed monitor is still marked paused: %#v", scheduler.pausedMonitors)
	}
}

func TestValidateSchedules(t *testing.T) {
	if err := ValidateSchedules([]string{"*/5 * * * *", "@hourly"}, "Asia/Shanghai"); err != nil {
		t.Fatalf("ValidateSchedules: %v", err)
	}
	if err := ValidateSchedules([]string{""}, "Asia/Shanghai"); err == nil {
		t.Fatal("expected empty cron expression to fail")
	}
}

func TestNextScheduleTimesMergesExpressions(t *testing.T) {
	times, err := NextScheduleTimes([]string{"0 * * * *", "30 * * * *"}, "Asia/Shanghai", 5)
	if err != nil {
		t.Fatalf("NextScheduleTimes: %v", err)
	}
	if len(times) != 5 {
		t.Fatalf("got %d next times, want 5", len(times))
	}
	for index := 1; index < len(times); index++ {
		if times[index].Before(times[index-1]) {
			t.Fatalf("times are not sorted: %v", times)
		}
	}
	if !strings.Contains(times[0].Location().String(), "Asia/Shanghai") {
		t.Fatalf("unexpected timezone: %s", times[0].Location())
	}
	if times[0].Before(time.Now().In(times[0].Location())) {
		t.Fatalf("next time is in the past: %v", times[0])
	}
}

func TestNextScheduleTimesDeduplicatesOverlaps(t *testing.T) {
	times, err := NextScheduleTimes([]string{"*/5 * * * *", "*/5 * * * *"}, "UTC", 4)
	if err != nil {
		t.Fatalf("NextScheduleTimes: %v", err)
	}
	if len(times) != 4 {
		t.Fatalf("got %d next times, want 4", len(times))
	}
	for index := 1; index < len(times); index++ {
		if !times[index].After(times[index-1]) {
			t.Fatalf("overlapping times were not deduplicated: %v", times)
		}
	}
}

func TestAdvanceScheduleTaskConsumesOverlappingDueTimeOnce(t *testing.T) {
	dueAt := time.Date(2026, time.August, 5, 10, 0, 0, 0, time.UTC)
	task := &scheduleTask{expressions: []string{"0 * * * *", "0 */2 * * *"}, next: dueAt}
	due, err := advanceScheduleTask(task, "UTC", dueAt)
	if err != nil || !due {
		t.Fatalf("first advance = (%v, %v), want due", due, err)
	}
	if expected := dueAt.Add(time.Hour); !task.next.Equal(expected) {
		t.Fatalf("next = %v, want %v", task.next, expected)
	}
	due, err = advanceScheduleTask(task, "UTC", dueAt)
	if err != nil || due {
		t.Fatalf("second advance = (%v, %v), want not due", due, err)
	}
}

func TestDescribeSchedule(t *testing.T) {
	tests := map[string]string{
		"*/5 * * * *":  "每 5 分钟执行一次",
		"15 * * * *":   "每小时第 15 分钟执行",
		"0 9 * * 1-5":  "每个工作日 09:00 执行",
		"0 0 9 * * *":  "每天 09:00 执行",
		"@hourly":      "每小时执行一次",
		"10 8 * * 1,3": "按自定义 Cron 计划执行",
	}
	for expression, expected := range tests {
		if actual := DescribeSchedule(expression); actual != expected {
			t.Errorf("DescribeSchedule(%q) = %q, want %q", expression, actual, expected)
		}
	}
}
