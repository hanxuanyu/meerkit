package browser

import "github.com/hanxuanyu/meerkit/sdk"

func tabAutoDiscardableAction() actionSpec {
	return newAction("tab.auto_discardable", "自动卸载", "设置 Chrome 是否可以在内存紧张时自动卸载标签页。", "tab", "标签页", "memory-stick", "tab", "tab_required", false, []sdk.ParameterDescriptor{
		{Key: "auto_discardable", Label: "允许自动卸载", Description: "关闭后 Chrome 不会自动从内存卸载此标签页。", Type: sdk.ParameterBoolean, Default: true},
	}, func(params map[string]any) error { return validateOptionalBooleans(params, "auto_discardable") })
}
