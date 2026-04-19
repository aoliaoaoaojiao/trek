package model

import (
	"trek/internal/engine/config"
	"trek/internal/engine/core/types"
	"trek/logger"
)

type Model struct {
	graph           *Graph
	deviceAgentMap  map[string]types.IAgent
	configManager   *config.Manager
	packageName     string
	netActionTaskID int
}

type IAgentCreator func(model *Model, deviceType types.DeviceType) (types.IAgent, error)

var agentCreators map[string]IAgentCreator = make(map[string]IAgentCreator)

func RegisterAgentCreator(algorithmType string, agentCreator IAgentCreator) {
	agentCreators[algorithmType] = agentCreator
}

type IElementCreator func(guiContent string) (types.IElement, error)

var elementCreators map[string]IElementCreator = make(map[string]IElementCreator)

func RegisterElementCreator(eleType string, creator IElementCreator) {
	elementCreators[eleType] = creator
}

func NewModel(packageName string) *Model {
	return &Model{
		graph:          NewGraph(),
		deviceAgentMap: make(map[string]types.IAgent),
		configManager:  config.GetInstance(),
		packageName:    packageName,
	}
}

func (m *Model) GetGraph() *Graph {
	return m.graph
}

func (m *Model) SetGraph(graph *Graph) {
	m.graph = graph
}

// AddAgent 娣诲姞涓€涓柊鐨刟gent鍒癿odel灞備腑
func (m *Model) AddAgent(deviceID string, algorithmType string, deviceType types.DeviceType) types.IAgent {
	var graphListener types.IAgent
	var err error
	var agentCreator = agentCreators[algorithmType]
	if agentCreator != nil {
		graphListener, err = agentCreator(m, deviceType)
		if err != nil {
			panic("failed to create an agent")
		}
	}
	m.deviceAgentMap[deviceID] = graphListener
	m.graph.AddListener(graphListener)
	return graphListener
}

func (m *Model) GetAgent(deviceID string) interface{} {
	return m.deviceAgentMap[deviceID]
}

func (m *Model) GetConfigManager() *config.Manager {
	return m.configManager
}

func (m *Model) SetConfigManager(manager *config.Manager) {
	m.configManager = manager
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

const DefaultDeviceID = "0000001"

func (m *Model) GetOperate(elemType string, descContent string, pageName string, deviceID string) string {
	var elem types.IElement
	var err error

	if elementCreator, ok := elementCreators[elemType]; ok {
		elem, err = elementCreator(descContent)
	}

	if err != nil || elem == nil {
		return ""
	}
	operate := m.GetOperateOpt(elem, pageName, deviceID)
	return operate.ToJSON()
}

func (m *Model) GetOperateOpt(elem types.IElement, pageName string, deviceID string) *types.ActionCommand {
	customAction := m.resolvePageAndGetSpecifiedAction(pageName, elem)
	if customAction != nil {
		logger.Debugf("try get custom action from config manager")

	}

	visitedPages := m.graph.GetVisitedPages()
	var pageString string
	found := false
	for act := range visitedPages {
		if act == pageName {
			pageString = act
			found = true
			break
		}
	}
	if !found {
		pageString = pageName
	}

	if len(m.deviceAgentMap) == 0 {
		logger.Debugf("use reuseAgent as the default agent")
		m.AddAgent(DefaultDeviceID, types.Reuse.String(), types.Phone)
	}

	var agent types.IAgent
	if deviceID == "" {
		agent = m.deviceAgentMap[DefaultDeviceID]
	} else {
		agent = m.deviceAgentMap[deviceID]
		if agent == nil {
			agent = m.deviceAgentMap[DefaultDeviceID]
		}
	}

	if agent == nil {
		return types.ActionCommandNop
	}

	var state types.IState
	if elem != nil {
		state = agent.CreateState(pageString, elem)
		if state != nil {
			state = m.graph.AddState(state)
			if state != nil {
				state.Visit(m.graph.GetTimestamp())
			}
		}
	}

	if state != nil {
		widgetsStr := ""
		actionsStr := ""
		if len(state.GetWidgets()) > 0 {
			for _, widget := range state.GetWidgets() {
				if widgetsStr != "" {
					widgetsStr += "\n   "
				}
				widgetsStr += widget.String()
			}
		}
		if len(state.GetActions()) > 0 {
			for _, action := range state.GetActions() {
				if actionsStr != "" {
					actionsStr += "\n   "
				}
				actionsStr += action.String()
			}
		}
		_, _ = widgetsStr, actionsStr
	}

	action := customAction
	shouldSkipActionsFromModel := m.skipAllActionsFromModel()

	if action == nil && !shouldSkipActionsFromModel {
		if agent.GetCurrentStateBlockTimes() > 0 {
			action = types.RESTARTAction
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
				return types.ActionCommandNop
			}
			if action.(*types.StatefulAction).IsModelAct() && state != nil {
				action.(*types.StatefulAction).Visit(m.graph.GetTimestamp())
				agent.MoveForward(state)
			}
		}
	}

	operate := types.ActionCommandNop
	if action != nil {
		switch a := action.(type) {
		case *config.CustomAction:
			logger.Infof("selected custom action %s", a.ActionType.String())
			operate = a.ToActionCommand()
		case *types.StatefulAction:
			logger.Debugf("selected action %s", a.String())
			operate = a.ToOperate()
		default:
			logger.Errorf("unsupported action type: %T", action)
			return types.ActionCommandNop
		}
		if m.configManager != nil {
			m.patchOperate(operate)
		}
		if state != nil && state.HasDetail() {
			state.ClearDetails()
		}
	}

	return operate
}

func (m *Model) resolvePageAndGetSpecifiedAction(page string, elem types.IElement) types.IAction {
	if m.configManager != nil {
		return m.configManager.ResolvePageAndGetSpecifiedAction(page, elem)
	}
	return nil
}

func (m *Model) skipAllActionsFromModel() bool {
	if m.configManager != nil {
		return m.configManager.SkipAllActionsFromModel()
	}
	return false
}

func (m *Model) patchOperate(operate *types.ActionCommand) {
	if m.configManager != nil {
		m.configManager.PatchOperate(operate)
	}
}
