package runtime

import (
	"errors"
	"path/filepath"
	"strings"
	"trek/internal/engine/config"
	engineplugin "trek/internal/engine/plugin"
	"trek/internal/scripting"
)

func LoadConfigFile(resourceMappingFilepath string) error {
	ensureModel("")
	if err := LoadScriptPlugin(resourceMappingFilepath); err != nil {
		return err
	}
	if err := LoadPluginsFromConfig(resourceMappingFilepath); err != nil {
		return err
	}
	cfg := config.GetInstance()
	if cfg == nil {
		return nil
	}
	return cfg.LoadResourceMapping(resourceMappingFilepath)
}

func LoadPluginsFromConfig(configPath string) error {
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
	// 替换前先销毁旧插件
	mu.Lock()
	oldPlugin := scriptPlugin
	ctx := lifecycleCtx
	chain := newPluginChain(adapters)
	scriptPlugin = chain
	mu.Unlock()

	if oldPlugin != nil {
		_ = oldPlugin.OnDestroy(ctx)
	}
	if chain != nil {
		_ = chain.OnInit(ctx)
	}
	return nil
}

// LoadResourceMapping 加载资源配置（主入口）。
func LoadResourceMapping(resourceMappingFilepath string) {
	_ = LoadConfigFile(resourceMappingFilepath)
}

// Deprecated: 请使用 LoadResourceMapping。
func LoadResMapping(resMappingFilepath string) {
	LoadResourceMapping(resMappingFilepath)
}

func CheckPointIsInBlackRects(activity string, pointX float32, pointY float32) bool {
	cfg := config.GetInstance()
	if cfg == nil {
		return false
	}
	return cfg.CheckPointIsInBlackRects(activity, int(pointX), int(pointY))
}
