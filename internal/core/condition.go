package core

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

type ConditionConfig struct {
	Logic              string          `json:"logic"`
	Rules              []ConditionRule `json:"rules"`
	NotificationPolicy string          `json:"notification_policy,omitempty"`
}

const (
	NotificationPolicyOnce  = "once"
	NotificationPolicyEvery = "every"
)

func NormalizeNotificationPolicy(policy string) string {
	if policy == NotificationPolicyEvery {
		return NotificationPolicyEvery
	}
	return NotificationPolicyOnce
}

type ConditionRule struct {
	Field       string `json:"field"`
	Source      string `json:"source,omitempty"`
	Path        string `json:"path,omitempty"`
	Operator    string `json:"operator"`
	Value       any    `json:"value,omitempty"`
	ValueSource string `json:"value_source,omitempty"`
	ValueField  string `json:"value_field,omitempty"`
	ValuePath   string `json:"value_path,omitempty"`
}

type RuleResult struct {
	Field       string `json:"field"`
	Source      string `json:"source,omitempty"`
	Path        string `json:"path,omitempty"`
	Operator    string `json:"operator"`
	State       string `json:"state"`
	Expected    any    `json:"expected,omitempty"`
	Actual      any    `json:"actual,omitempty"`
	ValueSource string `json:"value_source,omitempty"`
	ValueField  string `json:"value_field,omitempty"`
	Message     string `json:"message,omitempty"`
}

type Evaluation struct {
	State   string       `json:"state"`
	Details []RuleResult `json:"details"`
}

func EvaluateConditions(config ConditionConfig, current, previous map[string]any) Evaluation {
	if len(config.Rules) == 0 {
		return Evaluation{State: "false", Details: []RuleResult{}}
	}
	logic := strings.ToUpper(config.Logic)
	if logic != "ANY" {
		logic = "ALL"
	}
	results := make([]RuleResult, 0, len(config.Rules))
	unknown := false
	trueCount := 0
	falseCount := 0
	for _, rule := range config.Rules {
		result := evaluateRule(rule, current, previous)
		results = append(results, result)
		switch result.State {
		case "true":
			trueCount++
		case "false":
			falseCount++
		default:
			unknown = true
		}
	}
	if logic == "ANY" {
		if trueCount > 0 {
			return Evaluation{State: "true", Details: results}
		}
		if unknown && falseCount == 0 {
			return Evaluation{State: "unknown", Details: results}
		}
		return Evaluation{State: "false", Details: results}
	}
	if falseCount > 0 {
		return Evaluation{State: "false", Details: results}
	}
	if unknown {
		return Evaluation{State: "unknown", Details: results}
	}
	return Evaluation{State: "true", Details: results}
}

func evaluateRule(rule ConditionRule, current, previous map[string]any) RuleResult {
	source := normalizeSource(rule.Source)
	field := normalizeField(rule.Field)
	valueSource := normalizeValueSource(rule.ValueSource)
	result := RuleResult{Field: field, Source: source, Path: rule.Path, Operator: rule.Operator, ValueSource: valueSource, ValueField: normalizeField(rule.ValueField)}
	value, ok := lookupSourceValue(source, current, previous, field, rule.Path)
	if ok {
		result.Actual = value
	}
	if rule.Operator == "exists" {
		result.State = boolState(ok)
		return result
	}
	if rule.Operator == "not_exists" {
		result.State = boolState(!ok)
		return result
	}
	if rule.Operator == "changed" {
		if previous == nil {
			result.State = "false"
			result.Message = "baseline established"
			return result
		}
		currentValue, currentOK := lookupValue(current, field, rule.Path)
		previousValue, previousOK := lookupValue(previous, field, rule.Path)
		if !currentOK || !previousOK {
			result.State = "unknown"
			result.Message = "value is missing from current or previous result"
			return result
		}
		result.Actual = currentValue
		result.Expected = previousValue
		result.State = boolState(!reflect.DeepEqual(currentValue, previousValue))
		return result
	}
	if !ok {
		result.State = "unknown"
		result.Message = "value is missing from result"
		return result
	}

	comparison := rule.Value
	if valueSource != "literal" {
		comparison, ok = lookupSourceValue(valueSource, current, previous, rule.ValueField, rule.ValuePath)
		if !ok {
			result.State = "unknown"
			result.Message = "comparison value is missing from current or previous result"
			return result
		}
	}
	result.Expected = comparison

	var matched bool
	var err error
	switch strings.ToLower(rule.Operator) {
	case "equals", "eq":
		matched = valuesEqual(value, comparison)
	case "not_equals", "neq":
		matched = !valuesEqual(value, comparison)
	case "contains":
		matched, err = containsValue(value, comparison)
	case "not_contains":
		var contains bool
		contains, err = containsValue(value, comparison)
		matched = !contains
	case "regex":
		var expression string
		expression, ok = comparison.(string)
		if !ok {
			err = fmt.Errorf("regex value must be a string")
			break
		}
		var compiled *regexp.Regexp
		compiled, err = regexp.Compile(expression)
		if err == nil {
			matched = compiled.MatchString(fmt.Sprint(value))
		}
	case "gt", "gte", "lt", "lte":
		var left, right float64
		left, err = numberValue(value)
		if err == nil {
			right, err = numberValue(comparison)
			if err == nil {
				switch rule.Operator {
				case "gt":
					matched = left > right
				case "gte":
					matched = left >= right
				case "lt":
					matched = left < right
				case "lte":
					matched = left <= right
				}
			}
		}
	case "between":
		bounds, boundsOK := comparison.([]any)
		if !boundsOK {
			if text, textOK := comparison.(string); textOK {
				parts := strings.Split(text, ",")
				if len(parts) == 2 {
					bounds = []any{strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])}
					boundsOK = true
				}
			}
		}
		if !boundsOK || len(bounds) != 2 {
			err = fmt.Errorf("between value must contain two numbers")
			break
		}
		left, leftErr := numberValue(value)
		low, lowErr := numberValue(bounds[0])
		high, highErr := numberValue(bounds[1])
		if leftErr != nil || lowErr != nil || highErr != nil {
			err = fmt.Errorf("between values must be numeric")
		} else {
			matched = left >= low && left <= high
		}
	case "length_gt", "length_eq":
		length, lengthErr := valueLength(value)
		threshold, thresholdErr := numberValue(comparison)
		if lengthErr != nil || thresholdErr != nil {
			err = fmt.Errorf("value or comparison is not measurable")
		} else if rule.Operator == "length_gt" {
			matched = float64(length) > threshold
		} else {
			matched = float64(length) == threshold
		}
	case "is_true":
		matched, ok = value.(bool)
		if !ok {
			err = fmt.Errorf("value is not boolean")
		}
	case "is_false":
		var boolean bool
		boolean, ok = value.(bool)
		if !ok {
			err = fmt.Errorf("value is not boolean")
		} else {
			matched = !boolean
		}
	default:
		err = fmt.Errorf("unsupported operator %q", rule.Operator)
	}
	if err != nil {
		result.State = "unknown"
		result.Message = err.Error()
		return result
	}
	result.State = boolState(matched)
	return result
}

