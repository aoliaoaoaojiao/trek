package runtime

import (
	"context"
	"errors"
	"trek/internal/engine/decision"
	_ "trek/internal/engine/decision/reuse"
	_ "trek/internal/engine/decision/uctbandit"
	types2 "trek/internal/engine/decision/shared/types"
	perceptionfusion "trek/internal/engine/perception/fusion"
	engineplugin "trek/internal/engine/plugin"
	"trek/internal/scripting"
)

const ENGINE_VERSION = "1.0.0"

type ActionRequestOptions struct {
	BlockRecovery bool
}

type PageSnapshotInput struct {
	PageName   string
	XML        string
	Screenshot []byte
}

type StepResultInput struct {
	Step       int
	Action     *types2.ActionCommand
	Success    bool
	Error      string
	DurationMs int64
	Crash      bool
	ANR        bool
	Before     PageSnapshotInput
	After      *PageSnapshotInput
}

var engineModel *decision.Model
var observationMode = perceptionfusion.ModeXMLOnly
var defaultOrchestrator = newDefaultOrchestrator()
var scriptPlugin *engineplugin.Adapter

func GetAction(activity string, xmlDescOfGuiTree string) string {
	operate := GetActionOpt(activity, xmlDescOfGuiTree)
	if operate == nil {
		return ""
	}
	return operate.ToJSON()
}

func GetActionOpt(activity string, xmlDescOfGuiTree string) *types2.ActionCommand {
	return GetActionOptWithInput(activity, xmlDescOfGuiTree, nil)
}

func GetActionOptWithInput(activity string, xmlDescOfGuiTree string, screenshot []byte) *types2.ActionCommand {
	return getActionOptWithOptions(activity, xmlDescOfGuiTree, screenshot, ActionRequestOptions{})
}

func GetBlockRecoveryActionOptWithInput(activity string, xmlDescOfGuiTree string, screenshot []byte) *types2.ActionCommand {
	return getActionOptWithOptions(activity, xmlDescOfGuiTree, screenshot, ActionRequestOptions{
		BlockRecovery: true,
	})
}

