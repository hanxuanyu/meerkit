package browser

func domBlurAction() actionSpec {
	return newAction("dom.blur", "取消元素焦点", "取消 CSS Selector 匹配元素的焦点。", "dom", "DOM", "unlink", "status", "tab_required", false, genericSelectorParameters("取消页面中第一个匹配元素的焦点。"), validateSelector)
}
