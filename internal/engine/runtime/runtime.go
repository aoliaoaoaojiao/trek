package runtime

import (
	"trek/internal/engine/core/types"
	engineplugin "trek/internal/engine/plugin"
)

// SetLifecycleContext 设置插件生命周期上下文，应在加载插件前调用。
func SetLifecycleContext(ctx engineplugin.LifecycleContext) {
	defaultRuntime.SetLifecycleContext(ctx)
}

// NewLifecycleContext 构造生命周期上下文。
func NewLifecycleContext(packageName string) engineplugin.LifecycleContext {
	rt := New(packageName)
	return rt.NewLifecycleContext()
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
	Action     *types.ActionCommand
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
	return defaultRuntime.SetObservationMode(mode)
}

// GetObservationMode 返回当前感知模式。
func GetObservationMode() string {
	return defaultRuntime.GetObservationMode()
}
