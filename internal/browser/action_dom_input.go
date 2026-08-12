package browser

import "github.com/hanxuanyu/meerkit/sdk"

func domInputAction() actionSpec {
	return newAction("dom.input", "填写控件", "设置输入框、文本域、下拉框或可编辑元素的值。", "dom", "DOM", "keyboard", "status", "tab_required", false, []sdk.ParameterDescriptor{
		{Key: "selector", Label: "CSS Selector", Description: "定位首个可填写或可编辑元素。", Type: sdk.ParameterString, Required: true, Placeholder: "input[name=email]", FullWidth: true},
		{Key: "value", Label: "输入内容", Description: "写入内容并触发 input 和 change 事件。", Type: sdk.ParameterText, Default: "", FullWidth: true, Rows: 5},
	}, validateSelector)
}
