package model

import (
	"Trek/internal/fastbot/core/types"
	"Trek/internal/fastbot/preference"
	"Trek/log"
)

type Model struct {
	graph           *Graph
	deviceAgentMap  map[string]types.IAgent
	preference      *preference.Preference
	packageName     string
	netActionTaskID int
}

type IAgentCreator func(model *Model, deviceType types.DeviceType) (types.IAgent, error)

var agentCreators map[string]IAgentCreator = make(map[string]IAgentCreator)

func RegisterAgentCreator(algorithmType string, agentCreator IAgentCreator) {
	agentCreators[algorithmType] = agentCreator
}

func NewModel() *Model {

	return &Model{
		graph:          NewGraph(),
		deviceAgentMap: make(map[string]types.IAgent),
		preference:     preference.GetInstance(),
		packageName:    "",
	}
}

func (m *Model) GetGraph() *Graph {
	return m.graph
}

func (m *Model) SetGraph(graph *Graph) {
	m.graph = graph
}

// AddAgent 添加一个新的agent到model层中
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
	//if graphListener, ok := agentInterface.(types.IGraphListener); ok {
	m.graph.AddListener(graphListener)
	//}
	return graphListener
}

func (m *Model) GetAgent(deviceID string) interface{} {
	return m.deviceAgentMap[deviceID]
}

func (m *Model) GetPreference() *preference.Preference {
	return m.preference
}

func (m *Model) SetPreference(preference *preference.Preference) {
	m.preference = preference
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

func (m *Model) GetOperate(descContent string, pageName string, deviceID string) string {

	elem, err := types.CreateFromXml(descContent)
	if err != nil || elem == nil {

		return ""
	}
	operate := m.GetOperateOpt(elem, pageName, deviceID)
	return operate.ToJSON()
}

func (m *Model) GetOperateOpt(elem *types.Element, pageName string, deviceID string) *types.DeviceOperateWrapper {
	//methodStartTime := time.Now()

	customAction := m.resolvePageAndGetSpecifiedAction(pageName, elem)
	if customAction != nil {
		log.Debugf("try get custom action from preference")
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
		log.Debugf("use reuseAgent as the default agent")
		// todo 可扩展点

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
		return types.OperateNop
	}

	var state types.IState
	//algorithmType := agent.GetAlgorithmType()

	if elem != nil {

		state = agent.CreateState(pageString, elem)

		if state != nil {

			state = m.graph.AddState(state)

			if state != nil {

				state.Visit(m.graph.GetTimestamp())
			}
		}

	}

	//stateGeneratedTime := time.Now()

	if state != nil {

		//stateStr := state.String()
		widgetsStr := ""
		actionsStr := ""

		// 获取widgets信息
		if len(state.GetWidgets()) > 0 {
			for _, widget := range state.GetWidgets() {
				if widgetsStr != "" {
					widgetsStr += "\n   "
				}
				widgetsStr += widget.String()
			}
		}

		// 获取actions信息
		if len(state.GetActions()) > 0 {
			for _, action := range state.GetActions() {
				if actionsStr != "" {
					actionsStr += "\n   "
				}
				actionsStr += action.String()
			}
		}

		//stateInfo := fmt.Sprintf("{\nstate: %s\nwidgets: \n   %s\naction: \n   %s\n}",
		//	stateStr, widgetsStr, actionsStr)

	}
	action := customAction

	shouldSkipActionsFromModel := m.skipAllActionsFromModel()
	if shouldSkipActionsFromModel {

	}

	//startGeneratingActionTime := time.Time{}

	if action == nil && !shouldSkipActionsFromModel {
		//startGeneratingActionTime = time.Now()
		if agent.GetCurrentStateBlockTimes() > 0 {
			action = types.RESTARTAction
			stateID := ""
			if state != nil {
				stateID = state.GetId()
			}
			log.Infof("Ran into a block state %s", stateID)

		} else {
			action = agent.ResolveNewAction()
			agent.UpdateStrategy()
			if action == nil {
				log.Errorf("get null action!!!!")
				return types.OperateNop
			}
			if action.(*types.StatefulAction).IsModelAct() && state != nil {
				action.(*types.StatefulAction).Visit(m.graph.GetTimestamp())
				agent.MoveForward(state)
			}
		}
	}

	//endGeneratingActionTime := time.Now()

	operate := types.OperateNop
	if action != nil {
		log.Infof("selected action %s", action.(*types.StatefulAction).String())
		operate = action.(*types.StatefulAction).ToOperate()
		if m.preference != nil {
			m.patchOperate(operate)
		}

		// 如果当前状态有详情，则清除详情
		if state != nil && state.HasDetail() {
			state.ClearDetails()
		}
	}

	//methodEndTime := time.Now()

	return operate
}

func (m *Model) resolvePageAndGetSpecifiedAction(page string, elem *types.Element) types.IAction {
	if m.preference != nil {
		return m.preference.ResolvePageAndGetSpecifiedAction(page, elem)
	}
	return nil
}

func (m *Model) skipAllActionsFromModel() bool {
	if m.preference != nil {
		return m.preference.SkipAllActionsFromModel()
	}
	return false
}

func (m *Model) patchOperate(operate *types.DeviceOperateWrapper) {
	if m.preference != nil {
		m.preference.PatchOperate(operate)
	}
}
