package model

import decisionpkg "trek/internal/engine/decision"

type Model = decisionpkg.Model
type IAgentCreator = decisionpkg.IAgentCreator
type IElementCreator = decisionpkg.IElementCreator
type ActionCounter = decisionpkg.ActionCounter
type Graph = decisionpkg.Graph
type VisitCountReward = decisionpkg.VisitCountReward

const DefaultDeviceID = decisionpkg.DefaultDeviceID

func RegisterAgentCreator(algorithmType string, agentCreator IAgentCreator) {
	decisionpkg.RegisterAgentCreator(algorithmType, agentCreator)
}

func RegisterElementCreator(eleType string, creator IElementCreator) {
	decisionpkg.RegisterElementCreator(eleType, creator)
}

func NewModel(packageName string) *Model {
	return decisionpkg.NewModel(packageName)
}

func NewActionCounter() *ActionCounter {
	return decisionpkg.NewActionCounter()
}

func NewGraph() *Graph {
	return decisionpkg.NewGraph()
}
