package browser

import "github.com/hanxuanyu/meerkit/sdk"

func tabMuteAction() actionSpec {
	return newAction("tab.mute", "标签页静音", "设置标签页的音频静音状态。", "tab", "标签页", "volume-x", "tab", "tab_required", false, []sdk.ParameterDescriptor{
		{Key: "muted", Label: "静音", Description: "开启时阻止标签页播放声音。", Type: sdk.ParameterBoolean, Default: true},
	}, func(params map[string]any) error { return validateOptionalBooleans(params, "muted") })
}
