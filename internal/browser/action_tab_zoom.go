package browser

import "github.com/hanxuanyu/meerkit/sdk"

func tabZoomAction() actionSpec {
	return newAction("tab.zoom", "页面缩放", "设置标签页页面缩放比例。", "tab", "标签页", "zoom-in", "tab", "tab_required", false, []sdk.ParameterDescriptor{
		{Key: "factor", Label: "缩放比例", Description: "1 表示 100% 原始大小。", Type: sdk.ParameterNumber, Required: true, Default: 1, Minimum: number(0.25), Maximum: number(5), Step: number(0.1)},
	}, func(params map[string]any) error { return validateNumberRange(params, "factor", 0.25, 5, false) })
}