func getActionOptWithOptions(activity string, xmlDescOfGuiTree string, screenshot []byte, options ActionRequestOptions) *types2.ActionCommand {
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

// TransformPageInfoWithInput 浣跨敤閰嶇疆鑴氭湰鏀归€犻〉闈俊鎭苟杩斿洖鏂扮粨鏋滐紙鏀寔鎴浘杈撳叆锛夈€?
func TransformPageInfoWithInput(activity string, xmlDescOfGuiTree string, screenshot []byte) (string, string, error) {
	ctx := buildPluginContext(activity, xmlDescOfGuiTree, screenshot, ActionRequestOptions{})
	page, err := transformPageForDecision(ctx)
	if err != nil {
		return activity, xmlDescOfGuiTree, err
	}
	return page.Name, page.XML, nil
}

func LoadScriptPlugin(path string) error {
	plugin, err := engineplugin.LoadFile(path)
	if err != nil {
		if errors.Is(err, scripting.ErrPluginNotFound) {
			scriptPlugin = nil
			return nil
		}
		scriptPlugin = nil
		return err
	}
	scriptPlugin = plugin
	return nil
}

func ClearScriptPlugin() {
	scriptPlugin = nil
}

func HasScriptPlugin() bool {
	return scriptPlugin != nil
}

func transformPageForDecision(ctx engineplugin.PluginContext) (engineplugin.PageSnapshot, error) {
	if scriptPlugin == nil {
		return ctx.Page, nil
	}
	return scriptPlugin.TransformPage(ctx)
}

func beforeDecide(ctx engineplugin.PluginContext) (*types2.ActionCommand, bool, error) {
	if scriptPlugin == nil {
		return nil, false, nil
	}
	return scriptPlugin.BeforeDecide(ctx)
}

func afterDecide(ctx engineplugin.PluginContext, cmd *types2.ActionCommand) (*types2.ActionCommand, bool, error) {
	if scriptPlugin == nil {
		return cmd, false, nil
	}
	return scriptPlugin.AfterDecide(ctx, cmd)
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

func LoadConfigFile(resourceMappingFilepath string) error {
	ensureModel("")
	if err := LoadScriptPlugin(resourceMappingFilepath); err != nil {
		return err
	}
	configManager := engineModel.GetConfigManager()
	if configManager == nil {
		return nil
	}
	return configManager.LoadResourceMapping(resourceMappingFilepath)
}

func OnStepResult(input StepResultInput) error {
	if scriptPlugin == nil {
		return nil
	}
	before := pageSnapshotFromInput(input.Before)
	ctx := engineplugin.StepResultContext{
		PluginContext: engineplugin.PluginContext{
			Page: before,
			Runtime: engineplugin.RuntimeContext{
				PackageName: packageName(),
			},
		},
		Result: engineplugin.StepResult{
			Step:       input.Step,
			Action:     engineplugin.FromActionCommand(input.Action),
			Success:    input.Success,
			Error:      input.Error,
			DurationMs: input.DurationMs,
			Crash:      input.Crash,
			ANR:        input.ANR,
			Before:     before,
			After:      pageSnapshotPtrFromInput(input.After),
		},
	}
	return scriptPlugin.OnStepResult(ctx)
}

func InitAgent(agentType decision.AlgorithmType, packageName string, deviceType types2.DeviceType) {
	ensureModel(packageName)
	engineModel.AddAgent(decision.DefaultDeviceID, agentType.String(), deviceType)
	engineModel.SetPackageName(packageName)
}

// LoadResourceMapping 鍔犺浇璧勬簮鏄犲皠閰嶇疆锛堜富鍏ュ彛锛夈€?
func LoadResourceMapping(resourceMappingFilepath string) {
	_ = LoadConfigFile(resourceMappingFilepath)
}

// Deprecated: 璇蜂娇鐢?LoadResourceMapping銆?
func LoadResMapping(resMappingFilepath string) {
	LoadResourceMapping(resMappingFilepath)
}

func CheckPointIsInBlackRects(activity string, pointX float32, pointY float32) bool {
	if engineModel == nil {
		return false
	}
	configManager := engineModel.GetConfigManager()
	if configManager == nil {
		return false
	}
	return configManager.CheckPointIsInBlackRects(activity, int(pointX), int(pointY))
}

func GetNativeVersion() string {
	return ENGINE_VERSION
}

func GetModel() *decision.Model {
	return engineModel
}

func ResetModel() {
	engineModel = nil
	defaultOrchestrator = newDefaultOrchestrator()
	ClearScriptPlugin()
}

// SetObservationMode 璁剧疆鎰熺煡妯″紡锛歺ml-only / image-only / hybrid銆?
func SetObservationMode(mode string) error {
	parsed, err := perceptionfusion.ParseMode(mode)
	if err != nil {
		return err
	}
	observationMode = parsed
	defaultOrchestrator = newOrchestratorWithMode(observationMode)
	return nil
}

// GetObservationMode 杩斿洖褰撳墠鎰熺煡妯″紡銆?
func GetObservationMode() string {
	return string(observationMode)
}

func ensureModel(packageName string) *decision.Model {
	if engineModel == nil {
		engineModel = decision.NewModel(packageName)
	}
	return engineModel
}

func pageSnapshotFromInput(input PageSnapshotInput) engineplugin.PageSnapshot {
	page := engineplugin.PageSnapshot{
		Name: input.PageName,
		XML:  input.XML,
	}
	if len(input.Screenshot) > 0 {
		page.Screenshot = &engineplugin.Screenshot{
			Bytes: append([]byte(nil), input.Screenshot...),
			MIME:  "image/png",
		}
	}
	return page
}

func pageSnapshotPtrFromInput(input *PageSnapshotInput) *engineplugin.PageSnapshot {
	if input == nil {
		return nil
	}
	page := pageSnapshotFromInput(*input)
	return &page
}

func packageName() string {
	if engineModel == nil {
		return ""
	}
	return engineModel.GetPackageName()
}
