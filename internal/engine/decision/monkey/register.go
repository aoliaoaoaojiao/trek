package monkey

import (
	"trek/internal/engine/core/types"
	"trek/internal/engine/decision"
	sharedgraph "trek/internal/engine/decision/shared/graph"
)

var createMonkeyAgent = func(m *sharedgraph.Model, deviceType types.DeviceType) (types.IAgent, error) {
	return NewMonkeyAgent(m), nil
}

func init() {
	sharedgraph.RegisterAgentCreator(decision.AlgorithmRandom.String(), createMonkeyAgent)
}
