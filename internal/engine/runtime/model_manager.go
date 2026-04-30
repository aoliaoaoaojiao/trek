package runtime

import (
	"fmt"

	"trek/internal/engine/config"
	"trek/internal/engine/decision"
	"trek/internal/engine/decision/shared/types"
)

const ENGINE_VERSION = "1.0.0"

func InitAgent(agentType decision.AlgorithmType, packageName string, deviceType types.DeviceType) error {
	mu.Lock()
	if engineModel == nil {
		engineModel = decision.NewModel(packageName, config.GetInstance())
	}
	if _, err := engineModel.AddAgent(decision.DefaultDeviceID, agentType.String(), deviceType); err != nil {
		mu.Unlock()
		return fmt.Errorf("初始化决策代理失败: %w", err)
	}
	engineModel.SetPackageName(packageName)
	mu.Unlock()
	return nil
}

func GetModel() *decision.Model {
	mu.RLock()
	defer mu.RUnlock()
	return engineModel
}

func ResetModel() {
	mu.Lock()
	engineModel = nil
	defaultOrchestrator = newDefaultOrchestrator()
	if scriptPlugin != nil {
		_ = scriptPlugin.OnDestroy(lifecycleCtx)
		scriptPlugin = nil
	}
	mu.Unlock()
}

func GetNativeVersion() string {
	return ENGINE_VERSION
}

func ensureModel(packageName string) *decision.Model {
	// 快速路径：已初始化
	mu.RLock()
	if engineModel != nil {
		m := engineModel
		mu.RUnlock()
		return m
	}
	mu.RUnlock()

	// 慢速路径：需要创建
	mu.Lock()
	if engineModel == nil {
		engineModel = decision.NewModel(packageName, config.GetInstance())
	}
	m := engineModel
	mu.Unlock()
	return m
}

func packageName() string {
	mu.RLock()
	defer mu.RUnlock()
	if engineModel == nil {
		return ""
	}
	return engineModel.GetPackageName()
}
