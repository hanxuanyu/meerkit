package browser

import "github.com/hanxuanyu/meerkit/sdk"

func tabPinAction() actionSpec {
	return newAction("tab.pin", "固定标签页", "设置标签页的固定状态。", "tab", "标签页", "pin", "tab", "tab_required", false, []sdk.ParameterDescriptor{
		{Key: "pinned", Label: "固定标签页", Description: "开启时将标签页固定在窗口左侧。", Type: sdk.ParameterBoolean, Default: true},
	}, func(params map[string]any) error { return validateOptionalBooleans(params, "pinned") })
}
