package decision

import (
	coremodel "trek/internal/engine/core/model"
)

type Model = coremodel.Model
type IAgentCreator = coremodel.IAgentCreator
type IElementCreator = coremodel.IElementCreator
type ActionCounter = coremodel.ActionCounter
type Graph = coremodel.Graph
type VisitCountReward = coremodel.VisitCountReward

const DefaultDeviceID = coremodel.DefaultDeviceID

func RegisterAgentCreator(algorithmType string, agentCreator IAgentCreator) {
	coremodel.RegisterAgentCreator(algorithmType, agentCreator)
}

func RegisterElementCreator(eleType string, creator IElementCreator) {
	coremodel.RegisterElementCreator(eleType, creator)
}

func NewModel(packageName string) *Model {
	return coremodel.NewModel(packageName)
}

func NewActionCounter() *ActionCounter {
	return coremodel.NewActionCounter()
}

func NewGraph() *Graph {
	return coremodel.NewGraph()
}
