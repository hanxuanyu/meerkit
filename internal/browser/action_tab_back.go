package browser

func tabBackAction() actionSpec {
	return newAction("tab.back", "后退", "导航到标签页的上一条历史记录。", "tab", "标签页", "arrow-left", "tab", "tab_required", false, nil, noValidation)
}
