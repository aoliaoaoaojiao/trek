package runtime

import (
	"fmt"

	"trek/internal/engine/core/types"
	"trek/internal/engine/decision"
)

const ENGINE_VERSION = "1.0.0"

func InitAgent(agentType decision.AlgorithmType, packageName string, deviceType types.DeviceType) error {
	if err := defaultRuntime.InitAgent(agentType, packageName, deviceType); err != nil {
		return fmt.Errorf("初始化决策代理失败: %w", err)
	}
	return nil
}

func GetModel() *decision.Model {
	return defaultRuntime.GetModel()
}

func ResetModel() {
	defaultRuntime.ResetModel()
}

func GetNativeVersion() string {
	return defaultRuntime.GetNativeVersion()
}

func ensureModel(packageName string) *decision.Model {
	return defaultRuntime.ensureModel(packageName)
}

func packageName() string {
	return defaultRuntime.packageNameValue()
}
