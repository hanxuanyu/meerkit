package browser

import "github.com/hanxuanyu/meerkit/sdk"

func windowOpenAction() actionSpec {
	return newAction("window.open", "打开窗口", "创建新的 Chrome 窗口。", "window", "窗口", "app-window", "window", "none", false, []sdk.ParameterDescriptor{
		{Key: "url", Label: "页面地址", Description: "新窗口首个标签页地址。", Type: sdk.ParameterURL, Default: "about:blank", FullWidth: true},
		{Key: "type", Label: "窗口类型", Description: "普通窗口或无工具栏弹窗。", Type: sdk.ParameterList, Default: "normal", Options: labeledOptions("normal", "普通窗口", "popup", "弹出窗口")},
		{Key: "state", Label: "窗口状态", Description: "窗口创建后的显示状态。", Type: sdk.ParameterList, Default: "normal", Options: windowStateOptions()},
		{Key: "width", Label: "宽度", Description: "普通状态窗口宽度。", Type: sdk.ParameterInteger, Minimum: number(100), Maximum: number(10000), Unit: "px"},
		{Key: "height", Label: "高度", Description: "普通状态窗口高度。", Type: sdk.ParameterInteger, Minimum: number(100), Maximum: number(10000), Unit: "px"},
		{Key: "left", Label: "左侧位置", Description: "窗口左边缘屏幕坐标。", Type: sdk.ParameterInteger, Minimum: number(-10000), Maximum: number(10000), Unit: "px"},
		{Key: "top", Label: "顶部位置", Description: "窗口上边缘屏幕坐标。", Type: sdk.ParameterInteger, Minimum: number(-10000), Maximum: number(10000), Unit: "px"},
	}, validateWindowOpen)
}
