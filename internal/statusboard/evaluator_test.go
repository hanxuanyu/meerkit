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

func TestBuildSamplesSupportsExactValueColorsAndTextDefault(t *testing.T) {
	now := time.Now().UTC()
	maximum := 100.0
	numericItem := core.StatusBoardItem{
		Source:     core.StatusItemSource{Kind: core.StatusSourceResultField, ResultSet: "response", Field: "code", ValueType: core.StatusValueNumber, ValueMappings: []core.StatusValueMapping{{Value: "0", Level: core.StatusLevelFailure, Label: "零值异常", Color: "green"}}},
		Thresholds: []core.StatusThreshold{{Maximum: &maximum, Level: core.StatusLevelSuccess, Label: "正常"}, {Level: core.StatusLevelWarning, Label: "偏高"}},
	}
	records := []core.MonitorRecord{{ID: "zero", StartedAt: now, Result: map[string]any{"response": map[string]any{"code": 0}}}, {ID: "other", StartedAt: now, Result: map[string]any{"response": map[string]any{"code": 20}}}}
	samples := BuildSamples(numericItem, records)
	if samples[0].Level != core.StatusLevelFailure || samples[0].Color != "green" || samples[0].Label != "零值异常" || samples[1].Level != core.StatusLevelSuccess {
		t.Fatalf("unexpected numeric exact-value samples: %#v", samples)
	}

	textItem := core.StatusBoardItem{Source: core.StatusItemSource{Kind: core.StatusSourceResultField, ResultSet: "response", Field: "state", ValueType: core.StatusValueText, DefaultLevel: core.StatusLevelSuccess, DefaultLabel: "其他状态", ValueMappings: []core.StatusValueMapping{{Value: "DOWN", Level: core.StatusLevelFailure, Label: "离线"}, {Value: "", Level: core.StatusLevelWarning, Label: "无状态"}}}}
	records = []core.MonitorRecord{{ID: "down", StartedAt: now, Result: map[string]any{"response": map[string]any{"state": "DOWN"}}}, {ID: "empty", StartedAt: now, Result: map[string]any{"response": map[string]any{"state": ""}}}, {ID: "other", StartedAt: now, Result: map[string]any{"response": map[string]any{"state": "UP"}}}}
	samples = BuildSamples(textItem, records)
	if samples[0].Level != core.StatusLevelFailure || samples[1].Level != core.StatusLevelWarning || samples[1].Display != "空" || samples[2].Level != core.StatusLevelSuccess || samples[2].Label != "其他状态" {
		t.Fatalf("unexpected text exact-value samples: %#v", samples)
	}
}

func TestBuildSamplesSupportsOrderedTextRegexMappings(t *testing.T) {
	now := time.Now().UTC()
	item := core.StatusBoardItem{Source: core.StatusItemSource{
		Kind:         core.StatusSourceResultField,
		ResultSet:    "response",
		Field:        "state",
		ValueType:    core.StatusValueText,
		DefaultLevel: core.StatusLevelSuccess,
		DefaultLabel: "其他状态",
		ValueMappings: []core.StatusValueMapping{
			{Value: `^ERROR_`, MatchType: core.StatusMatchRegex, Level: core.StatusLevelWarning, Label: "错误前缀"},
			{Value: "ERROR_TIMEOUT", Level: core.StatusLevelFailure, Label: "超时"},
			{Value: `^DOWN$`, MatchType: core.StatusMatchRegex, Level: core.StatusLevelFailure, Label: "离线"},
		},
	}}
	records := []core.MonitorRecord{
		{ID: "ordered", StartedAt: now, Result: map[string]any{"response": map[string]any{"state": "ERROR_TIMEOUT"}}},
		{ID: "regex", StartedAt: now, Result: map[string]any{"response": map[string]any{"state": "DOWN"}}},
		{ID: "default", StartedAt: now, Result: map[string]any{"response": map[string]any{"state": "UP"}}},
	}
	samples := BuildSamples(item, records)
	if samples[0].Level != core.StatusLevelWarning || samples[0].Label != "错误前缀" {
		t.Fatalf("first matching rule did not win: %#v", samples[0])
	}
	if samples[1].Level != core.StatusLevelFailure || samples[1].Label != "离线" {
		t.Fatalf("regex rule did not match: %#v", samples[1])
	}
	if samples[2].Level != core.StatusLevelSuccess || samples[2].Label != "其他状态" {
		t.Fatalf("unmatched text did not use default: %#v", samples[2])
	}
}

func TestValidateValueMappingsRejectsInvalidRegexAndNumericRegex(t *testing.T) {
	invalidRegex := core.StatusItemSource{ValueType: core.StatusValueText, ValueMappings: []core.StatusValueMapping{{Value: "[", MatchType: core.StatusMatchRegex, Level: core.StatusLevelFailure}}}
	if err := normalizeAndValidateValueMappings(&invalidRegex); err == nil {
		t.Fatal("invalid text regex was accepted")
	}
	numericRegex := core.StatusItemSource{ValueType: core.StatusValueNumber, ValueMappings: []core.StatusValueMapping{{Value: "1", MatchType: core.StatusMatchRegex, Level: core.StatusLevelFailure}}}
	if err := normalizeAndValidateValueMappings(&numericRegex); err == nil {
		t.Fatal("numeric regex mapping was accepted")
	}
}

func TestStatusColorPresetsAreIndependentFromSemanticLevel(t *testing.T) {
	maximum := 10.0
	thresholds := []core.StatusThreshold{{Maximum: &maximum, Level: core.StatusLevelFailure, Label: "异常", Color: "green"}, {Level: core.StatusLevelSuccess, Label: "正常", Color: "red"}}
	if err := validateThresholds(thresholds); err != nil {
		t.Fatalf("independent threshold colors should be valid: %v", err)
	}
	source := core.StatusItemSource{ValueType: core.StatusValueText, DefaultLevel: core.StatusLevelFailure, DefaultLabel: "异常", DefaultColor: "green", ValueMappings: []core.StatusValueMapping{{Value: "ready", Level: core.StatusLevelSuccess, Label: "正常", Color: "red"}}}
	if err := normalizeAndValidateValueMappings(&source); err != nil {
		t.Fatalf("independent exact-value colors should be valid: %v", err)
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
