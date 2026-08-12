package browser

import "github.com/hanxuanyu/meerkit/sdk"

func runtimeEvaluateAction() actionSpec {
	return sensitiveAction(newAction("runtime.evaluate", "执行 JavaScript", "在页面主世界执行表达式并返回可序列化结果。", "runtime", "运行时", "code-2", "script", "tab_required", true, []sdk.ParameterDescriptor{
		{Key: "expression", Label: "JavaScript 表达式", Description: "在页面主世界执行；仅运行可信代码。", Type: sdk.ParameterText, Required: true, Default: "({ title: document.title, url: location.href })", FullWidth: true, Rows: 8, Format: "code"},
	}, validateExpression))
}
