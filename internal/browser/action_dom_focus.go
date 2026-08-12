package browser

func domFocusAction() actionSpec {
	return newAction("dom.focus", "聚焦元素", "聚焦 CSS Selector 匹配的第一个元素。", "dom", "DOM", "focus", "status", "tab_required", false, genericSelectorParameters("聚焦页面中第一个匹配元素。"), validateSelector)
}
