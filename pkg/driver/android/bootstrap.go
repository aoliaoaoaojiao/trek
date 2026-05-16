package android

import (
	"fmt"
	"strings"

	"trek/pkg/driver/common/page/poco"
)

// DriverBootstrapConfig 描述构建设备驱动所需的最小配置集合。
type DriverBootstrapConfig struct {
	PageSource    string
	TouchMode     string
	UIAServerPort int
	PocoEngine    string
	PocoPort      int
}

// ResolvePageSourceType 解析页面源类型，并补齐默认值。
func ResolvePageSourceType(value string) (string, error) {
	pageSource := strings.TrimSpace(value)
	if pageSource == "" {
		pageSource = "uia"
	}
	switch strings.ToLower(pageSource) {
	case "uia":
		return "uia", nil
	case "poco":
		return "poco", nil
	case "screenshot":
		return "screenshot", nil
	default:
		return "", fmt.Errorf("不支持的页面源类型: %s（可选: uia, poco, screenshot）", pageSource)
	}
}

// ResolveTouchMode 解析触控模式，并同时返回标准化后的模式文本与驱动枚举。
func ResolveTouchMode(value string) (string, TouchType, error) {
	touchMode := strings.TrimSpace(value)
	if touchMode == "" {
		touchMode = "motion"
	}
	switch strings.ToLower(touchMode) {
	case "motion":
		return "motion", TouchTypeMotion, nil
	case "uia":
		return "uia", TouchTypeUIA, nil
	case "adb":
		return "adb", TouchTypeADB, nil
	default:
		return "", "", fmt.Errorf("不支持的触控模式: %s（可选: motion, uia, adb）", touchMode)
	}
}

// BuildDriverOptions 根据统一配置构建 Android 驱动选项。
func BuildDriverOptions(cfg DriverBootstrapConfig, pageSourceType string, touchType TouchType) ([]AndroidDriverOption, error) {
	options := []AndroidDriverOption{
		WithTouch(touchType),
	}

	if cfg.UIAServerPort > 0 {
		options = append(options, WithUIAServerPort(cfg.UIAServerPort))
	}

	if strings.EqualFold(pageSourceType, "poco") {
		engineText := strings.TrimSpace(cfg.PocoEngine)
		if engineText == "" {
			return nil, fmt.Errorf("使用 poco 页面源时必须指定 Poco 引擎")
		}
		engine, err := ParsePocoEngine(engineText)
		if err != nil {
			return nil, err
		}

		pocoPort := cfg.PocoPort
		if pocoPort <= 0 {
			pocoPort = engine.GetDefaultPort()
		}
		if pocoPort <= 0 {
			return nil, fmt.Errorf("Poco 端口无效，请显式指定")
		}
		options = append(options, WithPoco(engine, pocoPort))
	}

	return options, nil
}

// ParsePocoEngine 将字符串解析为 Poco 引擎类型。
func ParsePocoEngine(text string) (poco.Engine, error) {
	raw := strings.TrimSpace(text)
	normalized := strings.ToUpper(strings.ReplaceAll(strings.ReplaceAll(raw, "-", "_"), " ", "_"))
	switch normalized {
	case string(poco.Unity3d), "UNITY", "UNITY3D":
		return poco.Unity3d, nil
	case string(poco.UE4):
		return poco.UE4, nil
	case string(poco.Cocos2dxJs), "COCOS2DX_JS", "COCOS_JS":
		return poco.Cocos2dxJs, nil
	case string(poco.CocosCreator), "COCOS_CREATOR3D":
		return poco.CocosCreator, nil
	case string(poco.Egret):
		return poco.Egret, nil
	case string(poco.Cocos2dxLua), "COCOS2DX_LUA":
		return poco.Cocos2dxLua, nil
	case string(poco.Cocos2dxCPlus), "COCOS2DX_C++", "COCOS2DX_CPLUS", "COCOS2DX_CPP":
		return poco.Cocos2dxCPlus, nil
	default:
		return "", fmt.Errorf("不支持的 Poco 引擎: %s", raw)
	}
}
