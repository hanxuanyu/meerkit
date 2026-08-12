package browser

import "github.com/hanxuanyu/meerkit/sdk"

func inputClickAction() actionSpec {
	return newAction("input.click", "真实鼠标点击", "通过 CDP 向元素中心发送鼠标点击。", "input", "输入", "mouse-pointer-click", "status", "tab_required", false, []sdk.ParameterDescriptor{
		{Key: "selector", Label: "CSS Selector", Description: "用于计算点击坐标的首个元素。", Type: sdk.ParameterString, Required: true, FullWidth: true},
		{Key: "button", Label: "鼠标按键", Description: "发送左键、右键或中键点击。", Type: sdk.ParameterList, Default: "left", Options: options("left", "right", "middle")},
		{Key: "click_count", Label: "点击次数", Description: "单击、双击或三击。", Type: sdk.ParameterInteger, Default: 1, Minimum: number(1), Maximum: number(3)},
	}, validateInputClick)
}
