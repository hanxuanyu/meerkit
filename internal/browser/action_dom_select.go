package browser

import "github.com/hanxuanyu/meerkit/sdk"

func domSelectAction() actionSpec {
	return newAction("dom.select", "选择下拉选项", "设置 select 控件的值。", "dom", "DOM", "list-checks", "status", "tab_required", false, []sdk.ParameterDescriptor{
		{Key: "selector", Label: "CSS Selector", Description: "定位页面中的首个 select。", Type: sdk.ParameterCSSSelector, Required: true, FullWidth: true, SelectorCandidates: selectorCandidates(60, "select")},
		{Key: "value", Label: "选项值", Description: "要选择的 option value。", Type: sdk.ParameterString, Required: true, FullWidth: true},
	}, validateDOMSelect)
}
