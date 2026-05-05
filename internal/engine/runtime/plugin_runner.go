package runtime

import (
	"trek/internal/engine/core/types"
	engineplugin "trek/internal/engine/plugin"
)

type scriptPluginRunner interface {
	TransformPage(ctx engineplugin.PluginContext) (engineplugin.PageSnapshot, error)
	ResolvePageName(ctx engineplugin.PluginContext) (string, error)
	BeforeDecide(ctx engineplugin.PluginContext) (*types.ActionCommand, bool, error)
	AfterDecide(ctx engineplugin.PluginContext, cmd *types.ActionCommand) (*types.ActionCommand, bool, error)
	OnStepResult(ctx engineplugin.StepResultContext) error
	OnInit(ctx engineplugin.LifecycleContext) error
	OnDestroy(ctx engineplugin.LifecycleContext) error
}

type pluginChain struct {
	items []*engineplugin.Adapter
}

func newPluginChain(items []*engineplugin.Adapter) *pluginChain {
	normalized := make([]*engineplugin.Adapter, 0, len(items))
	for _, item := range items {
		if item != nil {
			normalized = append(normalized, item)
		}
	}
	if len(normalized) == 0 {
		return nil
	}
	return &pluginChain{items: normalized}
}

func (c *pluginChain) ResolvePageName(ctx engineplugin.PluginContext) (string, error) {
	if c == nil || len(c.items) == 0 {
		return "", nil
	}
	for _, item := range c.items {
		name, err := item.ResolvePageName(ctx)
		if err != nil {
			return "", err
		}
		if name != "" {
			return name, nil
		}
	}
	return "", nil
}

func (c *pluginChain) TransformPage(ctx engineplugin.PluginContext) (engineplugin.PageSnapshot, error) {
	if c == nil || len(c.items) == 0 {
		return ctx.Page, nil
	}
	page := ctx.Page
	for _, item := range c.items {
		nextCtx := ctx
		nextCtx.Page = page
		nextPage, err := item.TransformPage(nextCtx)
		if err != nil {
			return page, err
		}
		page = nextPage
	}
	return page, nil
}

func (c *pluginChain) BeforeDecide(ctx engineplugin.PluginContext) (*types.ActionCommand, bool, error) {
	if c == nil || len(c.items) == 0 {
		return nil, false, nil
	}
	for _, item := range c.items {
		cmd, handled, err := item.BeforeDecide(ctx)
		if err != nil {
			return nil, false, err
		}
		if handled {
			return cmd, true, nil
		}
	}
	return nil, false, nil
}

func (c *pluginChain) AfterDecide(ctx engineplugin.PluginContext, cmd *types.ActionCommand) (*types.ActionCommand, bool, error) {
	if c == nil || len(c.items) == 0 {
		return cmd, false, nil
	}
	current := cmd
	handledAny := false
	for _, item := range c.items {
		next, handled, err := item.AfterDecide(ctx, current)
		if err != nil {
			return current, handledAny, err
		}
		if handled {
			handledAny = true
			current = next
		}
	}
	return current, handledAny, nil
}

func (c *pluginChain) OnStepResult(ctx engineplugin.StepResultContext) error {
	if c == nil || len(c.items) == 0 {
		return nil
	}
	for _, item := range c.items {
		if err := item.OnStepResult(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (c *pluginChain) OnInit(ctx engineplugin.LifecycleContext) error {
	if c == nil || len(c.items) == 0 {
		return nil
	}
	for _, item := range c.items {
		if err := item.OnInit(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (c *pluginChain) OnDestroy(ctx engineplugin.LifecycleContext) error {
	if c == nil || len(c.items) == 0 {
		return nil
	}
	for _, item := range c.items {
		if err := item.OnDestroy(ctx); err != nil {
			return err
		}
	}
	return nil
}

func LoadScriptPlugin(path string) error {
	return defaultRuntime.LoadScriptPlugin(path)
}

func ClearScriptPlugin() {
	defaultRuntime.ClearScriptPlugin()
}

func HasScriptPlugin() bool {
	return defaultRuntime.HasScriptPlugin()
}

func transformPageForDecision(ctx engineplugin.PluginContext) (engineplugin.PageSnapshot, error) {
	return defaultRuntime.transformPageForDecision(ctx)
}

func resolvePageNameFromPlugin(ctx engineplugin.PluginContext) (string, error) {
	return defaultRuntime.resolvePageNameFromPlugin(ctx)
}

func beforeDecide(ctx engineplugin.PluginContext) (*types.ActionCommand, bool, error) {
	return defaultRuntime.beforeDecide(ctx)
}

func afterDecide(ctx engineplugin.PluginContext, cmd *types.ActionCommand) (*types.ActionCommand, bool, error) {
	return defaultRuntime.afterDecide(ctx, cmd)
}
