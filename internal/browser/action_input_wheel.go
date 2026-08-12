package browser

import "github.com/hanxuanyu/meerkit/sdk"

func inputWheelAction() actionSpec {
	return newAction("input.wheel", "真实滚轮", "通过 CDP 发送鼠标滚轮事件。", "input", "输入", "mouse", "status", "tab_required", false, []sdk.ParameterDescriptor{
		{Key: "selector", Label: "CSS Selector", Description: "可选的滚轮坐标元素，留空使用视口中心。", Type: sdk.ParameterCSSSelector, FullWidth: true},
		{Key: "delta_x", Label: "横向滚动", Description: "水平方向滚轮距离。", Type: sdk.ParameterInteger, Default: 0, Minimum: number(-1000000), Maximum: number(1000000), Unit: "px"},
		{Key: "delta_y", Label: "纵向滚动", Description: "垂直方向滚轮距离。", Type: sdk.ParameterInteger, Default: 600, Minimum: number(-1000000), Maximum: number(1000000), Unit: "px"},
	}, validateInputWheel)
}
