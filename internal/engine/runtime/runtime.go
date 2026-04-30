package runtime

import (
	"trek/internal/engine/decision"
	types2 "trek/internal/engine/decision/shared/types"
	perceptionfusion "trek/internal/engine/perception/fusion"
	engineplugin "trek/internal/engine/plugin"
)

var engineModel *decision.Model
var observationMode = perceptionfusion.ModeXMLOnly
var defaultOrchestrator = newDefaultOrchestrator()
var scriptPlugin scriptPluginRunner
var lifecycleCtx engineplugin.LifecycleContext

// SetLifecycleContext 设置插件生命周期上下文，应在加载插件前调用。
func SetLifecycleContext(ctx engineplugin.LifecycleContext) {
	lifecycleCtx = ctx
}

// NewLifecycleContext 构造生命周期上下文。
func NewLifecycleContext(packageName string) engineplugin.LifecycleContext {
	return engineplugin.LifecycleContext{
		PackageName:    packageName,
		PageSourceType: string(observationMode),
	}
}

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

// SetObservationMode 设置感知模式：xml-only / image-only / hybrid。
func SetObservationMode(mode string) error {
	parsed, err := perceptionfusion.ParseMode(mode)
	if err != nil {
		return err
	}
	observationMode = parsed
	defaultOrchestrator = newOrchestratorWithMode(observationMode)
	return nil
}

// GetObservationMode 返回当前感知模式。
func GetObservationMode() string {
	return string(observationMode)
}
