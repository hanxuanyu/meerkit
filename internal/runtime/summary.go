package runtime

import (
	"encoding/json"
	"fmt"
	"strings"

	"meerkit/internal/core"
)

const maxSummaryValueLength = 600

func composeExecutionSummary(descriptor core.ModuleDescriptor, moduleSummary string, success bool, durationMS int64, errorCode, errorMessage, eventType string, evaluation core.Evaluation, logic string, matchedCount int) string {
	sections := make([]string, 0, 2)
	if moduleSummary = strings.TrimSpace(moduleSummary); moduleSummary != "" {
		sections = append(sections, moduleSummary)
	}

	common := []string{
		fmt.Sprintf("执行结果：%s", executionStateLabel(success)),
		fmt.Sprintf("执行耗时：%d ms", durationMS),
	}
	if errorCode != "" {
		common = append(common, fmt.Sprintf("错误代码：%s", errorCode))
	}
	if errorMessage != "" {
		common = append(common, fmt.Sprintf("错误信息：%s", summaryValue(errorMessage)))
	}
	common = append(common,
		fmt.Sprintf("事件类型：%s", eventTypeLabel(eventType)),
		conditionSummary(descriptor, evaluation, logic, matchedCount),
	)
	sections = append(sections, strings.Join(common, "\n"))
	return strings.Join(sections, "\n\n")
}

func conditionSummary(descriptor core.ModuleDescriptor, evaluation core.Evaluation, logic string, matchedCount int) string {
	if len(evaluation.Details) == 0 {
		return "条件状态：未配置"
	}
	conditionLine := fmt.Sprintf("条件状态：%s（%s 逻辑，满足 %d/%d 条）", conditionStateLabel(evaluation.State), logic, matchedCount, len(evaluation.Details))
	detailLines := make([]string, 0, len(evaluation.Details)+1)
	detailLines = append(detailLines, conditionLine, "条件详情：")
	for index, detail := range evaluation.Details {
		field := conditionFieldDisplay(descriptor, detail.Field, detail.Path)
		line := fmt.Sprintf("%d. [%s] %s · %s %s", index+1, ruleStateLabel(detail.State), sourceLabel(detail.Source), field, operatorLabel(detail.Operator))
		if detail.Expected != nil {
			expectedSource := detail.ValueSource
			if detail.Operator == "changed" {
				expectedSource = "previous"
			}
			if expectedSource == "current" || expectedSource == "previous" {
				line += fmt.Sprintf(" %s期望：%s", sourceLabel(expectedSource), summaryValue(detail.Expected))
			} else {
				line += fmt.Sprintf("期望：%s", summaryValue(detail.Expected))
			}
		}
		if detail.Actual != nil {
			line += fmt.Sprintf("；实际值：%s", summaryValue(detail.Actual))
		} else if detail.Message != "" {
			line += "；实际值：缺失"
		}
		if detail.Message != "" {
			line += fmt.Sprintf("；说明：%s", summaryValue(detail.Message))
		}
		detailLines = append(detailLines, line)
	}
	return strings.Join(detailLines, "\n")
}

func conditionFieldDisplay(descriptor core.ModuleDescriptor, field, path string) string {
	key := field
	if path != "" {
		key += "." + strings.TrimPrefix(path, ".")
	}
	if label := resultFieldLabel(descriptor, field); label != "" {
		return fmt.Sprintf("%s（%s）", label, key)
	}
	return key
}

func resultFieldLabel(descriptor core.ModuleDescriptor, field string) string {
	descriptor = core.WithCommonResultSets(descriptor)
	for _, set := range descriptor.ResultSets {
		for _, resultField := range set.Fields {
			if field == set.Key+"."+resultField.Name {
				return resultField.Label
			}
		}
	}
	for _, legacyField := range descriptor.Fields {
		if field == legacyField.Name {
			return legacyField.Label
		}
	}
	return ""
}

func summaryValue(value any) string {
	if value == nil {
		return "空"
	}
	var text string
	if typed, ok := value.(string); ok {
		text = strings.ReplaceAll(typed, "\r", "")
		text = strings.ReplaceAll(text, "\n", "\\n")
	} else {
		data, err := json.Marshal(value)
		if err != nil {
			text = fmt.Sprint(value)
		} else {
			text = string(data)
		}
	}
	return truncateSummaryValue(text, maxSummaryValueLength)
}

func truncateSummaryValue(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit]) + "..."
}

func executionStateLabel(success bool) string {
	if success {
		return "成功"
	}
	return "失败"
}

func eventTypeLabel(eventType string) string {
	switch eventType {
	case "triggered":
		return "已触发"
	case "recovered":
		return "已恢复"
	case "none":
		return "无事件"
	default:
		if eventType == "" {
			return "未确定"
		}
		return eventType
	}
}

func conditionStateLabel(state string) string {
	switch state {
	case "true":
		return "满足"
	case "false":
		return "不满足"
	case "unknown":
		return "未知"
	default:
		if state == "" {
			return "未确定"
		}
		return state
	}
}

func ruleStateLabel(state string) string {
	switch state {
	case "true":
		return "满足"
	case "false":
		return "不满足"
	case "unknown":
		return "未知"
	default:
		if state == "" {
			return "未确定"
		}
		return state
	}
}

func sourceLabel(source string) string {
	if source == "previous" {
		return "上一次结果"
	}
	return "当前结果"
}

func operatorLabel(operator string) string {
	labels := map[string]string{
		"equals":       "等于",
		"eq":           "等于",
		"not_equals":   "不等于",
		"neq":          "不等于",
		"contains":     "包含",
		"not_contains": "不包含",
		"regex":        "匹配正则",
		"gt":           "大于",
		"gte":          "大于等于",
		"lt":           "小于",
		"lte":          "小于等于",
		"between":      "介于",
		"length_gt":    "长度大于",
		"length_eq":    "长度等于",
		"is_true":      "为真",
		"is_false":     "为假",
		"exists":       "存在",
		"not_exists":   "不存在",
		"changed":      "发生变化",
	}
	if label, ok := labels[operator]; ok {
		return label
	}
	return operator
}
