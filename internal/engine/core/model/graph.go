package model

import (
	"time"
	"trek/internal/engine/core/types"
	"trek/logger"
)

type ActionCounter struct {
	actCount []int64
	total    int64
}

func NewActionCounter() *ActionCounter {
	return &ActionCounter{
		actCount: make([]int64, types.ActTypeSize),
		total:    0,
	}
}

func (a *ActionCounter) CountAction(action *types.StatefulAction) {
	actionType := action.GetActionType()
	if actionType < types.ActTypeSize {
		a.actCount[actionType]++
	}
	a.total++
}

func (a *ActionCounter) GetTotal() int64 {
	return a.total
}

func (a *ActionCounter) GetCount(actionType types.ActionType) int64 {
	if actionType < types.ActTypeSize {
		return a.actCount[actionType]
	}
	return 0
}

type Graph struct {
	types.Node
	states           types.StateSet
	visitedPages     map[string]struct{}
	pageDistri       map[string]VisitCountReward
	totalDistri      int64
	widgetActions    map[uintptr]types.StatefulActionSet
	unvisitedActions types.StatefulActionSet
	visitedActions   types.StatefulActionSet
	actionCounter    ActionCounter
	listeners        []types.IGraphListener
	timeStamp        time.Time
}

type VisitCountReward struct {
	Count  int     // 访问计数
	Reward float64 // 奖励值
}

func NewGraph() *Graph {
	return &Graph{
		Node:             *types.NewNode(),
		states:           make(types.StateSet),
		visitedPages:     make(map[string]struct{}),
		pageDistri:       make(map[string]VisitCountReward),
		totalDistri:      0,
		actionCounter:    *NewActionCounter(),
		widgetActions:    make(map[uintptr]types.StatefulActionSet),
		unvisitedActions: make(types.StatefulActionSet),
		visitedActions:   make(types.StatefulActionSet),
		listeners:        make([]types.IGraphListener, 0),
		timeStamp:        time.Now(),
	}
}

func (g *Graph) StateSize() int {
	return len(g.states)
}

func (g *Graph) GetTimestamp() time.Time {
	return g.timeStamp
}

func (g *Graph) AddListener(listener types.IGraphListener) {
	g.listeners = append(g.listeners, listener)
}

func (g *Graph) AddState(state types.IState) types.IState {
	pageNameString := state.GetPageNameString()
	// 使用哈希值检查状态是否已存在
	stateHash := state.Hash()
	existingState, found := g.states[stateHash]

	if !found {
		// 如果是新状态，设置ID并添加到状态集合
		newStateId := int32(len(g.states))
		state.SetId(newStateId)
		g.states[stateHash] = state
		logger.Debugf("adding new state with ID: %d, pageNameString: %s, total states: %d", newStateId, pageNameString, len(g.states))
	} else {
		// 如果状态已存在，复用已有状态
		if !existingState.HasDetail() {
			existingState.FillDetails(state)
		}
		state = existingState
		logger.Debugf("reusing existing state with ID: %d, pageNameString: %s", state.GetId(), pageNameString)
	}

	// 通知新状态事件
	g.NotifyNewStateEvents(state)

	// 添加活动到已访问活动集合
	g.visitedPages[pageNameString] = struct{}{}

	// 更新总分布计数
	g.totalDistri++

	// 更新活动分布统计
	if _, exists := g.pageDistri[pageNameString]; !exists {
		g.pageDistri[pageNameString] = VisitCountReward{0, 0.0}
	}
	pair := g.pageDistri[pageNameString]
	pair.Count++
	pair.Reward = float64(pair.Count) / float64(g.totalDistri)
	g.pageDistri[pageNameString] = pair

	// 添加来自状态的动作
	g.addActionFromState(state)

	return state
}

func (g *Graph) GetTotalDistri() int64 {
	return g.totalDistri
}

func (g *Graph) GetVisitedPages() map[string]struct{} {
	return g.visitedPages
}

func (g *Graph) GetActionCounter() *ActionCounter {
	return &g.actionCounter
}

func (g *Graph) GetWidgetActions() map[uintptr]types.StatefulActionSet {
	return g.widgetActions
}

func (g *Graph) GetUnvisitedActions() types.StatefulActionSet {
	return g.unvisitedActions
}

func (g *Graph) GetVisitedActions() types.StatefulActionSet {
	return g.visitedActions
}

func (g *Graph) GetStates() types.StateSet {
	return g.states
}

func (g *Graph) GetPageDistri() map[string]VisitCountReward {
	return g.pageDistri
}

func (g *Graph) UpdateTimeStamp() {
	g.timeStamp = time.Now()
}

func (g *Graph) IncrementTotalDistri() {
	g.totalDistri++
}

func (g *Graph) AddVisitedPage(pageName string) {
	g.visitedPages[pageName] = struct{}{}
}

func (g *Graph) UpdatePageDistri(pageName string, count int, reward float64) {
	g.pageDistri[pageName] = VisitCountReward{count, reward}
}

func (g *Graph) AddWidgetAction(widget types.IWidget, action *types.StatefulAction) {
	if g.widgetActions[widget.Hash()] == nil {
		g.widgetActions[widget.Hash()] = make(types.StatefulActionSet)
	}
	g.widgetActions[widget.Hash()][action.Hash()] = action
}

func (g *Graph) AddUnvisitedAction(action *types.StatefulAction) {
	if action != nil {
		g.unvisitedActions[action.Hash()] = action
	}
}

func (g *Graph) AddVisitedAction(action *types.StatefulAction) {
	if action != nil {
		g.visitedActions[action.Hash()] = action
		delete(g.unvisitedActions, action.Hash())
	}
}

func (g *Graph) NotifyNewStateEvents(node types.IState) {
	for _, listener := range g.listeners {
		listener.OnAddNode(node)
	}
}

func (g *Graph) addActionFromState(node types.IState) {
	nodeActions := node.GetActions()
	logger.Debugf("addActionFromState - node has %d actions", len(nodeActions))

	for _, action := range nodeActions {
		// 检查动作是否已访问
		visitedAdd := false
		actionHash := action.Hash()
		existingVisitedAction, visitedAdd := g.visitedActions[actionHash]

		unvisitedAdd := false
		var existingUnvisitedAction *types.StatefulAction
		if !visitedAdd {
			existingUnvisitedAction, unvisitedAdd = g.unvisitedActions[actionHash]
		}

		logger.Debugf("action %s - visited:%t unvisited:%t", action.String(), visitedAdd, unvisitedAdd)

		if visitedAdd || unvisitedAdd {
			// 如果动作已存在，复用ID
			var existingId int32
			if visitedAdd {
				existingId = existingVisitedAction.GetIdi()
			} else {
				existingId = existingUnvisitedAction.GetIdi()
			}
			action.SetId(existingId)
			logger.Debugf("reusing existing action ID: %d", existingId)
		} else {
			// 如果是新动作，分配新ID并计数
			newId := int32(g.actionCounter.GetTotal())
			action.SetId(newId)
			g.actionCounter.CountAction(action)
			logger.Debugf("assigning new action ID: %d, total count now: %d", newId, g.actionCounter.GetTotal())
		}

		if !visitedAdd && action.IsVisited() {
			g.visitedActions[actionHash] = action
		}

		if !unvisitedAdd && !action.IsVisited() {
			g.unvisitedActions[actionHash] = action
		}
	}
	logger.Debugf("unvisited action: %d, visited action %d", len(g.unvisitedActions), len(g.visitedActions))
}
