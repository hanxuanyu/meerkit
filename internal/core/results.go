package core

// CommonResultSets describes result groups produced by the runtime for every
// monitor module. Protocol modules only need to describe their own result sets.
func CommonResultSets() []ResultSetDescriptor {
	return []ResultSetDescriptor{
		{
			Key:         "summary",
			Label:       "执行摘要",
			Description: "本次监控任务的执行结果、触发状态和条件详情。",
			Scope:       "common",
			Fields: []ResultFieldDescriptor{
				{Name: "success", Label: "执行结果", Type: "boolean", Operators: []string{"is_true", "is_false", "changed"}},
				{Name: "triggered", Label: "是否触发", Type: "boolean", Operators: []string{}},
				{Name: "condition_state", Label: "条件状态", Type: "string", Operators: []string{}},
				{Name: "event_type", Label: "事件类型", Type: "string", Operators: []string{}},
				{Name: "condition_logic", Label: "条件组合逻辑", Type: "string", Operators: []string{}},
				{Name: "matched_count", Label: "满足条件数", Type: "number", Operators: []string{}},
				{Name: "condition_details", Label: "条件详情", Type: "array", Format: "condition_list", Operators: []string{}},
				{Name: "duration_ms", Label: "执行耗时", Type: "number", Unit: "ms", Operators: []string{"equals", "gt", "gte", "lt", "lte", "changed"}},
				{Name: "error_code", Label: "错误代码", Type: "string", Operators: []string{"equals", "not_equals", "contains", "changed"}},
				{Name: "error_message", Label: "错误信息", Type: "text", Operators: []string{"equals", "not_equals", "contains", "changed"}},
				{Name: "summary", Label: "执行摘要", Type: "text", Operators: []string{"equals", "not_equals", "contains", "changed"}},
			},
		},
	}
}

// WithCommonResultSets adds runtime-owned result groups to a module descriptor.
// It is idempotent so callers can safely use it for every descriptor response.
func WithCommonResultSets(descriptor ModuleDescriptor) ModuleDescriptor {
	result := make([]ResultSetDescriptor, 0, len(CommonResultSets())+len(descriptor.ResultSets))
	seen := make(map[string]struct{}, len(descriptor.ResultSets))
	for _, set := range CommonResultSets() {
		result = append(result, set)
		seen[set.Key] = struct{}{}
	}
	for _, set := range descriptor.ResultSets {
		if _, exists := seen[set.Key]; exists {
			continue
		}
		if set.Scope == "" {
			set.Scope = "module"
		}
		result = append(result, set)
		seen[set.Key] = struct{}{}
	}
	descriptor.ResultSets = result
	return descriptor
}
