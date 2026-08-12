package browser

import "github.com/hanxuanyu/meerkit/sdk"

func cookieListAction() actionSpec {
	return sensitiveAction(newAction("cookie.list", "读取 Cookie", "读取当前标签页 URL 可见的 Cookie。", "cookie", "Cookie", "cookie", "cookies", "tab_required", false, []sdk.ParameterDescriptor{
		{Key: "name", Label: "名称过滤", Description: "留空时返回当前 URL 的全部 Cookie。", Type: sdk.ParameterString, FullWidth: true},
	}, noValidation))
}
