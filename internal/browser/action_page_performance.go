package browser

func pagePerformanceAction() actionSpec {
	return newAction("page.performance", "性能快照", "读取导航、绘制和资源加载的页面性能指标。", "page", "页面", "gauge", "performance", "tab_required", false, nil, noValidation)
}
