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
	Logic string          `json:"logic"`
	Rules []ConditionRule `json:"rules"`
}

type ConditionRule struct {
	Field    string `json:"field"`
	Path     string `json:"path,omitempty"`
	Operator string `json:"operator"`
	Value    any    `json:"value,omitempty"`
}

type RuleResult struct {
	Field    string `json:"field"`
	Operator string `json:"operator"`
	State    string `json:"state"`
	Message  string `json:"message,omitempty"`
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
	result := RuleResult{Field: rule.Field, Operator: rule.Operator}
	value, ok := lookupValue(current, rule.Field, rule.Path)
	if rule.Operator == "changed" {
		if previous == nil {
			result.State = "false"
			result.Message = "baseline established"
			return result
		}
		previousValue, previousOK := lookupValue(previous, rule.Field, rule.Path)
		if !ok || !previousOK {
			result.State = "unknown"
			result.Message = "value is missing from current or previous result"
			return result
		}
		result.State = boolState(!reflect.DeepEqual(value, previousValue))
		return result
	}
	if !ok {
		result.State = "unknown"
		result.Message = "value is missing from result"
		return result
	}

	var matched bool
	var err error
	switch strings.ToLower(rule.Operator) {
	case "equals", "eq":
		matched = valuesEqual(value, rule.Value)
	case "not_equals", "neq":
		matched = !valuesEqual(value, rule.Value)
	case "contains":
		matched = strings.Contains(fmt.Sprint(value), fmt.Sprint(rule.Value))
	case "not_contains":
		matched = !strings.Contains(fmt.Sprint(value), fmt.Sprint(rule.Value))
	case "regex":
		var expression string
		expression, ok = rule.Value.(string)
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
			right, err = numberValue(rule.Value)
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
