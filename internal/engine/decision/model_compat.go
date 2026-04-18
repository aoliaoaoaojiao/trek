package decision

import (
	oldmodel "trek/internal/engine/core/model"
)

type Model = oldmodel.Model
type IAgentCreator = oldmodel.IAgentCreator
type IElementCreator = oldmodel.IElementCreator
type ActionCounter = oldmodel.ActionCounter
type Graph = oldmodel.Graph
type VisitCountReward = oldmodel.VisitCountReward

const DefaultDeviceID = oldmodel.DefaultDeviceID

func RegisterAgentCreator(algorithmType string, agentCreator IAgentCreator) {
	oldmodel.RegisterAgentCreator(algorithmType, agentCreator)
}

func RegisterElementCreator(eleType string, creator IElementCreator) {
	oldmodel.RegisterElementCreator(eleType, creator)
}

func NewModel(packageName string) *Model {
	return oldmodel.NewModel(packageName)
}

func NewActionCounter() *ActionCounter {
	return oldmodel.NewActionCounter()
}

func NewGraph() *Graph {
	return oldmodel.NewGraph()
}
