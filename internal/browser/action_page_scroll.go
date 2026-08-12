package browser

import "github.com/hanxuanyu/meerkit/sdk"

func pageScrollAction() actionSpec {
	return newAction("page.scroll", "滚动页面", "按位置或距离滚动页面。", "page", "页面", "move-vertical", "page", "tab_required", false, []sdk.ParameterDescriptor{
		{Key: "mode", Label: "滚动方式", Description: "选择绝对位置、相对距离或页面边界。", Type: sdk.ParameterList, Default: "relative", Options: labeledOptions("absolute", "绝对位置", "relative", "相对距离", "top", "页面顶部", "bottom", "页面底部")},
		{Key: "x", Label: "横向位置/距离", Description: "绝对坐标或相对偏移。", Type: sdk.ParameterInteger, Default: 0, Minimum: number(-10000000), Maximum: number(10000000), Unit: "px", VisibleWhen: []sdk.ParameterCondition{{Field: "mode", Operator: "in", Value: []string{"absolute", "relative"}}}},
		{Key: "y", Label: "纵向位置/距离", Description: "绝对坐标或相对偏移。", Type: sdk.ParameterInteger, Default: 600, Minimum: number(-10000000), Maximum: number(10000000), Unit: "px", VisibleWhen: []sdk.ParameterCondition{{Field: "mode", Operator: "in", Value: []string{"absolute", "relative"}}}},
		{Key: "behavior", Label: "滚动效果", Description: "立即滚动或使用页面平滑动画。", Type: sdk.ParameterList, Default: "auto", Options: labeledOptions("auto", "立即", "smooth", "平滑")},
	}, validatePageScroll)
}
