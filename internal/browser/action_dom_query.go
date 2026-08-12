package browser

import "github.com/hanxuanyu/meerkit/sdk"

func domQueryAction() actionSpec {
	return newAction("dom.query", "查询元素", "通过 CSS Selector 读取元素文本、HTML 和属性。", "dom", "DOM", "search", "element", "tab_required", false, []sdk.ParameterDescriptor{
		{Key: "selector", Label: "CSS Selector", Description: "读取页面中第一个匹配元素。", Type: sdk.ParameterString, Required: true, Placeholder: "#app, main, [data-id]", FullWidth: true},
		{Key: "max_length", Label: "最大返回长度", Description: "文本和 HTML 超出此长度时截断。", Type: sdk.ParameterInteger, Default: 65536, Minimum: number(256), Maximum: number(1048576), Step: number(1024), Unit: "字符"},
	}, validateDOMQuery)
}
