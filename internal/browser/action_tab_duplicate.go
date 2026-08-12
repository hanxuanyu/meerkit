package browser

import "github.com/hanxuanyu/meerkit/sdk"

func tabDuplicateAction() actionSpec {
	return newAction("tab.duplicate", "复制标签页", "复制指定标签页及其导航状态。", "tab", "标签页", "copy", "tab", "tab_required", false, []sdk.ParameterDescriptor{
		{Key: "active", Label: "激活副本", Description: "创建后切换到复制出的标签页。", Type: sdk.ParameterBoolean, Default: true},
	}, func(params map[string]any) error { return validateOptionalBooleans(params, "active") })
}
