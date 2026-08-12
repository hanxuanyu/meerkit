package browser

import "github.com/hanxuanyu/meerkit/sdk"

func domScrollIntoViewAction() actionSpec {
	return newAction("dom.scroll_into_view", "滚动到元素", "将匹配元素滚动到视口中。", "dom", "DOM", "locate-fixed", "status", "tab_required", false, []sdk.ParameterDescriptor{
		{Key: "selector", Label: "CSS Selector", Description: "定位要滚动到的首个元素。", Type: sdk.ParameterString, Required: true, FullWidth: true},
		{Key: "block", Label: "垂直对齐", Description: "元素在视口内的垂直位置。", Type: sdk.ParameterList, Default: "center", Options: options("start", "center", "end", "nearest")},
		{Key: "inline", Label: "水平对齐", Description: "元素在视口内的水平位置。", Type: sdk.ParameterList, Default: "nearest", Options: options("start", "center", "end", "nearest")},
		{Key: "behavior", Label: "滚动效果", Description: "立即滚动或使用平滑动画。", Type: sdk.ParameterList, Default: "auto", Options: labeledOptions("auto", "立即", "smooth", "平滑")},
	}, validateDOMScrollIntoView)
}
