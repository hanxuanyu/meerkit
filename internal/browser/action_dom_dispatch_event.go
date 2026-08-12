package browser

import "github.com/hanxuanyu/meerkit/sdk"

func domDispatchEventAction() actionSpec {
	return newAction("dom.dispatch_event", "派发 DOM 事件", "向 CSS Selector 匹配元素派发受限的标准事件。", "dom", "DOM", "zap", "status", "tab_required", false, []sdk.ParameterDescriptor{
		{Key: "selector", Label: "CSS Selector", Description: "定位接收事件的首个元素。", Type: sdk.ParameterCSSSelector, Required: true, FullWidth: true},
		{Key: "event", Label: "事件类型", Description: "仅允许常见表单和焦点事件。", Type: sdk.ParameterList, Required: true, Default: "change", Options: options("input", "change", "blur", "focus", "submit", "reset")},
		{Key: "bubbles", Label: "事件冒泡", Description: "允许事件沿 DOM 树向上冒泡。", Type: sdk.ParameterBoolean, Default: true},
		{Key: "cancelable", Label: "允许取消", Description: "允许事件监听器调用 preventDefault。", Type: sdk.ParameterBoolean, Default: true},
	}, validateDOMDispatchEvent)
}
