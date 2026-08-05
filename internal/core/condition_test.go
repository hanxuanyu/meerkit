package core

import "testing"

func TestEvaluateConditionsChangedBaselineAndTransition(t *testing.T) {
	config := ConditionConfig{Logic: "ALL", Rules: []ConditionRule{{Field: "body_text", Operator: "changed"}}}
	first := EvaluateConditions(config, map[string]any{"body_text": "one"}, nil)
	if first.State != "false" {
		t.Fatalf("first observation should establish baseline, got %s", first.State)
	}
	second := EvaluateConditions(config, map[string]any{"body_text": "two"}, map[string]any{"body_text": "one"})
	if second.State != "true" {
		t.Fatalf("changed observation should be true, got %s", second.State)
	}
	unchanged := EvaluateConditions(config, map[string]any{"body_text": "one"}, map[string]any{"body_text": "one"})
	if unchanged.State != "false" {
		t.Fatalf("unchanged observation should be false, got %s", unchanged.State)
	}
}

func TestEvaluateConditionsAnyAndUnknown(t *testing.T) {
	config := ConditionConfig{Logic: "ANY", Rules: []ConditionRule{{Field: "missing", Operator: "equals", Value: "x"}, {Field: "code", Operator: "equals", Value: 200}}}
	result := EvaluateConditions(config, map[string]any{"code": float64(200)}, nil)
	if result.State != "true" {
		t.Fatalf("ANY should short circuit on a true rule, got %s", result.State)
	}
	unknown := EvaluateConditions(ConditionConfig{Logic: "ALL", Rules: []ConditionRule{{Field: "missing", Operator: "equals", Value: "x"}}}, map[string]any{}, nil)
	if unknown.State != "unknown" {
		t.Fatalf("missing values should be unknown, got %s", unknown.State)
	}
}

func TestEvaluateConditionsIncludesExpectedAndActualValues(t *testing.T) {
	result := EvaluateConditions(
		ConditionConfig{Logic: "ALL", Rules: []ConditionRule{{Field: "duration_ms", Operator: "gt", Value: 100}}},
		map[string]any{"duration_ms": 125},
		nil,
	)
	if len(result.Details) != 1 {
		t.Fatalf("got %d condition details, want 1", len(result.Details))
	}
	if result.Details[0].Expected != 100 || result.Details[0].Actual != 125 {
		t.Fatalf("unexpected condition values: %#v", result.Details[0])
	}
}

func TestEvaluateConditionsComparesCurrentAndPreviousFields(t *testing.T) {
	config := ConditionConfig{Logic: "ALL", Rules: []ConditionRule{{
		Field: "response.status_code", Source: "previous", Operator: "lt",
		ValueSource: "current", ValueField: "response.status_code",
	}}}
	result := EvaluateConditions(config,
		map[string]any{"response": map[string]any{"status_code": float64(201)}},
		map[string]any{"response": map[string]any{"status_code": float64(200)}},
	)
	if result.State != "true" {
		t.Fatalf("previous value should compare with current value, got %s: %#v", result.State, result.Details)
	}
	if result.Details[0].Actual != float64(200) || result.Details[0].Expected != float64(201) {
		t.Fatalf("unexpected comparison values: %#v", result.Details[0])
	}
}
