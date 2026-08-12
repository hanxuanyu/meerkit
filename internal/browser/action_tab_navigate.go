package browser

import "github.com/hanxuanyu/meerkit/sdk"

func tabNavigateAction() actionSpec {
	return newAction("tab.navigate", "导航标签页", "将指定标签页导航到对应地址。", "tab", "标签页", "globe", "tab", "tab_required", false, []sdk.ParameterDescriptor{
		{Key: "url", Label: "导航地址", Description: "标签页要访问的完整 HTTP 或 HTTPS 地址。", Type: sdk.ParameterURL, Required: true, Placeholder: "https://example.com", FullWidth: true},
		{Key: "wait", Label: "等待加载完成", Description: "页面加载完成后再返回执行结果。", Type: sdk.ParameterBoolean, Default: true},
	}, func(params map[string]any) error { return validateHTTPURL(stringParam(params, "url"), false) })
}
