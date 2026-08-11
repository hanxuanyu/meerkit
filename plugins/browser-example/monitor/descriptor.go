package browsermonitor

import "github.com/hanxuanyu/meerkit/sdk"

func stringOperators() []string {
	return []string{"equals", "not_equals", "contains", "not_contains", "regex", "changed"}
}
func numberOperators() []string { return []string{"equals", "gt", "gte", "lt", "lte", "changed"} }
func boolOperators() []string   { return []string{"is_true", "is_false", "changed"} }

func resultSchema(fields []sdk.ResultFieldDescriptor) map[string]any {
	properties := make(map[string]any, len(fields))
	for _, field := range fields {
		fieldType := field.Type
		switch fieldType {
		case "text":
			fieldType = "string"
		case "map":
			fieldType = "object"
		case "json":
			properties[field.Name] = map[string]any{}
			continue
		}
		properties[field.Name] = map[string]any{"type": fieldType}
	}
	return map[string]any{"type": "object", "properties": properties}
}

func legacyFields(fields []sdk.ResultFieldDescriptor) []sdk.FieldDescriptor {
	result := make([]sdk.FieldDescriptor, 0, len(fields))
	for _, field := range fields {
		result = append(result, sdk.FieldDescriptor{Name: field.Name, Label: field.Label, Type: field.Type, Operators: field.Operators})
	}
	return result
}
