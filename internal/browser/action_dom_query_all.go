package browser

import "github.com/hanxuanyu/meerkit/sdk"

func domQueryAllAction() actionSpec {
	return newAction("dom.query_all", "查询多个元素", "读取 CSS Selector 匹配的多个元素。", "dom", "DOM", "list-tree", "elements", "tab_required", false, []sdk.ParameterDescriptor{
		{Key: "selector", Label: "CSS Selector", Description: "读取页面中的全部匹配元素。", Type: sdk.ParameterString, Required: true, FullWidth: true},
		{Key: "limit", Label: "返回数量", Description: "最多返回的匹配元素数量。", Type: sdk.ParameterInteger, Default: 50, Minimum: number(1), Maximum: number(500)},
		{Key: "max_length", Label: "单项长度", Description: "每个元素文本和 HTML 的截断长度。", Type: sdk.ParameterInteger, Default: 4096, Minimum: number(64), Maximum: number(65536), Unit: "字符"},
	}, validateDOMQueryAll)
}
