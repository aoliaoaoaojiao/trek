package runtime

import (
	"trek/internal/engine/config"
	"trek/internal/engine/decision"
	"trek/internal/engine/decision/shared/types"
)

const ENGINE_VERSION = "1.0.0"

func InitAgent(agentType decision.AlgorithmType, packageName string, deviceType types.DeviceType) {
	ensureModel(packageName)
	engineModel.AddAgent(decision.DefaultDeviceID, agentType.String(), deviceType)
	engineModel.SetPackageName(packageName)
}

func GetModel() *decision.Model {
	return engineModel
}

func ResetModel() {
	engineModel = nil
	defaultOrchestrator = newDefaultOrchestrator()
	ClearScriptPlugin()
}

func GetNativeVersion() string {
	return ENGINE_VERSION
}

func ensureModel(packageName string) *decision.Model {
	if engineModel == nil {
		engineModel = decision.NewModel(packageName, config.GetInstance())
	}
	return engineModel
}

func packageName() string {
	if engineModel == nil {
		return ""
	}
	return engineModel.GetPackageName()
}
