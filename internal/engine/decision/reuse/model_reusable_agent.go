package reuse

import (
	"fmt"
	"sync"

	"trek/internal/engine/core/types"
	"trek/internal/engine/decision"
	sharedgraph "trek/internal/engine/decision/shared/graph"
)

var _ types.IAgent = (*ModelReusableAgent)(nil)

const (
	SarsaRLDefaultAlpha     = 0.25
	SarsaRLDefaultEpsilon   = 0.05
	SarsaRLDefaultGamma     = 0.8
	SarsaNStep              = 5
	EntropyAlpha            = 0.1
	BLOCK_STATETIME_RESTART = -1
)

type PageVisitCount map[string]int
type ActionPageStatistics map[uintptr]PageVisitCount
type ActionQValue map[uintptr]float64

type ModelReusableAgent struct {
	model                           *sharedgraph.Model
	lastState                       types.IState
	currentState                    types.IState
	newState                        types.IState
	lastAction                      *types.StatefulAction
	currentAction                   *types.StatefulAction
	newAction                       *types.StatefulAction
	validateFilter                  types.IStatefulActionFilter
	graphStableCounter              int64
	stateStableCounter              int64
	pageNameStableCounter           int64
	disableFuzz                     bool
	requestRestart                  bool
	appPageNameJustStartedFromClean bool
	appPageNameJustStarted          bool
	currentStateRecovered           bool
	currentStateBlockTimes          int
	algorithmType                   string

	alpha           float64
	epsilon         float64
	rewardCache     []float64
	previousActions []types.IAction
	reuseModel      ActionPageStatistics
	reuseQValue     ActionQValue
	qValueFilter    types.IStatefulActionFilter
	modelSavePath   string
	reuseModelLock  sync.Mutex
	visitStats      reuseVisitStats

	stopChan chan struct{}
	stopOnce sync.Once
}

func NewModelReusableAgent(model *sharedgraph.Model) *ModelReusableAgent {
	agent := &ModelReusableAgent{
		validateFilter:                  NewActionFilterValidDatePriority(),
		graphStableCounter:              0,
		stateStableCounter:              0,
		pageNameStableCounter:           0,
		disableFuzz:                     false,
		requestRestart:                  false,
		appPageNameJustStartedFromClean: false,
		appPageNameJustStarted:          false,
		currentStateRecovered:           false,
		currentStateBlockTimes:          0,
		algorithmType:                   decision.AlgorithmReuse.String(),
		model:                           model,
		alpha:                           SarsaRLDefaultAlpha,
		epsilon:                         SarsaRLDefaultEpsilon,
		rewardCache:                     make([]float64, 0),
		previousActions:                 make([]types.IAction, 0),
		reuseModel:                      make(ActionPageStatistics),
		reuseQValue:                     make(ActionQValue),
		visitStats: reuseVisitStats{
			visitedPages: make(map[string]struct{}),
		},
		stopChan: make(chan struct{}),
	}
	agent.qValueFilter = NewActionFilterValidValuePriority(func(action *types.StatefulAction) float64 {
		return agent.getQValueByHash(action.Hash())
	})
	if model.GetPackageName() != "" {
		agent.modelSavePath = fmt.Sprintf("./%s_reuse.model", model.GetPackageName())
	} else {
		agent.modelSavePath = "./default_reuse.model"
	}
	return agent
}

func (a *ModelReusableAgent) CreateState(pageName string, element types.IElement) types.IState {

	statePointer := Create(pageName, element)

	return &statePointer.State
}

func (a *ModelReusableAgent) OnAddNode(node types.IState) {
	a.newState = node
	a.visitStats.Record(node)

	if BLOCK_STATETIME_RESTART != -1 {
		if a.newState.Equals(a.currentState) {
			a.currentStateBlockTimes++
		} else {
			a.currentStateBlockTimes = 0
		}
	}
}

func (a *ModelReusableAgent) GetCurrentStateBlockTimes() int {
	return a.currentStateBlockTimes
}

func (a *ModelReusableAgent) ResolveNewAction() types.IAction {
	a.adjustActions()
	action := a.SelectNewAction()
	if action == nil {
		return nil
	}

	statefulAction, ok := action.(*types.StatefulAction)
	if !ok {
		return nil
	}

	if statefulAction == nil {
		return nil
	}

	a.newAction = statefulAction

	return action
}

func (a *ModelReusableAgent) MoveForward(nextState types.IState) {
	a.lastState = a.currentState
	a.currentState = a.newState
	a.newState = nextState

	a.lastAction = a.currentAction
	a.currentAction = a.newAction
	a.newAction = nil

}

func (a *ModelReusableAgent) GetAlgorithmType() string {
	return a.algorithmType
}

