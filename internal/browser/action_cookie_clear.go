package browser

func cookieClearAction() actionSpec {
	return sensitiveAction(newAction("cookie.clear", "清空 Cookie", "清空当前标签页 URL 可见的 Cookie。", "cookie", "Cookie", "trash-2", "status", "tab_required", true, nil, noValidation))
}
