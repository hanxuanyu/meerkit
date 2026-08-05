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
