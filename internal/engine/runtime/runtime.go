package runtime

import (
	"trek/internal/engine/core/model"
	types2 "trek/internal/engine/core/types"
	"trek/internal/engine/core/types/elements"
)

const ENGINE_VERSION = "1.0.0"

var engineModel *model.Model

func GetAction(activity string, xmlDescOfGuiTree string) string {
	operate := GetActionOpt(activity, xmlDescOfGuiTree)
	if operate == nil {
		return ""
	}
	return operate.ToJSON()
}

func GetActionOpt(activity string, xmlDescOfGuiTree string) *types2.DeviceOperateWrapper {
	if engineModel == nil {
		engineModel = model.NewModel(activity)
	}

	elem, err := elements.CreateAndroidElementFromXml(xmlDescOfGuiTree)
	if err != nil || elem == nil {
		return nil
	}

	return engineModel.GetOperateOpt(elem, activity, "")
}

func InitAgent(agentType types2.AlgorithmType, packageName string, deviceType types2.DeviceType) {
	if engineModel == nil {
		engineModel = model.NewModel(packageName)
	}

	engineModel.AddAgent(model.DefaultDeviceID, agentType.String(), deviceType)
	engineModel.SetPackageName(packageName)
}

func LoadResMapping(resMappingFilepath string) {
	if engineModel == nil {
		engineModel = model.NewModel("")
	}

	configManager := engineModel.GetConfigManager()
	if configManager != nil {
		_ = configManager.LoadMixResMapping(resMappingFilepath)
	}
}

func CheckPointIsInBlackRects(activity string, pointX float32, pointY float32) bool {
	if engineModel == nil {
		return false
	}

	configManager := engineModel.GetConfigManager()
	if configManager == nil {
		return false
	}

	isShield := configManager.CheckPointIsInBlackRects(activity, int(pointX), int(pointY))
	return isShield
}

func GetNativeVersion() string {
	return ENGINE_VERSION
}

func GetModel() *model.Model {
	return engineModel
}

func ResetModel() {
	engineModel = nil
}
