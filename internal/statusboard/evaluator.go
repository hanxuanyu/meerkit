package statusboard

import (
	"encoding/json"
	"math"
	"regexp"
	"strconv"
	"strings"

	"meerkit/internal/core"
)

func BuildSamples(item core.StatusBoardItem, records []core.MonitorRecord) []core.StatusSample {
	textMatchers := compileTextValueMappings(item.Source.ValueMappings)
	samples := make([]core.StatusSample, 0, len(records))
	for _, record := range records {
		samples = append(samples, buildSample(item, record, textMatchers))
	}
	scaleNumericSamples(samples)
	return samples
}

func buildSample(item core.StatusBoardItem, record core.MonitorRecord, textMatchers []textValueMatcher) core.StatusSample {
	sample := core.StatusSample{RecordID: record.ID, StartedAt: record.StartedAt, Level: core.StatusLevelUnknown, State: core.StatusLevelUnknown, Display: "未知", Height: 28}
	value, ok := extractValue(item.Source, record)
	if !ok {
		return sample
	}
	sample.RawValue = value
	switch item.Source.ValueType {
	case core.StatusValueBoolean:
		boolean, valid := booleanValue(value)
		if !valid {
			return sample
		}
		if item.Invert {
			boolean = !boolean
		}
		sample.Display = strconv.FormatBool(boolean)
		sample.Height = 100
		if boolean {
			sample.State, sample.Level, sample.Label = core.StatusLevelSuccess, core.StatusLevelSuccess, "成功"
		} else {
			sample.State, sample.Level, sample.Label = core.StatusLevelFailure, core.StatusLevelFailure, "失败"
		}
	case core.StatusValueNumber:
		number, err := core.NumberValue(value)
		if err != nil || math.IsNaN(number) || math.IsInf(number, 0) {
			return sample
		}
		sample.Numeric = core.Float64(number)
		sample.Display = strconv.FormatFloat(number, 'f', -1, 64)
		if mapping := matchNumericValueMapping(item.Source.ValueMappings, number); mapping != nil {
			applySampleLevel(&sample, mapping.Level, mapping.Label, mapping.Color)
		} else {
			threshold := matchThreshold(item.Thresholds, number)
			if threshold == nil {
				return sample
			}
			applySampleLevel(&sample, threshold.Level, threshold.Label, threshold.Color)
		}
	case core.StatusValueText:
		text, valid := value.(string)
		if !valid {
			return sample
		}
		sample.RawValue, sample.Display, sample.Height = text, text, 100
		if strings.TrimSpace(text) == "" {
			sample.Display = "空"
		}
		if mapping := matchTextValueMapping(textMatchers, text); mapping != nil {
			applySampleLevel(&sample, mapping.Level, mapping.Label, mapping.Color)
		} else {
			level, label := item.Source.DefaultLevel, item.Source.DefaultLabel
			if level == "" {
				level = core.StatusLevelSuccess
			}
			if label == "" {
				label = "正常"
			}
			applySampleLevel(&sample, level, label, item.Source.DefaultColor)
		}
	}
	return sample
}

func matchNumericValueMapping(mappings []core.StatusValueMapping, value float64) *core.StatusValueMapping {
	for index := range mappings {
		mapped, err := strconv.ParseFloat(strings.TrimSpace(mappings[index].Value), 64)
		if err == nil && mapped == value {
			return &mappings[index]
		}
	}
	return nil
}

type textValueMatcher struct {
	mapping core.StatusValueMapping
	regex   *regexp.Regexp
}

func compileTextValueMappings(mappings []core.StatusValueMapping) []textValueMatcher {
	matchers := make([]textValueMatcher, 0, len(mappings))
	for _, mapping := range mappings {
		matcher := textValueMatcher{mapping: mapping}
		if mapping.MatchType == core.StatusMatchRegex {
			matcher.regex, _ = regexp.Compile(mapping.Value)
		}
		matchers = append(matchers, matcher)
	}
	return matchers
}

func matchTextValueMapping(matchers []textValueMatcher, value string) *core.StatusValueMapping {
	for index := range matchers {
		matcher := &matchers[index]
		if matcher.regex != nil && matcher.regex.MatchString(value) {
			return &matcher.mapping
		}
		if matcher.mapping.MatchType != core.StatusMatchRegex && matcher.mapping.Value == value {
			return &matcher.mapping
		}
	}
	return nil
}

func applySampleLevel(sample *core.StatusSample, level, label, color string) {
	sample.Level, sample.Label, sample.Color = level, label, color
	if level == core.StatusLevelSuccess {
		sample.State = core.StatusLevelSuccess
	} else {
		sample.State = core.StatusLevelFailure
	}
}

