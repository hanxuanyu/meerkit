package runtime

import (
	"strings"
	"testing"
	"time"
)

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
