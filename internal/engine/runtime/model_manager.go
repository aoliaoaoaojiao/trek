package runtime

import (
	"fmt"

	"trek/internal/engine/config"
	"trek/internal/engine/decision"
	"trek/internal/engine/decision/shared/types"
)

const ENGINE_VERSION = "1.0.0"

func InitAgent(agentType decision.AlgorithmType, packageName string, deviceType types.DeviceType) error {
	ensureModel(packageName)
	if _, err := engineModel.AddAgent(decision.DefaultDeviceID, agentType.String(), deviceType); err != nil {
		return fmt.Errorf("初始化决策代理失败: %w", err)
	}
	engineModel.SetPackageName(packageName)
	return nil
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
