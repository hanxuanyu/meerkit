package runtime

import (
	"strings"
	"testing"

	"meerkit/internal/core"
)

func TestComposeExecutionSummaryIncludesExecutionAndConditionDetails(t *testing.T) {
	summary := composeExecutionSummary(
		"HTTP 请求：GET https://example.test/health\n响应状态：503 Service Unavailable",
		true,
		42,
		"",
		"",
		"triggered",
		core.Evaluation{State: "true", Details: []core.RuleResult{{Field: "response.status_code", Operator: "equals", State: "true", Expected: float64(503), Actual: float64(503)}}},
		"ALL",
		1,
	)

	for _, expected := range []string{
		"响应状态：503 Service Unavailable",
		"执行结果：成功",
		"执行耗时：42 ms",
		"事件类型：已触发",
		"条件状态：满足（ALL 逻辑，满足 1/1 条）",
		"条件详情：",
		"response.status_code",
		"期望：503",
		"实际值：503",
	} {
		if !strings.Contains(summary, expected) {
			t.Fatalf("summary does not contain %q:\n%s", expected, summary)
		}
	}
	if !strings.Contains(summary, "\n\n") {
		t.Fatalf("summary should separate module and runtime sections:\n%s", summary)
	}
}
