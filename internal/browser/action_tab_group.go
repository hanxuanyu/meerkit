package browser

import "github.com/hanxuanyu/meerkit/sdk"

func tabGroupAction() actionSpec {
	return newAction("tab.group", "加入标签页分组", "将指定标签页加入或复用 Chrome 分组。", "tab", "标签页", "group", "tab", "tab_required", false, []sdk.ParameterDescriptor{
		{Key: "title", Label: "分组名称", Description: "标签页分组名称，最多 128 个字符。", Type: sdk.ParameterString, Required: true, Default: "Meerkit", FullWidth: true},
		{Key: "color", Label: "分组颜色", Description: "分组在 Chrome 中显示的标识颜色。", Type: sdk.ParameterList, Default: "blue", Options: options("grey", "blue", "red", "yellow", "green", "pink", "purple", "cyan", "orange")},
		{Key: "collapsed", Label: "折叠分组", Description: "创建或更新后折叠标签页分组。", Type: sdk.ParameterBoolean, Default: false},
		{Key: "reuse_group", Label: "复用同名分组", Description: "优先复用当前窗口中的同名分组。", Type: sdk.ParameterBoolean, Default: true},
	}, validateTabGroup)
}
