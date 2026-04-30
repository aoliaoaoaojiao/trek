package monkey

import (
	"trek/internal/engine/decision"
	sharedgraph "trek/internal/engine/decision/shared/graph"
	"trek/internal/engine/decision/shared/types"
)

var createMonkeyAgent = func(m *sharedgraph.Model, deviceType types.DeviceType) (types.IAgent, error) {
	return NewMonkeyAgent(m), nil
}

func init() {
	sharedgraph.RegisterAgentCreator(decision.AlgorithmRandom.String(), createMonkeyAgent)
}
