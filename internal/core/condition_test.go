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
