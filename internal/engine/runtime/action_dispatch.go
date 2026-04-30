package runtime

import (
	"context"
	"trek/internal/engine/decision"
	"trek/internal/engine/decision/shared/types"
	engineplugin "trek/internal/engine/plugin"
)

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
	return getActionOptWithOptions(activity, xmlDescOfGuiTree, screenshot, ActionRequestOptions{})
}

func GetBlockRecoveryActionOptWithInput(activity string, xmlDescOfGuiTree string, screenshot []byte) *types.ActionCommand {
	return getActionOptWithOptions(activity, xmlDescOfGuiTree, screenshot, ActionRequestOptions{
		BlockRecovery: true,
	})
}

func getActionOptWithOptions(activity string, xmlDescOfGuiTree string, screenshot []byte, options ActionRequestOptions) *types.ActionCommand {
	if defaultOrchestrator == nil {
		defaultOrchestrator = newDefaultOrchestrator()
	}
	pluginCtx := buildPluginContext(activity, xmlDescOfGuiTree, screenshot, options)
	page, _ := transformPageForDecision(pluginCtx)
	pluginCtx.Page = page
	if cmd, handled, err := beforeDecide(pluginCtx); err == nil && handled {
		return cmd
	}
	operate := defaultOrchestrator.NextActionWithInput(context.Background(), decision.PerceptionInput{
		PageName:   page.Name,
		XMLDesc:    page.XML,
		Screenshot: screenshot,
	})
	if operate == nil {
		return nil
	}
	if cmd, handled, err := afterDecide(pluginCtx, operate); err == nil && handled {
		return cmd
	}
	return operate
}

// TransformPageInfoWithInput 使用配置脚本改造页面信息并返回新结果（支持截图输入）。
func TransformPageInfoWithInput(activity string, xmlDescOfGuiTree string, screenshot []byte) (string, string, error) {
	ctx := buildPluginContext(activity, xmlDescOfGuiTree, screenshot, ActionRequestOptions{})
	page, err := transformPageForDecision(ctx)
	if err != nil {
		return activity, xmlDescOfGuiTree, err
	}
	return page.Name, page.XML, nil
}

func buildPluginContext(activity string, xmlDescOfGuiTree string, screenshot []byte, options ActionRequestOptions) engineplugin.PluginContext {
	page := engineplugin.PageSnapshot{
		Name: activity,
		XML:  xmlDescOfGuiTree,
	}
	if len(screenshot) > 0 {
		page.Screenshot = &engineplugin.Screenshot{
			Bytes: append([]byte(nil), screenshot...),
			MIME:  "image/png",
		}
	}
	packageName := ""
	if engineModel != nil {
		packageName = engineModel.GetPackageName()
	}
	return engineplugin.PluginContext{
		Page: page,
		Runtime: engineplugin.RuntimeContext{
			PackageName:    packageName,
			PageSourceType: string(observationMode),
			BlockRecovery: &engineplugin.BlockRecoveryContext{
				Requested: options.BlockRecovery,
			},
		},
	}
}