func normalizeSource(source string) string {
	if source == "previous" {
		return "previous"
	}
	return "current"
}

func normalizeValueSource(source string) string {
	if source == "current" || source == "previous" {
		return source
	}
	return "literal"
}

func normalizeField(field string) string {
	return strings.TrimPrefix(strings.TrimPrefix(field, "result."), "current.")
}

func lookupSourceValue(source string, current, previous map[string]any, field, path string) (any, bool) {
	root := current
	if normalizeSource(source) == "previous" {
		root = previous
	}
	return lookupValue(root, normalizeField(field), path)
}

func containsValue(value, target any) (bool, error) {
	switch typed := value.(type) {
	case string:
		return strings.Contains(typed, fmt.Sprint(target)), nil
	case []any:
		for _, item := range typed {
			if valuesEqual(item, target) {
				return true, nil
			}
		}
		return false, nil
	case map[string]any:
		key := fmt.Sprint(target)
		if _, ok := typed[key]; ok {
			return true, nil
		}
		return false, nil
	default:
		return false, fmt.Errorf("value does not support contains")
	}
}

func valueLength(value any) (int, error) {
	switch typed := value.(type) {
	case string:
		return len([]rune(typed)), nil
	case []any:
		return len(typed), nil
	case map[string]any:
		return len(typed), nil
	default:
		return 0, fmt.Errorf("value has no length")
	}
}

func lookupValue(root map[string]any, field, path string) (any, bool) {
	value, ok := root[field]
	if !ok && strings.Contains(field, ".") {
		parts := strings.Split(field, ".")
		value = root
		ok = true
		for _, part := range parts {
			object, objectOK := value.(map[string]any)
			if !objectOK {
				ok = false
				break
			}
			value, ok = object[part]
			if !ok {
				break
			}
		}
	}
	if !ok || path == "" {
		return value, ok
	}
	for _, part := range strings.Split(strings.TrimPrefix(path, "."), ".") {
		object, objectOK := value.(map[string]any)
		if !objectOK {
			return nil, false
		}
		value, ok = object[part]
		if !ok {
			return nil, false
		}
	}
	return value, true
}

func valuesEqual(left, right any) bool {
	if leftNumber, err := numberValue(left); err == nil {
		if rightNumber, rightErr := numberValue(right); rightErr == nil {
			return leftNumber == rightNumber
		}
	}
	return reflect.DeepEqual(left, right) || fmt.Sprint(left) == fmt.Sprint(right)
}

func numberValue(value any) (float64, error) {
	switch typed := value.(type) {
	case float64:
		return typed, nil
	case float32:
		return float64(typed), nil
	case int:
		return float64(typed), nil
	case int64:
		return float64(typed), nil
	case json.Number:
		return typed.Float64()
	case string:
		return strconv.ParseFloat(typed, 64)
	default:
		return 0, fmt.Errorf("%T is not numeric", value)
	}
}

func boolState(value bool) string {
	if value {
		return "true"
	}
	return "false"
}
