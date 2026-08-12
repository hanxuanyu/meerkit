package browser

import "github.com/hanxuanyu/meerkit/sdk"

func domSubmitAction() actionSpec {
	return newAction("dom.submit", "提交表单", "提交 CSS Selector 匹配的表单或元素所属表单。", "dom", "DOM", "send", "status", "tab_required", false, []sdk.ParameterDescriptor{selectorParameter("定位表单或表单内控件。", selectorCandidates(60, "form", "button[type='submit']", "input[type='submit']"))}, validateSelector)
}
