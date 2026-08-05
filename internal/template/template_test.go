package template

import (
	"testing"

	"meerkit/internal/core"
)

func TestRenderNestedResultTemplates(t *testing.T) {
	event := core.NotificationEvent{EventType: "triggered", MonitorID: "m1", MonitorName: "API", ModuleType: "http", Summary: "changed", CurrentResult: map[string]any{
		"response": map[string]any{"status_code": 503, "body_json": map[string]any{"state": "down"}},
	}}
	rendered, missing, err := Render(map[string]any{
		"url":  "https://example.test/{{result.response.status_code}}",
		"body": `{"state":"{{result.response.body_json.state}}"}`,
	}, NewContext(event))
	if err != nil || len(missing) != 0 {
		t.Fatalf("render failed: value=%#v missing=%v err=%v", rendered, missing, err)
	}
	config := rendered.(map[string]any)
	if config["url"] != "https://example.test/503" || config["body"] != `{"state":"down"}` {
		t.Fatalf("unexpected rendered values: %#v", config)
	}
}

func TestRenderReportsMissingPlaceholders(t *testing.T) {
	_, missing, err := Render("{{result.response.unknown}}", NewContext(core.NotificationEvent{CurrentResult: map[string]any{}}))
	if err != nil || len(missing) != 1 || missing[0] != "result.response.unknown" {
		t.Fatalf("missing placeholders = %v, err = %v", missing, err)
	}
}
