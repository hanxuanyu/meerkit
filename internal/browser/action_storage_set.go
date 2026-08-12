package browser

import "github.com/hanxuanyu/meerkit/sdk"

func storageSetAction() actionSpec {
	return sensitiveAction(newAction("storage.set", "写入页面存储", "写入 localStorage 或 sessionStorage。", "storage", "页面存储", "database-zap", "status", "tab_required", true, []sdk.ParameterDescriptor{
		{Key: "area", Label: "存储区域", Description: "选择页面本地或会话存储。", Type: sdk.ParameterList, Default: "local", Options: storageAreaOptions()},
		{Key: "key", Label: "Key", Description: "要写入的存储键。", Type: sdk.ParameterString, Required: true, FullWidth: true},
		{Key: "value", Label: "Value", Description: "要写入的字符串值。", Type: sdk.ParameterText, Required: true, Secret: true, FullWidth: true, Rows: 4},
	}, validateStorageMutation))
}
