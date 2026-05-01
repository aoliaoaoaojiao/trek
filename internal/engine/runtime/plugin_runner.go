package runtime

import (
	"errors"
	"trek/internal/engine/decision/shared/types"
	engineplugin "trek/internal/engine/plugin"
	"trek/internal/scripting"
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
	plugin, err := engineplugin.LoadFile(path)
	if err != nil {
		if errors.Is(err, scripting.ErrPluginNotFound) {
			mu.Lock()
			scriptPlugin = nil
			mu.Unlock()
			return nil
		}
		mu.Lock()
		scriptPlugin = nil
		mu.Unlock()
		return err
	}
	mu.Lock()
	if scriptPlugin != nil {
		_ = scriptPlugin.OnDestroy(lifecycleCtx)
	}
	scriptPlugin = plugin
	ctx := lifecycleCtx
	mu.Unlock()
	_ = plugin.OnInit(ctx)
	return nil
}

func ClearScriptPlugin() {
	mu.Lock()
	if scriptPlugin != nil {
		p := scriptPlugin
		ctx := lifecycleCtx
		scriptPlugin = nil
		mu.Unlock()
		_ = p.OnDestroy(ctx)
		return
	}
	mu.Unlock()
}

func HasScriptPlugin() bool {
	mu.RLock()
	defer mu.RUnlock()
	return scriptPlugin != nil
}

func transformPageForDecision(ctx engineplugin.PluginContext) (engineplugin.PageSnapshot, error) {
	mu.RLock()
	p := scriptPlugin
	mu.RUnlock()
	if p == nil {
		return ctx.Page, nil
	}
	return p.TransformPage(ctx)
}

func resolvePageNameFromPlugin(ctx engineplugin.PluginContext) (string, error) {
	mu.RLock()
	p := scriptPlugin
	mu.RUnlock()
	if p == nil {
		return "", nil
	}
	return p.ResolvePageName(ctx)
}

func beforeDecide(ctx engineplugin.PluginContext) (*types.ActionCommand, bool, error) {
	mu.RLock()
	p := scriptPlugin
	mu.RUnlock()
	if p == nil {
		return nil, false, nil
	}
	return p.BeforeDecide(ctx)
}

func afterDecide(ctx engineplugin.PluginContext, cmd *types.ActionCommand) (*types.ActionCommand, bool, error) {
	mu.RLock()
	p := scriptPlugin
	mu.RUnlock()
	if p == nil {
		return cmd, false, nil
	}
	return p.AfterDecide(ctx, cmd)
}
