package decision

import (
	"fmt"
	"trek/internal/engine/config"
	sharedmodel "trek/internal/engine/decision/shared/model"
	types2 "trek/internal/engine/decision/shared/types"
	"trek/logger"
)

type Model struct {
	core          *sharedmodel.Model
	configManager *config.Manager
}

type IAgentCreator = sharedmodel.IAgentCreator
type IElementCreator func(guiContent string) (types2.IElement, error)
type ActionCounter = sharedmodel.ActionCounter
type Graph = sharedmodel.Graph
type VisitCountReward = sharedmodel.VisitCountReward

const DefaultDeviceID = sharedmodel.DefaultDeviceID

var elementCreators = make(map[string]IElementCreator)

func RegisterAgentCreator(algorithmType string, agentCreator IAgentCreator) {
	sharedmodel.RegisterAgentCreator(algorithmType, agentCreator)
}

func RegisterElementCreator(eleType string, creator IElementCreator) {
	elementCreators[eleType] = creator
}

func NewModel(packageName string) *Model {
	return &Model{
		core:          sharedmodel.NewModel(packageName),
		configManager: config.GetInstance(),
	}
}

func NewActionCounter() *ActionCounter {
	return sharedmodel.NewActionCounter()
}

func NewGraph() *Graph {
	return sharedmodel.NewGraph()
}

func (m *Model) GetGraph() *Graph {
	if m == nil || m.core == nil {
		return nil
	}
	return m.core.GetGraph()
}

func (m *Model) SetGraph(graph *Graph) {
	if m == nil || m.core == nil {
		return
	}
	m.core.SetGraph(graph)
}

func (m *Model) AddAgent(deviceID string, algorithmType string, deviceType types2.DeviceType) types2.IAgent {
	if m == nil || m.core == nil {
		return nil
	}
	return m.core.AddAgent(deviceID, algorithmType, deviceType)
}

func (m *Model) GetAgent(deviceID string) interface{} {
	if m == nil || m.core == nil {
		return nil
	}
	return m.core.GetAgent(deviceID)
}

func (m *Model) GetConfigManager() *config.Manager {
	if m == nil {
		return nil
	}
	return m.configManager
}

func (m *Model) SetConfigManager(manager *config.Manager) {
	if m == nil {
		return
	}
	m.configManager = manager
}

func (m *Model) GetPackageName() string {
	if m == nil || m.core == nil {
		return ""
	}
	return m.core.GetPackageName()
}

func (m *Model) SetPackageName(packageName string) {
	if m == nil || m.core == nil {
		return
	}
	m.core.SetPackageName(packageName)
}

func (m *Model) GetNetActionTaskID() int {
	if m == nil || m.core == nil {
		return 0
	}
	return m.core.GetNetActionTaskID()
}

func (m *Model) SetNetActionTaskID(taskID int) {
	if m == nil || m.core == nil {
		return
	}
	m.core.SetNetActionTaskID(taskID)
}

func (m *Model) StateSize() int {
	if m == nil || m.core == nil {
		return 0
	}
	return m.core.StateSize()
}

func (m *Model) GetOperateOpt(elem types2.IElement, pageName string, deviceID string) *types2.ActionCommand {
	customAction := m.resolvePageAndGetSpecifiedAction(pageName, elem)
	if customAction != nil {
		logger.Debugf("try get custom action from config manager")
	}

	if m == nil || m.core == nil {
		return types2.ActionCommandNop
	}

	if m.core.AgentSize() == 0 {
		logger.Warnf("no decision agent registered, return NOP")
		return types2.ActionCommandNop
	}

	agent := m.resolveAgent(deviceID)
	if agent == nil {
		return types2.ActionCommandNop
	}

	var state types2.IState
	if elem != nil {
		state = agent.CreateState(pageName, elem)
		if state != nil {
			state = m.core.GetGraph().AddState(state)
			if state != nil {
				state.Visit(m.core.GetGraph().GetTimestamp())
			}
		}
	}

	action := customAction
	if action == nil && !m.skipAllActionsFromModel() {
		blockTimes := 0
		if blockAware, ok := agent.(types2.StateBlockAwareAgent); ok {
			blockTimes = blockAware.GetCurrentStateBlockTimes()
		}

		if blockTimes > 0 {
			action = types2.RESTARTAction
			stateID := ""
			if state != nil {
				stateID = state.GetId()
			}
			logger.Infof("Ran into a block state %s", stateID)
		} else {
			action = agent.ResolveNewAction()
			agent.UpdateStrategy()
			if action == nil {
				logger.Errorf("get null action!!!!")
				return types2.ActionCommandNop
			}
			if stateAction, ok := action.(*types2.StatefulAction); ok && stateAction.IsModelAct() && state != nil {
				stateAction.Visit(m.core.GetGraph().GetTimestamp())
				agent.MoveForward(state)
			}
		}
	}

	operate := types2.ActionCommandNop
	if action == nil {
		return operate
	}

	switch a := action.(type) {
	case *config.CustomAction:
		logger.Infof("selected custom action %s", a.ActionType.String())
		operate = a.ToActionCommand()
	case *types2.StatefulAction:
		logger.Debugf("selected action %s", a.String())
		operate = a.ToOperate()
	default:
		logger.Errorf("unsupported action type: %T", action)
		return types2.ActionCommandNop
	}

	if m.configManager != nil {
		m.configManager.PatchOperate(operate)
	}
	if state != nil && state.HasDetail() {
		state.ClearDetails()
	}
	return operate
}

// GetOperate 通过元素类型和内容解析元素并输出动作 JSON。
func (m *Model) GetOperate(elemType string, descContent string, pageName string, deviceID string) string {
	elem, err := CreateElement(elemType, descContent)
	if err != nil || elem == nil {
		return ""
	}
	return m.GetOperateOpt(elem, pageName, deviceID).ToJSON()
}

// CreateElement 使用 decision 层注册表解析元素，避免 shared 承担装配职责。
func CreateElement(elemType string, guiContent string) (types2.IElement, error) {
	creator, ok := elementCreators[elemType]
	if !ok || creator == nil {
		return nil, fmt.Errorf("element creator not found: %s", elemType)
	}
	return creator(guiContent)
}

func (m *Model) resolveAgent(deviceID string) types2.IAgent {
	if m == nil || m.core == nil {
		return nil
	}
	if deviceID == "" {
		if agent, _ := m.core.GetAgent(DefaultDeviceID).(types2.IAgent); agent != nil {
			return agent
		}
	}
	if agent, _ := m.core.GetAgent(deviceID).(types2.IAgent); agent != nil {
		return agent
	}
	if agent, _ := m.core.GetAgent(DefaultDeviceID).(types2.IAgent); agent != nil {
		return agent
	}
	return nil
}

func (m *Model) resolvePageAndGetSpecifiedAction(page string, elem types2.IElement) types2.IAction {
	if m != nil && m.configManager != nil {
		return m.configManager.ResolvePageAndGetSpecifiedAction(page, elem)
	}
	return nil
}

func (m *Model) skipAllActionsFromModel() bool {
	if m != nil && m.configManager != nil {
		return m.configManager.SkipAllActionsFromModel()
	}
	return false
}
