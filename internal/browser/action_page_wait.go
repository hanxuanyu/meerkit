package browser

import "github.com/hanxuanyu/meerkit/sdk"

func pageWaitAction() actionSpec {
	return newAction("page.wait", "等待页面", "等待页面加载、CSS 元素出现或固定时长。", "page", "页面", "timer", "status", "tab_required", false, []sdk.ParameterDescriptor{
		{Key: "mode", Label: "等待方式", Description: "等待页面加载、元素出现或固定时长。", Type: sdk.ParameterList, Default: "load", Options: []sdk.ParameterOption{{Value: "load", Label: "页面加载"}, {Value: "selector", Label: "CSS 元素"}, {Value: "duration", Label: "固定时长"}}},
		{Key: "selector", Label: "CSS Selector", Description: "首个匹配元素出现后结束等待。", Type: sdk.ParameterString, Placeholder: "#app, main", FullWidth: true, VisibleWhen: visibleWhen("mode", "selector")},
		{Key: "timeout_ms", Label: "最长等待", Description: "元素未出现时的超时上限。", Type: sdk.ParameterDuration, Default: 60000, Minimum: number(100), Maximum: number(300000), Step: number(1000), Unit: "ms", VisibleWhen: visibleWhen("mode", "selector")},
		{Key: "duration_ms", Label: "等待时长", Description: "无条件暂停执行的时间。", Type: sdk.ParameterDuration, Default: 1000, Minimum: number(0), Maximum: number(300000), Step: number(100), Unit: "ms", VisibleWhen: visibleWhen("mode", "duration")},
	}, validatePageWait)
}
