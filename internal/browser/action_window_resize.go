package browser

import "github.com/hanxuanyu/meerkit/sdk"

func windowResizeAction() actionSpec {
	return newAction("window.resize", "调整窗口", "调整普通状态窗口的尺寸和位置。", "window", "窗口", "move-diagonal-2", "window", "window_required", false, []sdk.ParameterDescriptor{
		{Key: "width", Label: "宽度", Description: "窗口目标宽度。", Type: sdk.ParameterInteger, Required: true, Default: 1280, Minimum: number(100), Maximum: number(10000), Unit: "px"},
		{Key: "height", Label: "高度", Description: "窗口目标高度。", Type: sdk.ParameterInteger, Required: true, Default: 800, Minimum: number(100), Maximum: number(10000), Unit: "px"},
		{Key: "left", Label: "左侧位置", Description: "可选的窗口左边缘坐标。", Type: sdk.ParameterInteger, Minimum: number(-10000), Maximum: number(10000), Unit: "px"},
		{Key: "top", Label: "顶部位置", Description: "可选的窗口上边缘坐标。", Type: sdk.ParameterInteger, Minimum: number(-10000), Maximum: number(10000), Unit: "px"},
	}, validateWindowResize)
}
