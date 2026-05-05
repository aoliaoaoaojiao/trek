package runtime

import "trek/internal/engine/core/types"

func GetAction(activity string, xmlDescOfGuiTree string) string {
	operate := GetActionOpt(activity, xmlDescOfGuiTree)
	if operate == nil {
		return ""
	}
	return operate.ToJSON()
}

func GetActionOpt(activity string, xmlDescOfGuiTree string) *types.ActionCommand {
	return GetActionOptWithInput(activity, xmlDescOfGuiTree, nil)
}

func GetActionOptWithInput(activity string, xmlDescOfGuiTree string, screenshot []byte) *types.ActionCommand {
	return defaultRuntime.GetActionOptWithInput(activity, xmlDescOfGuiTree, screenshot)
}

func GetBlockRecoveryActionOptWithInput(activity string, xmlDescOfGuiTree string, screenshot []byte) *types.ActionCommand {
	return defaultRuntime.GetBlockRecoveryActionOptWithInput(activity, xmlDescOfGuiTree, screenshot)
}

func getActionOptWithOptions(activity string, xmlDescOfGuiTree string, screenshot []byte, options ActionRequestOptions) *types.ActionCommand {
	return defaultRuntime.getActionOptWithOptions(activity, xmlDescOfGuiTree, screenshot, options)
}

// TransformPageInfoWithInput 使用配置脚本改造页面信息并返回新结果（支持截图输入）。
func TransformPageInfoWithInput(activity string, xmlDescOfGuiTree string, screenshot []byte) (string, string, error) {
	return defaultRuntime.TransformPageInfoWithInput(activity, xmlDescOfGuiTree, screenshot)
}

// ResolvePageNameWithInput 调用插件的 resolvePageName 钩子，返回自定义页面名。
func ResolvePageNameWithInput(activity string, xmlDescOfGuiTree string, screenshot []byte) (string, error) {
	return defaultRuntime.ResolvePageNameWithInput(activity, xmlDescOfGuiTree, screenshot)
}
