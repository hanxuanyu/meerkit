package browser

import "github.com/hanxuanyu/meerkit/sdk"

func windowStateAction() actionSpec {
	return newAction("window.state", "设置窗口状态", "切换窗口的最小化、最大化或全屏状态。", "window", "窗口", "panels-top-left", "window", "window_required", false, []sdk.ParameterDescriptor{
		{Key: "state", Label: "窗口状态", Description: "要切换到的目标状态。", Type: sdk.ParameterList, Required: true, Default: "normal", Options: windowStateOptions()},
	}, func(params map[string]any) error {
		return validateChoice(params, "state", true, "normal", "minimized", "maximized", "fullscreen")
	})
}
