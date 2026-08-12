package browser

import "github.com/hanxuanyu/meerkit/sdk"

func tabOpenAction() actionSpec {
	return newAction("tab.open", "打开标签页", "在指定窗口或当前窗口创建新标签页。", "tab", "标签页", "panel-top-open", "tab", "window_optional", false, []sdk.ParameterDescriptor{
		{Key: "url", Label: "页面地址", Description: "新标签页地址；留空时打开 about:blank。", Type: sdk.ParameterURL, Default: "about:blank", Placeholder: "https://example.com", FullWidth: true},
		{Key: "active", Label: "前台打开", Description: "创建后切换到新标签页，否则在后台打开。", Type: sdk.ParameterBoolean, Default: true},
		{Key: "wait", Label: "等待加载完成", Description: "页面加载完成后再返回执行结果。", Type: sdk.ParameterBoolean, Default: true},
	}, validateTabOpen)
}
