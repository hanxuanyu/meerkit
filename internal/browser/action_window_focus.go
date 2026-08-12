package browser

func windowFocusAction() actionSpec {
	return newAction("window.focus", "聚焦窗口", "将指定窗口切换到前台。", "window", "窗口", "scan", "window", "window_required", false, nil, noValidation)
}
