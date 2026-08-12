package browser

func tabForwardAction() actionSpec {
	return newAction("tab.forward", "前进", "导航到标签页的下一条历史记录。", "tab", "标签页", "arrow-right", "tab", "tab_required", false, nil, noValidation)
}
