package statusboard

import (
	"testing"
	"time"

	"meerkit/internal/core"
)

func TestBuildSamplesSupportsConditionRuleAndNumericScaling(t *testing.T) {
	now := time.Now().UTC()
	conditionItem := core.StatusBoardItem{Source: core.StatusItemSource{Kind: core.StatusSourceConditionRule, RuleID: "rule-1", ValueType: core.StatusValueBoolean}}
	conditionRecords := []core.MonitorRecord{
		{ID: "one", StartedAt: now, Result: map[string]any{"summary": map[string]any{"condition_details": []core.RuleResult{{ID: "rule-1", State: "true"}}}}},
		{ID: "two", StartedAt: now.Add(time.Second), Result: map[string]any{"summary": map[string]any{"condition_details": []any{map[string]any{"id": "rule-1", "state": "false"}}}}},
	}
	samples := BuildSamples(conditionItem, conditionRecords)
	if samples[0].State != core.StatusLevelSuccess || samples[1].State != core.StatusLevelFailure {
		t.Fatalf("unexpected condition samples: %#v", samples)
	}

	maximum := 100.0
	numericItem := core.StatusBoardItem{Source: core.StatusItemSource{Kind: core.StatusSourceResultField, ResultSet: "response", Field: "duration", ValueType: core.StatusValueNumber}, Thresholds: []core.StatusThreshold{{Maximum: &maximum, Level: core.StatusLevelSuccess, Label: "正常"}, {Level: core.StatusLevelFailure, Label: "过高"}}}
	numericRecords := []core.MonitorRecord{
		{ID: "one", StartedAt: now, Result: map[string]any{"response": map[string]any{"duration": 50.0}}},
		{ID: "two", StartedAt: now.Add(time.Second), Result: map[string]any{"response": map[string]any{"duration": 150.0}}},
	}
	samples = BuildSamples(numericItem, numericRecords)
	if samples[0].Level != core.StatusLevelSuccess || samples[1].Level != core.StatusLevelFailure || samples[0].Height != 10 || samples[1].Height != 100 {
		t.Fatalf("unexpected numeric samples: %#v", samples)
	}
}

func TestEvaluateTrendRuleTypesAndUnknown(t *testing.T) {
	numeric := func(values ...float64) []core.StatusSample {
		result := make([]core.StatusSample, len(values))
		for index, value := range values {
			level := core.StatusLevelSuccess
			if value >= 3 {
				level = core.StatusLevelFailure
			}
			result[index] = core.StatusSample{State: level, Numeric: core.Float64(value)}
		}
		return result
	}
	tests := []struct {
		name string
		rule core.TrendRule
		want string
	}{
		{"consecutive", core.TrendRule{Type: core.TrendConsecutive, Window: 2}, "true"},
		{"count", core.TrendRule{Type: core.TrendCount, Window: 4, Minimum: 2}, "true"},
		{"average", core.TrendRule{Type: core.TrendAverage, Window: 4, Operator: "gt", Threshold: 2}, "true"},
		{"absolute delta", core.TrendRule{Type: core.TrendDelta, Window: 4, Operator: "gte", Threshold: 3, DeltaMode: "absolute"}, "true"},
		{"percent delta", core.TrendRule{Type: core.TrendDelta, Window: 4, Operator: "gte", Threshold: 300, DeltaMode: "percent"}, "true"},
		{"slope", core.TrendRule{Type: core.TrendSlope, Window: 4, Operator: "gt", Threshold: .9}, "true"},
	}
	values := numeric(1, 2, 3, 4)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state, _ := EvaluateTrend(test.rule, values)
			if state != test.want {
				t.Fatalf("state=%s want=%s", state, test.want)
			}
		})
	}
	unknown := append([]core.StatusSample(nil), values...)
	unknown[2].State = core.StatusLevelUnknown
	if state, _ := EvaluateTrend(core.TrendRule{Type: core.TrendCount, Window: 4, Minimum: 1}, unknown); state != "unknown" {
		t.Fatalf("unknown sample returned %s", state)
	}
	if state, _ := EvaluateTrend(core.TrendRule{Type: core.TrendDelta, Window: 2, Operator: "gt", DeltaMode: "percent"}, numeric(0, 1)); state != "unknown" {
		t.Fatalf("zero baseline returned %s", state)
	}
}
