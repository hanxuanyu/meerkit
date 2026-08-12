package browser

import "github.com/hanxuanyu/meerkit/sdk"

func tabReloadAction() actionSpec {
	return newAction("tab.reload", "重新加载", "重新加载指定标签页。", "tab", "标签页", "refresh-cw", "tab", "tab_required", false, []sdk.ParameterDescriptor{
		{Key: "bypass_cache", Label: "绕过缓存", Description: "忽略浏览器缓存重新请求资源。", Type: sdk.ParameterBoolean, Default: false},
		{Key: "wait", Label: "等待加载完成", Description: "页面加载完成后再返回结果。", Type: sdk.ParameterBoolean, Default: true},
	}, func(params map[string]any) error { return validateOptionalBooleans(params, "bypass_cache", "wait") })
}
