package browser

import "github.com/hanxuanyu/meerkit/sdk"

func domSetAttributeAction() actionSpec {
	return newAction("dom.set_attribute", "设置元素属性", "设置 CSS Selector 匹配元素的指定属性。", "dom", "DOM", "tags", "status", "tab_required", false, []sdk.ParameterDescriptor{
		{Key: "selector", Label: "CSS Selector", Description: "定位要修改的首个元素。", Type: sdk.ParameterCSSSelector, Required: true, FullWidth: true},
		{Key: "name", Label: "属性名称", Description: "要设置的 HTML 属性名称。", Type: sdk.ParameterString, Required: true},
		{Key: "value", Label: "属性值", Description: "写入属性的字符串值。", Type: sdk.ParameterText, Default: "", FullWidth: true, Rows: 3},
	}, func(params map[string]any) error { return validateDOMAttribute(params, true) })
}
