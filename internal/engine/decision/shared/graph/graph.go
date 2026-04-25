package graph

import (
	"time"
	types2 "trek/internal/engine/decision/shared/types"
	"trek/logger"
)

// Legacy compatibility types (kept for old exported aliases).
type ActionCounter struct {
	actCount []int64
	total    int64
}

func NewActionCounter() *ActionCounter {
	return &ActionCounter{
		actCount: make([]int64, types2.ActTypeSize),
		total:    0,
	}
}

func (a *ActionCounter) CountAction(action *types2.StatefulAction) {
	actionType := action.GetActionType()
	if actionType < types2.ActTypeSize {
		a.actCount[actionType]++
	}
	a.total++
}

func (a *ActionCounter) GetTotal() int64 {
	return a.total
}

func (a *ActionCounter) GetCount(actionType types2.ActionType) int64 {
	if actionType < types2.ActTypeSize {
		return a.actCount[actionType]
	}
	return 0
}

// Legacy compatibility type (kept for old exported aliases).
type VisitCountReward struct {
	Count  int
	Reward float64
}

// Graph is a strategy-agnostic state graph core.
type Graph struct {
	types2.Node
	states    types2.StateSet
	listeners []types2.IGraphListener
	timeStamp time.Time
}

func NewGraph() *Graph {
	return &Graph{
		Node:      *types2.NewNode(),
		states:    make(types2.StateSet),
		listeners: make([]types2.IGraphListener, 0),
		timeStamp: time.Now(),
	}
}

func (g *Graph) StateSize() int {
	return len(g.states)
}

func (g *Graph) GetTimestamp() time.Time {
	return g.timeStamp
}

func (g *Graph) AddListener(listener types2.IGraphListener) {
	g.listeners = append(g.listeners, listener)
}

func (g *Graph) AddState(state types2.IState) types2.IState {
	stateHash := state.Hash()
	existingState, found := g.states[stateHash]

	if !found {
		newStateID := int32(len(g.states))
		state.SetId(newStateID)
		g.states[stateHash] = state
		logger.Debugf("adding new state with ID: %d, total states: %d", newStateID, len(g.states))
	} else {
		if !existingState.HasDetail() {
			existingState.FillDetails(state)
		}
		state = existingState
		logger.Debugf("reusing existing state with ID: %d", state.GetId())
	}

	g.NotifyNewStateEvents(state)
	return state
}

func (g *Graph) GetStates() types2.StateSet {
	return g.states
}

func (g *Graph) UpdateTimeStamp() {
	g.timeStamp = time.Now()
}

func (g *Graph) NotifyNewStateEvents(node types2.IState) {
	for _, listener := range g.listeners {
		listener.OnAddNode(node)
	}
}
