package browser

func domClickAction() actionSpec {
	return newAction("dom.click", "点击元素", "点击 CSS Selector 匹配的第一个元素。", "dom", "DOM", "mouse-pointer-click", "status", "tab_required", false, selectorParameters(), validateSelector)
}
