package browser

import "github.com/hanxuanyu/meerkit/sdk"

func pageScreenshotAction() actionSpec {
	return newAction("page.screenshot", "页面截图", "使用 CDP 截取指定标签页。", "page", "页面", "camera", "screenshot", "tab_required", false, []sdk.ParameterDescriptor{
		{Key: "format", Label: "图片格式", Description: "PNG 无损；长页面建议使用 JPEG 或 WebP。", Type: sdk.ParameterList, Default: "png", Options: []sdk.ParameterOption{{Value: "png", Label: "PNG"}, {Value: "jpeg", Label: "JPEG"}, {Value: "webp", Label: "WebP"}}},
		{Key: "quality", Label: "图片质量", Description: "JPEG/WebP 编码质量，越高体积越大。", Type: sdk.ParameterInteger, Default: 90, Minimum: number(1), Maximum: number(100), VisibleWhen: []sdk.ParameterCondition{{Field: "format", Operator: "in", Value: []string{"jpeg", "webp"}}}},
		{Key: "full_page", Label: "完整页面", Description: "捕获视口外内容；长页面耗时和体积更大。", Type: sdk.ParameterBoolean, Default: false},
	}, validateScreenshot)
}
