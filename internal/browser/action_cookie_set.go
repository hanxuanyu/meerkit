package browser

import "github.com/hanxuanyu/meerkit/sdk"

func cookieSetAction() actionSpec {
	return sensitiveAction(newAction("cookie.set", "设置 Cookie", "为当前标签页 URL 设置 Cookie。", "cookie", "Cookie", "cookie", "cookie", "tab_required", true, []sdk.ParameterDescriptor{
		{Key: "name", Label: "名称", Description: "Cookie 名称。", Type: sdk.ParameterString, Required: true},
		{Key: "value", Label: "值", Description: "Cookie 值。", Type: sdk.ParameterText, Required: true, Secret: true, FullWidth: true, Rows: 3},
		{Key: "domain", Label: "Domain", Description: "留空时使用当前标签页主机。", Type: sdk.ParameterString},
		{Key: "path", Label: "Path", Description: "Cookie 路径。", Type: sdk.ParameterString, Default: "/"},
		{Key: "same_site", Label: "SameSite", Description: "跨站请求携带策略。", Type: sdk.ParameterList, Default: "unspecified", Options: options("unspecified", "no_restriction", "lax", "strict")},
		{Key: "secure", Label: "Secure", Description: "仅通过安全连接发送。", Type: sdk.ParameterBoolean, Default: false},
		{Key: "http_only", Label: "HttpOnly", Description: "阻止页面 JavaScript 读取。", Type: sdk.ParameterBoolean, Default: false},
		{Key: "expiration_date", Label: "过期时间戳", Description: "Unix 秒；留空时创建会话 Cookie。", Type: sdk.ParameterNumber, Minimum: number(0)},
	}, validateCookieSet))
}
