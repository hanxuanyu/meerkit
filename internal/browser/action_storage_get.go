package browser

import "github.com/hanxuanyu/meerkit/sdk"

func storageGetAction() actionSpec {
	return sensitiveAction(newAction("storage.get", "读取页面存储", "读取 localStorage 或 sessionStorage。", "storage", "页面存储", "database", "storage", "tab_required", false, []sdk.ParameterDescriptor{
		{Key: "area", Label: "存储区域", Description: "选择页面本地或会话存储。", Type: sdk.ParameterList, Default: "local", Options: storageAreaOptions()},
		{Key: "key", Label: "Key", Description: "留空时读取该区域的所有项目。", Type: sdk.ParameterString, FullWidth: true},
		{Key: "max_value_length", Label: "单值上限", Description: "单个值超出此 UTF-8 字节数时截断。", Type: sdk.ParameterInteger, Default: 65536, Minimum: number(1), Maximum: number(1048576), Unit: "byte"},
	}, validateStorageGet))
}
