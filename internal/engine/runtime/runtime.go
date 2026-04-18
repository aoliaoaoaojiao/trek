package runtime

import (
	"context"
	types2 "trek/internal/engine/core/types"
	"trek/internal/engine/decision"
	_ "trek/internal/engine/decision/reuse"
	perceptionfusion "trek/internal/engine/perception/fusion"
)

const ENGINE_VERSION = "1.0.0"

var engineModel *decision.Model
var observationMode = perceptionfusion.ModeXMLOnly
var defaultOrchestrator = newDefaultOrchestrator()

func GetAction(activity string, xmlDescOfGuiTree string) string {
	operate := GetActionOpt(activity, xmlDescOfGuiTree)
	if operate == nil {
		return ""
	}
	return operate.ToJSON()
}

func GetActionOpt(activity string, xmlDescOfGuiTree string) *types2.DeviceOperateWrapper {
	return GetActionOptWithInput(activity, xmlDescOfGuiTree, nil)
}

func GetActionOptWithInput(activity string, xmlDescOfGuiTree string, screenshot []byte) *types2.DeviceOperateWrapper {
	if defaultOrchestrator == nil {
		defaultOrchestrator = newDefaultOrchestrator()
	}
	operate := defaultOrchestrator.NextActionWithInput(context.Background(), decision.PerceptionInput{
		PageName:   activity,
		XMLDesc:    xmlDescOfGuiTree,
		Screenshot: screenshot,
	})
	if operate == nil {
		return nil
	}
	return operate
}

func InitAgent(agentType types2.AlgorithmType, packageName string, deviceType types2.DeviceType) {
	ensureModel(packageName)
	engineModel.AddAgent(decision.DefaultDeviceID, agentType.String(), deviceType)
	engineModel.SetPackageName(packageName)
}

// LoadResourceMapping 加载资源映射配置（主入口）。
func LoadResourceMapping(resourceMappingFilepath string) {
	ensureModel("")
	configManager := engineModel.GetConfigManager()
	if configManager != nil {
		_ = configManager.LoadResourceMapping(resourceMappingFilepath)
	}
}

// Deprecated: 请使用 LoadResourceMapping。
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

func ensureModel(packageName string) *decision.Model {
	if engineModel == nil {
		engineModel = decision.NewModel(packageName)
	}
	return engineModel
}
