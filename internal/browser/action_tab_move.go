package browser

import "github.com/hanxuanyu/meerkit/sdk"

func tabMoveAction() actionSpec {
	return newAction("tab.move", "移动标签页", "移动标签页到指定窗口和位置。", "tab", "标签页", "move-horizontal", "tab", "tab_required", false, []sdk.ParameterDescriptor{
		{Key: "destination_window_id", Label: "目标窗口", Description: "留空时在当前窗口内移动。", Type: sdk.ParameterBrowserWindow},
		{Key: "index", Label: "目标位置", Description: "从 0 开始，-1 表示窗口末尾。", Type: sdk.ParameterInteger, Default: -1, Minimum: number(-1), Maximum: number(100000)},
	}, validateTabMove)
}
