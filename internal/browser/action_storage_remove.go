package browser

import "github.com/hanxuanyu/meerkit/sdk"

func storageRemoveAction() actionSpec {
	return sensitiveAction(newAction("storage.remove", "删除存储项", "删除页面存储中的指定 Key。", "storage", "页面存储", "database-x", "status", "tab_required", true, []sdk.ParameterDescriptor{
		{Key: "area", Label: "存储区域", Description: "选择页面本地或会话存储。", Type: sdk.ParameterList, Default: "local", Options: storageAreaOptions()},
		{Key: "key", Label: "Key", Description: "要删除的存储键。", Type: sdk.ParameterString, Required: true, FullWidth: true},
	}, validateStorageMutation))
}
