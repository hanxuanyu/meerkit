package browser

func tabCloseAction() actionSpec {
	return newAction("tab.close", "关闭标签页", "关闭指定标签页。", "tab", "标签页", "trash-2", "tab", "tab_required", true, nil, noValidation)
}
