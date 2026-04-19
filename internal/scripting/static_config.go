package scripting

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dop251/goja"
)

type StaticConfig struct {
	ResMapping map[string]string
	BlackRects map[string][][4]int
	SkipAll    bool
	PageSource string
	TouchMode  string
	UIA        StaticUIAConfig
	Poco       StaticPocoConfig
	Log        StaticLogConfig
}

type StaticLogConfig struct {
	FileLevel string
}

type StaticUIAConfig struct {
	ServerPort int
}

type StaticPocoConfig struct {
	Engine string
	Port   int
}

func LoadStaticConfigFile(path string) (StaticConfig, error) {
	if strings.ToLower(filepath.Ext(path)) != ".js" {
		return StaticConfig{}, fmt.Errorf("脚本配置仅支持 .js: %s", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return StaticConfig{}, err
	}
	return LoadStaticConfig(string(data))
}

func LoadStaticConfig(source string) (StaticConfig, error) {
	cfg := StaticConfig{
		ResMapping: make(map[string]string),
		BlackRects: make(map[string][][4]int),
	}
	vm := goja.New()
	if _, err := vm.RunString(source); err != nil {
		return cfg, fmt.Errorf("执行 goja 配置脚本失败: %w", err)
	}

	value := vm.Get("config")
	if isEmptyJSValue(value) {
		value = vm.Get("CONFIG")
	}
	if isEmptyJSValue(value) {
		return cfg, nil
	}
	obj := value.ToObject(vm)

	if resMappingValue := obj.Get("res_mapping"); !isEmptyJSValue(resMappingValue) {
		resMappingObj := resMappingValue.ToObject(vm)
		for _, key := range resMappingObj.Keys() {
			cfg.ResMapping[key] = resMappingObj.Get(key).String()
		}
	}

	if blackRectsValue := obj.Get("black_rects"); !isEmptyJSValue(blackRectsValue) {
		blackRectsObj := blackRectsValue.ToObject(vm)
		for _, pageName := range blackRectsObj.Keys() {
			rectsObj := blackRectsObj.Get(pageName).ToObject(vm)
			rects := make([][4]int, 0, len(rectsObj.Keys()))
			for _, rectKey := range rectsObj.Keys() {
				rectObj := rectsObj.Get(rectKey).ToObject(vm)
				rectKeys := rectObj.Keys()
				if len(rectKeys) != 4 {
					return cfg, fmt.Errorf("black_rects[%s][%s] 长度必须为4", pageName, rectKey)
				}
				var rect [4]int
				for i, key := range rectKeys {
					val, err := intFromJSValue(rectObj.Get(key))
					if err != nil {
						return cfg, fmt.Errorf("black_rects[%s][%s][%d] 非法: %w", pageName, rectKey, i, err)
					}
					rect[i] = val
				}
				rects = append(rects, rect)
			}
			cfg.BlackRects[pageName] = rects
		}
	}

	if skipValue := obj.Get("skip_all_actions_from_model"); !isEmptyJSValue(skipValue) {
		cfg.SkipAll = skipValue.ToBoolean()
	}
	if pageSourceValue := obj.Get("page_source"); !isEmptyJSValue(pageSourceValue) {
		cfg.PageSource = strings.TrimSpace(pageSourceValue.String())
	}
	if pageSourceValue := obj.Get("pageSource"); cfg.PageSource == "" && !isEmptyJSValue(pageSourceValue) {
		cfg.PageSource = strings.TrimSpace(pageSourceValue.String())
	}
	if touchModeValue := obj.Get("touch_mode"); !isEmptyJSValue(touchModeValue) {
		cfg.TouchMode = strings.TrimSpace(touchModeValue.String())
	}
	if touchModeValue := obj.Get("touchMode"); cfg.TouchMode == "" && !isEmptyJSValue(touchModeValue) {
		cfg.TouchMode = strings.TrimSpace(touchModeValue.String())
	}
	if uiaValue := obj.Get("uia"); !isEmptyJSValue(uiaValue) {
		uiaObj := uiaValue.ToObject(vm)
		if serverPortValue := uiaObj.Get("server_port"); !isEmptyJSValue(serverPortValue) {
			serverPort, err := intFromJSValue(serverPortValue)
			if err != nil {
				return cfg, fmt.Errorf("uia.server_port 非法: %w", err)
			}
			cfg.UIA.ServerPort = serverPort
		}
		if serverPortValue := uiaObj.Get("serverPort"); cfg.UIA.ServerPort == 0 && !isEmptyJSValue(serverPortValue) {
			serverPort, err := intFromJSValue(serverPortValue)
			if err != nil {
				return cfg, fmt.Errorf("uia.serverPort 非法: %w", err)
			}
			cfg.UIA.ServerPort = serverPort
		}
	}
	if pocoValue := obj.Get("poco"); !isEmptyJSValue(pocoValue) {
		pocoObj := pocoValue.ToObject(vm)
		if engineValue := pocoObj.Get("engine"); !isEmptyJSValue(engineValue) {
			cfg.Poco.Engine = strings.TrimSpace(engineValue.String())
		}
		if portValue := pocoObj.Get("port"); !isEmptyJSValue(portValue) {
			port, err := intFromJSValue(portValue)
			if err != nil {
				return cfg, fmt.Errorf("poco.port 非法: %w", err)
			}
			cfg.Poco.Port = port
		}
	}
	if logValue := obj.Get("log"); !isEmptyJSValue(logValue) {
		logObj := logValue.ToObject(vm)
		if fileLevelValue := logObj.Get("file_level"); !isEmptyJSValue(fileLevelValue) {
			cfg.Log.FileLevel = strings.TrimSpace(fileLevelValue.String())
		}
		if fileLevelValue := logObj.Get("fileLevel"); cfg.Log.FileLevel == "" && !isEmptyJSValue(fileLevelValue) {
			cfg.Log.FileLevel = strings.TrimSpace(fileLevelValue.String())
		}
	}
	return cfg, nil
}

func intFromJSValue(v goja.Value) (int, error) {
	if isEmptyJSValue(v) {
		return 0, errors.New("值不能为空")
	}
	f := v.ToFloat()
	if f != float64(int(f)) {
		return 0, fmt.Errorf("值必须是整数: %v", f)
	}
	return int(f), nil
}
