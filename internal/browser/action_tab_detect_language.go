package browser

func tabDetectLanguageAction() actionSpec {
	return newAction("tab.detect_language", "检测页面语言", "检测标签页内容使用的主要语言。", "tab", "标签页", "languages", "language", "tab_required", false, nil, noValidation)
}
