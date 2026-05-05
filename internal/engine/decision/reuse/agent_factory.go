package reuse

import (
	"time"

	"trek/internal/engine/core/types"
	"trek/internal/engine/decision"
	sharedgraph "trek/internal/engine/decision/shared/graph"
	"trek/logger"
)

var createReuseAgent = func(m *sharedgraph.Model, deviceType types.DeviceType) (types.IAgent, error) {
	reuseAgent := NewModelReusableAgent(m)

	reuseAgent.LoadReuseModel()

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
