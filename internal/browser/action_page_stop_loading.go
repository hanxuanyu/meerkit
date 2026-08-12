package browser

func pageStopLoadingAction() actionSpec {
	return newAction("page.stop_loading", "停止加载", "停止当前标签页仍在进行的页面加载。", "page", "页面", "circle-stop", "status", "tab_required", false, nil, noValidation)
}
