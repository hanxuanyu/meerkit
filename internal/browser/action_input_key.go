package browser

import "github.com/hanxuanyu/meerkit/sdk"

func inputKeyAction() actionSpec {
	return newAction("input.key", "发送按键", "通过 CDP 发送键盘按键。", "input", "输入", "keyboard", "status", "tab_required", false, []sdk.ParameterDescriptor{
		{Key: "key", Label: "按键", Description: "例如 Enter、Escape、Tab 或 ArrowDown。", Type: sdk.ParameterString, Required: true, FullWidth: true},
		{Key: "code", Label: "物理按键代码", Description: "可选，例如 Enter、KeyA。", Type: sdk.ParameterString},
		{Key: "text", Label: "按键文本", Description: "需要插入字符时设置。", Type: sdk.ParameterString},
		{Key: "modifiers", Label: "修饰键", Description: "可多选并用逗号分隔。", Type: sdk.ParameterString, Placeholder: "Control,Shift"},
		{Key: "repeat", Label: "重复次数", Description: "连续发送按键的次数。", Type: sdk.ParameterInteger, Default: 1, Minimum: number(1), Maximum: number(100)},
	}, validateInputKey)
}
