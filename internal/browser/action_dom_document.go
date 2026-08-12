package browser

import "github.com/hanxuanyu/meerkit/sdk"

func domDocumentAction() actionSpec {
	return newAction("dom.document", "获取页面 HTML", "读取指定标签页文档的完整 HTML。", "dom", "DOM", "file-code-2", "document", "tab_required", false, []sdk.ParameterDescriptor{
		{Key: "max_length", Label: "最大 HTML 长度", Description: "超出长度的 HTML 将被截断。", Type: sdk.ParameterInteger, Default: 262144, Minimum: number(1024), Maximum: number(1048576), Step: number(1024), Unit: "字符"},
	}, func(params map[string]any) error {
		return validateNumberRange(params, "max_length", 1024, 1048576, true)
	})
}
