package browser

import "github.com/hanxuanyu/meerkit/sdk"

func domRemoveAttributeAction() actionSpec {
	return newAction("dom.remove_attribute", "删除元素属性", "删除 CSS Selector 匹配元素的指定属性。", "dom", "DOM", "tags", "status", "tab_required", false, []sdk.ParameterDescriptor{
		{Key: "selector", Label: "CSS Selector", Description: "定位要修改的首个元素。", Type: sdk.ParameterCSSSelector, Required: true, FullWidth: true},
		{Key: "name", Label: "属性名称", Description: "要删除的 HTML 属性名称。", Type: sdk.ParameterString, Required: true},
	}, func(params map[string]any) error { return validateDOMAttribute(params, false) })
}
