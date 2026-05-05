package runtime

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"

	"trek/internal/engine/config"
	"trek/internal/engine/core/types"
	"trek/internal/engine/decision"
	perceptionfusion "trek/internal/engine/perception/fusion"
	engineplugin "trek/internal/engine/plugin"
	"trek/internal/scripting"
)

// Runtime 持有一次运行所需的运行时状态。
type Runtime struct {
	mu                  sync.RWMutex
	packageName         string
	engineModel         *decision.Model
	observationMode     perceptionfusion.Mode
	defaultOrchestrator *Orchestrator
	scriptPlugin        scriptPluginRunner
	lifecycleCtx        engineplugin.LifecycleContext
}

var defaultRuntime = newRuntime("")

func newRuntime(packageName string) *Runtime {
	r := &Runtime{
		packageName:     packageName,
		observationMode: perceptionfusion.ModeXMLOnly,
	}
	r.defaultOrchestrator = newOrchestratorWithModeAndModelProvider(r.observationMode, r.ensureModel)
	r.lifecycleCtx = engineplugin.LifecycleContext{
		PackageName:    packageName,
		PageSourceType: string(r.observationMode),
	}
	return r
}

// New 创建新的运行时实例。
func New(packageName string) *Runtime {
	return newRuntime(packageName)
}

// SetLifecycleContext 设置插件生命周期上下文。
func (r *Runtime) SetLifecycleContext(ctx engineplugin.LifecycleContext) {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.lifecycleCtx = ctx
	if strings.TrimSpace(ctx.PackageName) != "" {
		r.packageName = ctx.PackageName
	}
	r.mu.Unlock()
}

