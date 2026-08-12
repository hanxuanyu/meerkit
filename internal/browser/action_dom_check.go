package browser

import "github.com/hanxuanyu/meerkit/sdk"

func domCheckAction() actionSpec {
	return newAction("dom.check", "设置选中状态", "设置 checkbox 或 radio 的选中状态。", "dom", "DOM", "square-check-big", "status", "tab_required", false, []sdk.ParameterDescriptor{
		{Key: "selector", Label: "CSS Selector", Description: "定位首个 checkbox 或 radio。", Type: sdk.ParameterCSSSelector, Required: true, FullWidth: true, SelectorCandidates: selectorCandidates(60, "input[type='checkbox']", "input[type='radio']")},
		{Key: "checked", Label: "选中", Description: "设置控件的目标选中状态。", Type: sdk.ParameterBoolean, Default: true},
	}, func(params map[string]any) error {
		if err := validateSelector(params); err != nil {
			return err
		}
		return validateOptionalBooleans(params, "checked")
	})
}
