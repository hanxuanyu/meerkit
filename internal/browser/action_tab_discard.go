package browser

func tabDiscardAction() actionSpec {
	return newAction("tab.discard", "卸载标签页", "从内存卸载后台标签页；再次激活时会重新加载。", "tab", "标签页", "memory-stick", "tab", "tab_required", false, nil, noValidation)
}