func (a *ModelReusableAgent) GetModel() *sharedgraph.Model {
	return a.model
}

func (a *ModelReusableAgent) SetModel(model *sharedgraph.Model) {
	a.model = model
}

func (a *ModelReusableAgent) GetLastState() types.IState {
	return a.lastState
}

func (a *ModelReusableAgent) GetCurrentState() types.IState {
	return a.currentState
}

func (a *ModelReusableAgent) GetNewState() types.IState {
	return a.newState
}

func (a *ModelReusableAgent) GetLastAction() *types.StatefulAction {
	return a.lastAction
}

func (a *ModelReusableAgent) GetCurrentAction() *types.StatefulAction {
	return a.currentAction
}

func (a *ModelReusableAgent) GetNewAction() *types.StatefulAction {
	return a.newAction
}

func (a *ModelReusableAgent) GetValidateFilter() types.IStatefulActionFilter {
	return a.validateFilter
}

func (a *ModelReusableAgent) SetValidateFilter(filter types.IStatefulActionFilter) {
	a.validateFilter = filter
}

func (a *ModelReusableAgent) GetDisableFuzz() bool {
	return a.disableFuzz
}

func (a *ModelReusableAgent) SetDisableFuzz(disable bool) {
	a.disableFuzz = disable
}

func (a *ModelReusableAgent) GetRequestRestart() bool {
	return a.requestRestart
}

func (a *ModelReusableAgent) SetRequestRestart(request bool) {
	a.requestRestart = request
}

func (a *ModelReusableAgent) GetAppPageNameJustStartedFromClean() bool {
	return a.appPageNameJustStartedFromClean
}

func (a *ModelReusableAgent) SetAppPageNameJustStartedFromClean(justStarted bool) {
	a.appPageNameJustStartedFromClean = justStarted
}

func (a *ModelReusableAgent) GetAppPageNameJustStarted() bool {
	return a.appPageNameJustStarted
}

func (a *ModelReusableAgent) SetAppPageNameJustStarted(justStarted bool) {
	a.appPageNameJustStarted = justStarted
}

func (a *ModelReusableAgent) GetCurrentStateRecovered() bool {
	return a.currentStateRecovered
}

func (a *ModelReusableAgent) SetCurrentStateRecovered(recovered bool) {
	a.currentStateRecovered = recovered
}

func (a *ModelReusableAgent) handleNullAction() types.IAction {
	if a.newState == nil {
		return nil
	}

	action := a.newStateRandomPickAction(a.validateFilter)
	if action != nil {
		if pageStateAction, ok := action.(*types.StatefulAction); ok {
			resolved := a.newState.ResolveAt(pageStateAction, a.model.GetGraph().GetTimestamp())
			if resolved != nil {
				return resolved
			}
		}
	}

	return nil
}

func (a *ModelReusableAgent) newStateRandomPickAction(filter types.IStatefulActionFilter) types.IAction {
	return RandomPickAction(a.newState, filter, true)
}

func (a *ModelReusableAgent) adjustActions() {
	if a.newState == nil {
		return
	}

	totalPriority := 0

	for _, action := range a.newState.GetActions() {
		basePriority := action.GetPriorityByActionType()
		action.SetPriority(basePriority)

		if !action.RequireTarget() {
			if !action.IsVisited() {
				priority := action.GetPriority()
				priority += 5
				action.SetPriority(priority)
			}
			continue
		}

		if !action.IsValid() {
			continue
		}

		priority := action.GetPriority()
		if !action.IsVisited() {
			priority += 20
		}

		if !a.newState.IsSaturated(action) {
			priority += 5 * action.GetPriorityByActionType()
		}

		if priority <= 0 {
			priority = 0
		}

		action.SetPriority(priority)
		totalPriority += int(priority - action.GetPriorityByActionType())
	}

	a.newState.SetPriority(int32(totalPriority))
}

func (a *ModelReusableAgent) GetReuseModel() ActionPageStatistics {
	return a.reuseModel
}

func (a *ModelReusableAgent) GetReuseQValue() ActionQValue {
	return a.reuseQValue
}

func (a *ModelReusableAgent) GetAlpha() float64 {
	return a.alpha
}

func (a *ModelReusableAgent) GetEpsilon() float64 {
	return a.epsilon
}

func (a *ModelReusableAgent) SetAlpha(alpha float64) {
	a.alpha = alpha
}

func (a *ModelReusableAgent) SetEpsilon(epsilon float64) {
	a.epsilon = epsilon
}

func (a *ModelReusableAgent) SetModelSavePath(path string) {
	a.modelSavePath = path
}

func (a *ModelReusableAgent) GetModelSavePath() string {
	return a.modelSavePath
}