// NewLifecycleContext 构造生命周期上下文。
func (r *Runtime) NewLifecycleContext() engineplugin.LifecycleContext {
	if r == nil {
		return engineplugin.LifecycleContext{}
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return engineplugin.LifecycleContext{
		PackageName:    r.packageName,
		PageSourceType: string(r.observationMode),
	}
}

// ResetModel 重置模型与插件状态。
func (r *Runtime) ResetModel() {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.engineModel = nil
	r.defaultOrchestrator = newOrchestratorWithModeAndModelProvider(r.observationMode, r.ensureModel)
	oldPlugin := r.scriptPlugin
	ctx := r.lifecycleCtx
	r.scriptPlugin = nil
	r.mu.Unlock()

	if oldPlugin != nil {
		_ = oldPlugin.OnDestroy(ctx)
	}
}

// InitAgent 初始化决策代理。
func (r *Runtime) InitAgent(agentType decision.AlgorithmType, packageName string, deviceType types.DeviceType) error {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	if r.engineModel == nil {
		r.engineModel = decision.NewModel(packageName, config.GetInstance())
	}
	if _, err := r.engineModel.AddAgent(decision.DefaultDeviceID, agentType.String(), deviceType); err != nil {
		r.mu.Unlock()
		return err
	}
	r.engineModel.SetPackageName(packageName)
	r.packageName = packageName
	r.mu.Unlock()
	return nil
}

// GetModel 返回当前模型。
func (r *Runtime) GetModel() *decision.Model {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.engineModel
}

func (r *Runtime) ensureModel(packageName string) *decision.Model {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	if r.engineModel != nil {
		m := r.engineModel
		r.mu.RUnlock()
		return m
	}
	r.mu.RUnlock()

	r.mu.Lock()
	if r.engineModel == nil {
		r.engineModel = decision.NewModel(packageName, config.GetInstance())
	}
	m := r.engineModel
	r.mu.Unlock()
	return m
}

func (r *Runtime) packageNameValue() string {
	if r == nil {
		return ""
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.engineModel != nil {
		return r.engineModel.GetPackageName()
	}
	return r.packageName
}

// LoadConfigFile 加载配置文件。
func (r *Runtime) LoadConfigFile(path string) error {
	if r == nil {
		return nil
	}
	r.ensureModel("")
	if err := r.LoadScriptPlugin(path); err != nil {
		return err
	}
	if err := r.LoadPluginsFromConfig(path); err != nil {
		return err
	}
	cfg := config.GetInstance()
	if cfg == nil {
		return nil
	}
	return cfg.LoadResourceMapping(path)
}

// LoadPluginsFromConfig 从配置文件加载插件链。
func (r *Runtime) LoadPluginsFromConfig(configPath string) error {
	cfg, err := scripting.LoadStaticConfigFile(configPath)
	if err != nil {
		return err
	}
	if len(cfg.Plugins) == 0 {
		return nil
	}
	baseDir := filepath.Dir(configPath)
	adapters := make([]*engineplugin.Adapter, 0, len(cfg.Plugins))
	for _, item := range cfg.Plugins {
		path := strings.TrimSpace(item)
		if path == "" {
			continue
		}
		if !filepath.IsAbs(path) {
			path = filepath.Clean(filepath.Join(baseDir, path))
		}
		plugin, loadErr := engineplugin.LoadFile(path)
		if loadErr != nil {
			if errors.Is(loadErr, scripting.ErrPluginNotFound) {
				continue
			}
			return loadErr
		}
		adapters = append(adapters, plugin)
	}

	r.mu.Lock()
	oldPlugin := r.scriptPlugin
	ctx := r.lifecycleCtx
	chain := newPluginChain(adapters)
	r.scriptPlugin = chain
	r.mu.Unlock()

	if oldPlugin != nil {
		_ = oldPlugin.OnDestroy(ctx)
	}
	if chain != nil {
		_ = chain.OnInit(ctx)
	}
	return nil
}

// LoadScriptPlugin 加载旧单插件模式脚本。
func (r *Runtime) LoadScriptPlugin(path string) error {
	plugin, err := engineplugin.LoadFile(path)
	if err != nil {
		if errors.Is(err, scripting.ErrPluginNotFound) {
			r.mu.Lock()
			r.scriptPlugin = nil
			r.mu.Unlock()
			return nil
		}
		r.mu.Lock()
		r.scriptPlugin = nil
		r.mu.Unlock()
		return err
	}
	r.mu.Lock()
	oldPlugin := r.scriptPlugin
	ctx := r.lifecycleCtx
	r.scriptPlugin = plugin
	r.mu.Unlock()

	if oldPlugin != nil {
		_ = oldPlugin.OnDestroy(ctx)
	}
	_ = plugin.OnInit(ctx)
	return nil
}

// ClearScriptPlugin 清空脚本插件。
func (r *Runtime) ClearScriptPlugin() {
	if r == nil {
		return
	}
	r.mu.Lock()
	p := r.scriptPlugin
	ctx := r.lifecycleCtx
	r.scriptPlugin = nil
	r.mu.Unlock()
	if p != nil {
		_ = p.OnDestroy(ctx)
	}
}

// HasScriptPlugin 判断当前是否存在脚本插件。
func (r *Runtime) HasScriptPlugin() bool {
	if r == nil {
		return false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.scriptPlugin != nil
}

func (r *Runtime) transformPageForDecision(ctx engineplugin.PluginContext) (engineplugin.PageSnapshot, error) {
	r.mu.RLock()
	p := r.scriptPlugin
	r.mu.RUnlock()
	if p == nil {
		return ctx.Page, nil
	}
	return p.TransformPage(ctx)
}

func (r *Runtime) resolvePageNameFromPlugin(ctx engineplugin.PluginContext) (string, error) {
	r.mu.RLock()
	p := r.scriptPlugin
	r.mu.RUnlock()
	if p == nil {
		return "", nil
	}
	return p.ResolvePageName(ctx)
}

func (r *Runtime) beforeDecide(ctx engineplugin.PluginContext) (*types.ActionCommand, bool, error) {
	r.mu.RLock()
	p := r.scriptPlugin
	r.mu.RUnlock()
	if p == nil {
		return nil, false, nil
	}
	return p.BeforeDecide(ctx)
}

func (r *Runtime) afterDecide(ctx engineplugin.PluginContext, cmd *types.ActionCommand) (*types.ActionCommand, bool, error) {
	r.mu.RLock()
	p := r.scriptPlugin
	r.mu.RUnlock()
	if p == nil {
		return cmd, false, nil
	}
	return p.AfterDecide(ctx, cmd)
}

func (r *Runtime) ensureOrchestrator() *Orchestrator {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	orch := r.defaultOrchestrator
	r.mu.RUnlock()
	if orch != nil {
		return orch
	}
	r.mu.Lock()
	if r.defaultOrchestrator == nil {
		r.defaultOrchestrator = newOrchestratorWithModeAndModelProvider(r.observationMode, r.ensureModel)
	}
	orch = r.defaultOrchestrator
	r.mu.Unlock()
	return orch
}

func (r *Runtime) buildPluginContext(activity string, xmlDescOfGuiTree string, screenshot []byte, options ActionRequestOptions) engineplugin.PluginContext {
	page := engineplugin.PageSnapshot{
		Name: activity,
		XML:  xmlDescOfGuiTree,
	}
	if len(screenshot) > 0 {
		page.Screenshot = &engineplugin.Screenshot{
			Bytes: screenshot,
			MIME:  "image/png",
		}
	}

	r.mu.RLock()
	pkg := r.packageName
	if r.engineModel != nil {
		pkg = r.engineModel.GetPackageName()
	}
	mode := r.observationMode
	r.mu.RUnlock()

	return engineplugin.PluginContext{
		Page: page,
		Runtime: engineplugin.RuntimeContext{
			PackageName:    pkg,
			PageSourceType: string(mode),
			BlockRecovery: &engineplugin.BlockRecoveryContext{
				Requested: options.BlockRecovery,
			},
		},
	}
}

// GetActionOptWithInput 获取主决策动作。
func (r *Runtime) GetActionOptWithInput(activity string, xmlDescOfGuiTree string, screenshot []byte) *types.ActionCommand {
	return r.getActionOptWithOptions(activity, xmlDescOfGuiTree, screenshot, ActionRequestOptions{})
}

// GetBlockRecoveryActionOptWithInput 获取阻塞恢复动作。
func (r *Runtime) GetBlockRecoveryActionOptWithInput(activity string, xmlDescOfGuiTree string, screenshot []byte) *types.ActionCommand {
	return r.getActionOptWithOptions(activity, xmlDescOfGuiTree, screenshot, ActionRequestOptions{BlockRecovery: true})
}

func (r *Runtime) getActionOptWithOptions(activity string, xmlDescOfGuiTree string, screenshot []byte, options ActionRequestOptions) *types.ActionCommand {
	orch := r.ensureOrchestrator()
	if orch == nil {
		return nil
	}
	pluginCtx := r.buildPluginContext(activity, xmlDescOfGuiTree, screenshot, options)
	page, _ := r.transformPageForDecision(pluginCtx)
	pluginCtx.Page = page
	if cmd, handled, err := r.beforeDecide(pluginCtx); err == nil && handled {
		return cmd
	}
	operate := orch.NextActionWithInput(context.Background(), decision.PerceptionInput{
		PageName:   page.Name,
		XMLDesc:    page.XML,
		Screenshot: screenshot,
	})
	if operate == nil {
		return nil
	}
	if cmd, handled, err := r.afterDecide(pluginCtx, operate); err == nil && handled {
		return cmd
	}
	return operate
}

// TransformPageInfoWithInput 使用插件转换页面信息。
func (r *Runtime) TransformPageInfoWithInput(activity string, xmlDescOfGuiTree string, screenshot []byte) (string, string, error) {
	ctx := r.buildPluginContext(activity, xmlDescOfGuiTree, screenshot, ActionRequestOptions{})
	page, err := r.transformPageForDecision(ctx)
	if err != nil {
		return activity, xmlDescOfGuiTree, err
	}
	return page.Name, page.XML, nil
}

// ResolvePageNameWithInput 调用插件页面名钩子。
func (r *Runtime) ResolvePageNameWithInput(activity string, xmlDescOfGuiTree string, screenshot []byte) (string, error) {
	ctx := r.buildPluginContext(activity, xmlDescOfGuiTree, screenshot, ActionRequestOptions{})
	return r.resolvePageNameFromPlugin(ctx)
}

// OnStepResult 回传步骤结果。
func (r *Runtime) OnStepResult(input StepResultInput) error {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	p := r.scriptPlugin
	r.mu.RUnlock()
	if p == nil {
		return nil
	}
	before := pageSnapshotFromInput(input.Before)
	ctx := engineplugin.StepResultContext{
		PluginContext: engineplugin.PluginContext{
			Page: before,
			Runtime: engineplugin.RuntimeContext{
				PackageName: r.packageNameValue(),
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
	return p.OnStepResult(ctx)
}

// SetObservationMode 设置感知模式。
func (r *Runtime) SetObservationMode(mode string) error {
	if r == nil {
		return nil
	}
	parsed, err := perceptionfusion.ParseMode(mode)
	if err != nil {
		return err
	}
	r.mu.Lock()
	r.observationMode = parsed
	r.defaultOrchestrator = newOrchestratorWithModeAndModelProvider(r.observationMode, r.ensureModel)
	r.mu.Unlock()
	return nil
}

// GetObservationMode 获取当前感知模式。
func (r *Runtime) GetObservationMode() string {
	if r == nil {
		return string(perceptionfusion.ModeXMLOnly)
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return string(r.observationMode)
}

// CheckPointIsInBlackRects 检查点是否命中黑名单区域。
func (r *Runtime) CheckPointIsInBlackRects(activity string, pointX float32, pointY float32) bool {
	cfg := config.GetInstance()
	if cfg == nil {
		return false
	}
	return cfg.CheckPointIsInBlackRects(activity, int(pointX), int(pointY))
}

// GetNativeVersion 返回运行时版本。
func (r *Runtime) GetNativeVersion() string {
	return ENGINE_VERSION
}
