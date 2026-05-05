package graph

import (
	"fmt"

	"trek/internal/engine/core/types"
)

type Model struct {
	graph           *Graph
	deviceAgentMap  map[string]types.IAgent
	packageName     string
	netActionTaskID int
	staticConfig    types.StaticConfigProvider
}

type IAgentCreator func(model *Model, deviceType types.DeviceType) (types.IAgent, error)

var agentCreators = make(map[string]IAgentCreator)

func RegisterAgentCreator(algorithmType string, agentCreator IAgentCreator) {
	agentCreators[algorithmType] = agentCreator
}

func NewModel(packageName string) *Model {
	return &Model{
		graph:          NewGraph(),
		deviceAgentMap: make(map[string]types.IAgent),
		packageName:    packageName,
	}
}

func (m *Model) GetGraph() *Graph {
	return m.graph
}

func (m *Model) SetGraph(graph *Graph) {
	m.graph = graph
}

// AddAgent adds an agent to the model.
func (m *Model) AddAgent(deviceID string, algorithmType string, deviceType types.DeviceType) (types.IAgent, error) {
	var graphListener types.IAgent
	agentCreator := agentCreators[algorithmType]
	if agentCreator != nil {
		created, err := agentCreator(m, deviceType)
		if err != nil {
			return nil, fmt.Errorf("创建决策代理失败(algorithm=%s): %w", algorithmType, err)
		}
		graphListener = created
	}
	m.deviceAgentMap[deviceID] = graphListener
	m.graph.AddListener(graphListener)
	return graphListener, nil
}

func (m *Model) GetAgent(deviceID string) interface{} {
	return m.deviceAgentMap[deviceID]
}

func (m *Model) GetPackageName() string {
	return m.packageName
}

func (m *Model) SetPackageName(packageName string) {
	m.packageName = packageName
}

func (m *Model) GetNetActionTaskID() int {
	return m.netActionTaskID
}

func (m *Model) SetNetActionTaskID(taskID int) {
	m.netActionTaskID = taskID
}

func (m *Model) StateSize() int {
	return m.graph.StateSize()
}

func (m *Model) AgentSize() int {
	return len(m.deviceAgentMap)
}

func (m *Model) SetStaticConfig(cfg types.StaticConfigProvider) {
	m.staticConfig = cfg
}

func (m *Model) GetStaticConfig() types.StaticConfigProvider {
	return m.staticConfig
}

const DefaultDeviceID = "0000001"
