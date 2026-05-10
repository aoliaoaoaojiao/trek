package reuse

import (
	"os"
	"strings"
	"time"

	"trek/internal/engine/core/types"
	"trek/internal/engine/decision"
	sharedgraph "trek/internal/engine/decision/shared/graph"
	"trek/logger"
)

var createReuseAgent = func(m *sharedgraph.Model, deviceType types.DeviceType) (types.IAgent, error) {
	reuseAgent := NewModelReusableAgent(m)
	cfgProvider := m.GetStaticConfig()
	if cfgProvider != nil {
		reuseCfg := cfgProvider.GetReuseConfig()
		if reuseCfg.Epsilon.IsSet() {
			reuseAgent.SetEpsilon(reuseCfg.Epsilon.Get())
		}
		if reuseCfg.Gamma.IsSet() {
			reuseAgent.SetGamma(reuseCfg.Gamma.Get())
		}
		if reuseCfg.NStep.IsSet() && reuseCfg.NStep.Get() > 0 {
			reuseAgent.SetNStep(reuseCfg.NStep.Get())
		}
		if strings.TrimSpace(reuseCfg.ModelSavePath) != "" {
			reuseAgent.SetModelSavePath(strings.TrimSpace(reuseCfg.ModelSavePath))
		}
		if reuseCfg.EnableModelPersistence.IsSet() {
			reuseAgent.SetEnableModelPersistence(reuseCfg.EnableModelPersistence.Get())
		}
		if reuseCfg.ResetModelOnStart.IsSet() && reuseCfg.ResetModelOnStart.Get() && reuseAgent.GetEnableModelPersistence() {
			if err := reuseAgent.ResetModelFile(); err != nil && !os.IsNotExist(err) {
				logger.Errorf("Failed to reset reuse model file: %v", err)
			}
		}
	}

	reuseAgent.LoadReuseModel()

	if reuseAgent.GetEnableModelPersistence() {
		go func() {
			ticker := time.NewTicker(10 * time.Minute)
			defer ticker.Stop()

			for {
				select {
				case <-ticker.C:
					if err := reuseAgent.SaveReuseModel(); err != nil {
						logger.Errorf("Failed to auto-save reuse model: %v", err)
					}
				case <-reuseAgent.stopChan:
					logger.Info("Stopping reuse model auto-save routine")
					return
				}
			}
		}()
	}

	return reuseAgent, nil
}

func init() {
	sharedgraph.RegisterAgentCreator(decision.AlgorithmReuse.String(), createReuseAgent)
}

func (a *ModelReusableAgent) Stop() {
	a.SaveReuseModel()
	a.stopOnce.Do(func() {
		close(a.stopChan)
	})
}
