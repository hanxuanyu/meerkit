package browser

func tabActivateAction() actionSpec {
	return newAction("tab.activate", "激活标签页", "激活标签页并聚焦所在窗口。", "tab", "标签页", "mouse-pointer-2", "tab", "tab_required", false, nil, noValidation)
}
