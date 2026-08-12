package browser

func pageInfoAction() actionSpec {
	return newAction("page.info", "页面信息", "读取页面、视口和滚动区域基础信息。", "page", "页面", "info", "page", "tab_required", false, nil, noValidation)
}
