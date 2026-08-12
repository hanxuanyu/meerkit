package browser

func inputHoverAction() actionSpec {
	return newAction("input.hover", "真实鼠标悬停", "将 CDP 鼠标移动到元素中心。", "input", "输入", "mouse-pointer-2", "status", "tab_required", false, genericSelectorParameters("用于计算悬停坐标的首个元素。"), validateSelector)
}