func extractValue(source core.StatusItemSource, record core.MonitorRecord) (any, bool) {
	switch source.Kind {
	case core.StatusSourceConditionOverall:
		switch record.ConditionState {
		case "true":
			return true, true
		case "false":
			return false, true
		default:
			return nil, false
		}
	case core.StatusSourceConditionRule:
		summary, ok := record.Result["summary"].(map[string]any)
		if !ok {
			return nil, false
		}
		for _, detail := range decodeConditionDetails(summary["condition_details"]) {
			if detail.ID != source.RuleID {
				continue
			}
			switch detail.State {
			case "true":
				return true, true
			case "false":
				return false, true
			default:
				return nil, false
			}
		}
		return nil, false
	case core.StatusSourceResultField:
		field := source.Field
		if source.ResultSet != "" && source.ResultSet != "result" {
			field = source.ResultSet + "." + source.Field
		}
		return core.LookupResultValue(record.Result, field, source.Path)
	default:
		return nil, false
	}
}

func booleanValue(value any) (bool, bool) {
	boolean, ok := value.(bool)
	return boolean, ok
}

func matchThreshold(thresholds []core.StatusThreshold, value float64) *core.StatusThreshold {
	for index := range thresholds {
		threshold := &thresholds[index]
		if threshold.Maximum == nil || value <= *threshold.Maximum {
			return threshold
		}
	}
	return nil
}

func scaleNumericSamples(samples []core.StatusSample) {
	minimum, maximum := math.Inf(1), math.Inf(-1)
	for _, sample := range samples {
		if sample.Numeric == nil {
			continue
		}
		minimum = math.Min(minimum, *sample.Numeric)
		maximum = math.Max(maximum, *sample.Numeric)
	}
	if math.IsInf(minimum, 0) {
		return
	}
	for index := range samples {
		if samples[index].Numeric == nil {
			continue
		}
		if minimum == maximum {
			samples[index].Height = 100
			continue
		}
		samples[index].Height = 10 + ((*samples[index].Numeric-minimum)/(maximum-minimum))*90
	}
}

func EvaluateTrend(rule core.TrendRule, samples []core.StatusSample) (string, map[string]any) {
	if rule.Window < 1 || len(samples) < rule.Window {
		return "unknown", map[string]any{"reason": "window_incomplete"}
	}
	window := samples[len(samples)-rule.Window:]
	for _, sample := range window {
		if sample.State == core.StatusLevelUnknown {
			return "unknown", map[string]any{"reason": "unknown_sample"}
		}
	}
	detail := map[string]any{"window": rule.Window, "threshold": rule.Threshold, "operator": rule.Operator}
	switch rule.Type {
	case core.TrendConsecutive:
		matched := true
		for _, sample := range window {
			matched = matched && sample.State == core.StatusLevelFailure
		}
		detail["abnormal_count"] = countAbnormal(window)
		return trendState(matched), detail
	case core.TrendCount:
		count := countAbnormal(window)
		detail["abnormal_count"] = count
		detail["minimum"] = rule.Minimum
		return trendState(count >= rule.Minimum), detail
	case core.TrendAverage, core.TrendDelta, core.TrendSlope:
		values := make([]float64, len(window))
		for index, sample := range window {
			if sample.Numeric == nil {
				return "unknown", map[string]any{"reason": "non_numeric_sample"}
			}
			values[index] = *sample.Numeric
		}
		var measured float64
		switch rule.Type {
		case core.TrendAverage:
			for _, value := range values {
				measured += value
			}
			measured /= float64(len(values))
		case core.TrendDelta:
			measured = values[len(values)-1] - values[0]
			if rule.DeltaMode == "percent" {
				if values[0] == 0 {
					return "unknown", map[string]any{"reason": "zero_percent_baseline"}
				}
				measured = measured / math.Abs(values[0]) * 100
			}
		case core.TrendSlope:
			measured = linearSlope(values)
		}
		detail["value"] = measured
		return trendState(compareNumber(measured, rule.Operator, rule.Threshold)), detail
	default:
		return "unknown", map[string]any{"reason": "unsupported_rule"}
	}
}

func countAbnormal(samples []core.StatusSample) int {
	count := 0
	for _, sample := range samples {
		if sample.State == core.StatusLevelFailure {
			count++
		}
	}
	return count
}

func trendState(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func compareNumber(value float64, operator string, threshold float64) bool {
	switch operator {
	case "gt":
		return value > threshold
	case "gte":
		return value >= threshold
	case "lt":
		return value < threshold
	case "lte":
		return value <= threshold
	default:
		return false
	}
}

func linearSlope(values []float64) float64 {
	n := float64(len(values))
	var sumX, sumY, sumXY, sumXX float64
	for index, value := range values {
		x := float64(index)
		sumX += x
		sumY += value
		sumXY += x * value
		sumXX += x * x
	}
	denominator := n*sumXX - sumX*sumX
	if denominator == 0 {
		return 0
	}
	return (n*sumXY - sumX*sumY) / denominator
}

func decodeConditionDetails(value any) []core.RuleResult {
	data, _ := json.Marshal(value)
	var details []core.RuleResult
	_ = json.Unmarshal(data, &details)
	return details
}
