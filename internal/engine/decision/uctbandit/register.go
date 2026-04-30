package uctbandit

import (
	"trek/internal/engine/decision"
	sharedgraph "trek/internal/engine/decision/shared/graph"
	"trek/internal/engine/decision/shared/types"
)

var createUctBanditAgent = func(m *sharedgraph.Model, deviceType types.DeviceType) (types.IAgent, error) {
	agent := NewAgent(m, deviceType, m.GetStaticConfig())
	return agent, nil
}

func init() {
	sharedgraph.RegisterAgentCreator(decision.AlgorithmUctBandit.String(), createUctBanditAgent)
}