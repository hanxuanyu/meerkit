package browser

import "github.com/hanxuanyu/meerkit/sdk"

func inputTypeAction() actionSpec {
	return newAction("input.type", "真实键盘输入", "通过 CDP 向页面输入文本。", "input", "输入", "keyboard", "status", "tab_required", false, []sdk.ParameterDescriptor{
		{Key: "selector", Label: "CSS Selector", Description: "输入前要聚焦的页面元素。", Type: sdk.ParameterCSSSelector, Required: true, FullWidth: true, SelectorCandidates: editableSelectorCandidates()},
		{Key: "text", Label: "输入文本", Description: "按 Unicode 字符逐个输入。", Type: sdk.ParameterText, Default: "", FullWidth: true, Rows: 4},
		{Key: "clear", Label: "清空原内容", Description: "输入前使用全选和退格清空控件。", Type: sdk.ParameterBoolean, Default: false},
		{Key: "interval_ms", Label: "字符间隔", Description: "每个字符之间的等待时间。", Type: sdk.ParameterDuration, Default: 20, Minimum: number(0), Maximum: number(5000), Unit: "ms"},
	}, validateInputType)
}
