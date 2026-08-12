package browser

func windowCloseAction() actionSpec {
	return newAction("window.close", "关闭窗口", "关闭窗口及其中的所有标签页。", "window", "窗口", "x", "window", "window_required", true, nil, noValidation)
}
