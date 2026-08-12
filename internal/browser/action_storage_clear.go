package browser

import "github.com/hanxuanyu/meerkit/sdk"

func storageClearAction() actionSpec {
	return sensitiveAction(newAction("storage.clear", "清空页面存储", "清空指定页面存储区域。", "storage", "页面存储", "database-backup", "status", "tab_required", true, []sdk.ParameterDescriptor{
		{Key: "area", Label: "存储区域", Description: "选择要清空的本地或会话存储。", Type: sdk.ParameterList, Default: "local", Options: storageAreaOptions()},
	}, func(params map[string]any) error { return validateChoice(params, "area", false, "local", "session") }))
}
