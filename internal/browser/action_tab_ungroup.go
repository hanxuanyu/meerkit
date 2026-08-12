package browser

func tabUngroupAction() actionSpec {
	return newAction("tab.ungroup", "移出标签页分组", "将标签页从当前 Chrome 分组移除。", "tab", "标签页", "ungroup", "tab", "tab_required", false, nil, noValidation)
}
