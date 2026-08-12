package browser

import "github.com/hanxuanyu/meerkit/sdk"

func pageWaitAction() actionSpec {
	return newAction("page.wait", "等待页面", "等待页面加载、页面条件满足或固定时长。", "page", "页面", "timer", "status", "tab_required", false, []sdk.ParameterDescriptor{
		{Key: "mode", Label: "等待方式", Description: "等待加载、元素状态、页面文本、地址、标题或固定时长。", Type: sdk.ParameterList, Default: "load", Options: []sdk.ParameterOption{{Value: "load", Label: "页面加载"}, {Value: "selector", Label: "元素出现"}, {Value: "visible", Label: "元素可见"}, {Value: "hidden", Label: "元素隐藏"}, {Value: "text", Label: "页面包含文本"}, {Value: "url", Label: "地址包含文本"}, {Value: "title", Label: "标题包含文本"}, {Value: "duration", Label: "固定时长"}}},
		{Key: "selector", Label: "CSS Selector", Description: "用于判断元素存在或可见状态。", Type: sdk.ParameterCSSSelector, Placeholder: "#app, main", FullWidth: true, VisibleWhen: []sdk.ParameterCondition{{Field: "mode", Operator: "in", Value: []string{"selector", "visible", "hidden"}}}},
		{Key: "value", Label: "匹配文本", Description: "页面正文、地址或标题需要包含的文本。", Type: sdk.ParameterString, FullWidth: true, VisibleWhen: []sdk.ParameterCondition{{Field: "mode", Operator: "in", Value: []string{"text", "url", "title"}}}},
		{Key: "timeout_ms", Label: "最长等待", Description: "条件未满足时的超时上限。", Type: sdk.ParameterDuration, Default: 60000, Minimum: number(100), Maximum: number(300000), Step: number(1000), Unit: "ms", VisibleWhen: []sdk.ParameterCondition{{Field: "mode", Operator: "in", Value: []string{"selector", "visible", "hidden", "text", "url", "title"}}}},
		{Key: "duration_ms", Label: "等待时长", Description: "无条件暂停执行的时间。", Type: sdk.ParameterDuration, Default: 1000, Minimum: number(0), Maximum: number(300000), Step: number(100), Unit: "ms", VisibleWhen: visibleWhen("mode", "duration")},
	}, validatePageWait)
}
