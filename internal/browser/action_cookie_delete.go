package browser

import "github.com/hanxuanyu/meerkit/sdk"

func cookieDeleteAction() actionSpec {
	return sensitiveAction(newAction("cookie.delete", "删除 Cookie", "删除当前标签页 URL 的指定 Cookie。", "cookie", "Cookie", "cookie-off", "status", "tab_required", true, []sdk.ParameterDescriptor{
		{Key: "name", Label: "名称", Description: "要删除的 Cookie 名称。", Type: sdk.ParameterString, Required: true},
		{Key: "store_id", Label: "Store ID", Description: "可选的 Cookie 存储区 ID。", Type: sdk.ParameterString},
	}, func(params map[string]any) error { return validateRequiredString(params, "name", 256) }))
}
