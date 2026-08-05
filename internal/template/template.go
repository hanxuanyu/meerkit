package template

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"

	"meerkit/internal/core"
)

var placeholderPattern = regexp.MustCompile(`\{\{\s*([^{}]+?)\s*\}\}`)

type Context struct {
	Monitor  map[string]any
	Event    map[string]any
	Result   map[string]any
	Previous map[string]any
}

func NewContext(event core.NotificationEvent) Context {
	return Context{
		Monitor: map[string]any{"id": event.MonitorID, "name": event.MonitorName, "module_type": event.ModuleType},
		Event:   map[string]any{"type": event.EventType, "event_type": event.EventType, "record_id": event.RecordID, "detail_path": detailPath(event), "condition_state": event.ConditionState, "summary": event.Summary, "triggered_at": event.TriggeredAt.Format(time.RFC3339)},
		Result:  event.CurrentResult, Previous: event.PreviousResult,
	}
}

func detailPath(event core.NotificationEvent) string {
	if event.MonitorID == "" || event.RecordID == "" {
		return ""
	}
	return fmt.Sprintf("/monitors/%s/records/%s", event.MonitorID, event.RecordID)
}

func Scan(value any) []string {
	found := map[string]struct{}{}
	var walk func(any)
	walk = func(current any) {
		switch typed := current.(type) {
		case string:
			for _, match := range placeholderPattern.FindAllStringSubmatch(typed, -1) {
				found[strings.TrimSpace(match[1])] = struct{}{}
			}
		case map[string]any:
			for key, item := range typed {
				walk(key)
				walk(item)
			}
		case []any:
			for _, item := range typed {
				walk(item)
			}
		default:
			value := reflect.ValueOf(current)
			if value.IsValid() && value.Kind() == reflect.Map {
				for _, key := range value.MapKeys() {
					walk(fmt.Sprint(key.Interface()))
					walk(value.MapIndex(key).Interface())
				}
			}
		}
	}
	walk(value)
	result := make([]string, 0, len(found))
	for item := range found {
		result = append(result, item)
	}
	sort.Strings(result)
	return result
}

func Render(value any, context Context) (any, []string, error) {
	missing := map[string]struct{}{}
	var render func(any) (any, error)
	render = func(current any) (any, error) {
		switch typed := current.(type) {
		case string:
			return placeholderPattern.ReplaceAllStringFunc(typed, func(match string) string {
				parts := placeholderPattern.FindStringSubmatch(match)
				key := strings.TrimSpace(parts[1])
				value, ok := Lookup(key, context)
				if !ok {
					missing[key] = struct{}{}
					return match
				}
				return stringify(value)
			}), nil
		case map[string]any:
			result := make(map[string]any, len(typed))
			for key, item := range typed {
				rendered, err := render(item)
				if err != nil {
					return nil, err
				}
				result[key] = rendered
			}
			return result, nil
		case []any:
			result := make([]any, len(typed))
			for index, item := range typed {
				rendered, err := render(item)
				if err != nil {
					return nil, err
				}
				result[index] = rendered
			}
			return result, nil
		default:
			return current, nil
		}
	}
	rendered, err := render(value)
	if err != nil {
		return nil, nil, err
	}
	missingKeys := make([]string, 0, len(missing))
	for key := range missing {
		missingKeys = append(missingKeys, key)
	}
	sort.Strings(missingKeys)
	return rendered, missingKeys, nil
}

func Lookup(key string, context Context) (any, bool) {
	parts := strings.Split(strings.TrimSpace(key), ".")
	if len(parts) == 0 || parts[0] == "" {
		return nil, false
	}
	var value any
	switch parts[0] {
	case "monitor":
		value = context.Monitor
	case "event":
		value = context.Event
	case "result":
		value = context.Result
	case "previous":
		value = context.Previous
	default:
		return nil, false
	}
	if len(parts) == 1 {
		return value, true
	}
	for _, part := range parts[1:] {
		object, ok := value.(map[string]any)
		if !ok {
			return nil, false
		}
		value, ok = object[part]
		if !ok {
			return nil, false
		}
	}
	return value, true
}

func MustRenderString(value string, context Context) (string, error) {
	rendered, missing, err := Render(value, context)
	if err != nil {
		return "", err
	}
	if len(missing) > 0 {
		return "", fmt.Errorf("missing template placeholders: %s", strings.Join(missing, ", "))
	}
	return fmt.Sprint(rendered), nil
}

func stringify(value any) string {
	if value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	if reflect.TypeOf(value).Kind() == reflect.Map || reflect.TypeOf(value).Kind() == reflect.Slice {
		if data, err := json.MarshalIndent(value, "", "  "); err == nil {
			return string(data)
		}
	}
	return fmt.Sprint(value)
}
