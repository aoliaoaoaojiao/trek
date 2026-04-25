package reuse

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"sync"
	"time"
	"trek/internal/engine/decision"
	sharedgraph "trek/internal/engine/decision/shared/graph"
	"trek/internal/engine/decision/shared/tool"
	"trek/internal/engine/decision/shared/types"
	"trek/logger"

	"github.com/vmihailenco/msgpack/v5"
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

type reuseModelSnapshot struct {
	ActionStatistics map[uint64]map[string]int `msgpack:"action_statistics"`
	QvalueStatistics map[uint64]float64        `msgpack:"qvalue_statistics"`
}

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

func (a *ModelReusableAgent) Stop() {
	a.SaveReuseModel() // 闂備線娼荤拹鐔煎礉鐏炲墽顩烽柣妯烘▕濞间即鏌曡箛鏇炐㈡い銈呮噹鑿愰柛銉ｅ妿缁犳捇鏌ｆ幊閸斿矁鐏掗梺鐐藉劚绾绢參鍩€?
	a.stopOnce.Do(func() {
		close(a.stopChan)
	})
}

var createReuseAgent = func(m *sharedgraph.Model, deviceType types.DeviceType) (types.IAgent, error) {
	reuseAgent := NewModelReusableAgent(m)

	reuseAgent.LoadReuseModel()

	// 闁告凹鍨版慨鈺冣偓瑙勭濡炲倿鎳涢鍕楀ǎ鍥ㄧ箓閻°劑宕¤箛鏇楁煠闁挎稑鐭傛导鈺呭礂瀹ュ姣愰柡鍐ㄧ埣濡寧娼婚幇顖ｆ斀闁哄啳鍩栬啯闁搞劌顑囨慨鎼佸箑娴ｉ攱涓㈠鏈电┒閳?
	go func() {
		ticker := time.NewTicker(10 * time.Minute) // 婵?0闂備礁鎲＄敮鎺懳涘┑瀣闁规儳顕埞宥嗙節闂堟稒顥犻柟鐣屽Т閳藉骞橀姘婵?
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if err := reuseAgent.SaveReuseModel(); err != nil {
					logger.Errorf("Failed to auto-save reuse model: %v", err)
				}
			case <-reuseAgent.stopChan:
				logger.Info("Stopping reuse model auto-save routine")
				return
			}
		}
	}()

	return reuseAgent, nil
}

func init() {
	sharedgraph.RegisterAgentCreator(decision.AlgorithmReuse.String(), createReuseAgent)
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

func (a *ModelReusableAgent) UpdateStrategy() {
	if a.newAction == nil {
		return
	}

	if len(a.previousActions) > 0 {
		a.computeRewardOfLatestAction()
		a.updateReuseModel()
		value := a.getQValue(a.newAction)

		for i := len(a.previousActions) - 1; i >= 0; i-- {
			currentQValue := a.getQValue(a.previousActions[i])
			currentRewardValue := a.rewardCache[i]

			value = currentRewardValue + SarsaRLDefaultGamma*value

			if i == 0 {
				a.setQValue(a.previousActions[i], currentQValue+a.alpha*(value-currentQValue))
			}
		}
	} else {
		logger.Debugf("get action value failed!")
	}

	a.previousActions = append(a.previousActions, a.newAction)

	if len(a.previousActions) > SarsaNStep {
		//a.previousActions[0]
		a.previousActions = a.previousActions[1:]

	}
}

func (a *ModelReusableAgent) MoveForward(nextState types.IState) {
	a.lastState = a.currentState
	a.currentState = a.newState
	a.newState = nextState

	a.lastAction = a.currentAction
	a.currentAction = a.newAction
	a.newAction = nil

	//
	//if a.lastState != nil {
	//	lastStateHash = a.lastState.Hash()
	//}
	//if a.currentState != nil {
	//	currentStateHash = a.currentState.Hash()
	//}
	//if a.newState != nil {
	//	newStateHash = a.newState.Hash()
	//}

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

func (a *ModelReusableAgent) SelectNewAction() types.IAction {
	var action types.IAction

	action = a.selectUnperformedActionNotInReuseModel()
	if action != nil {
		logger.Debugf("select action not in reuse model")
		return action
	}

	action = a.selectUnperformedActionInReuseModel()
	if action != nil {
		logger.Infof("select action in reuse model")
		return action
	}

	action = RandomPickUnvisitedAction(a.newState)
	if action != nil {
		logger.Infof("select action in unvisited action")
		return action
	}

	action = a.selectActionByQValue()
	if action != nil {
		logger.Infof("select action by qvalue")
		return action
	}

	action = a.selectNewActionEpsilonGreedyRandomly()
	if action != nil {
		logger.Infof("select action by EpsilonGreedyRandom")
		return action
	}

	logger.Errorf("null action happened , handle null action")
	nullAction := a.handleNullAction()
	if nullAction != nil {
		return nullAction
	}
	return nil
}

func (a *ModelReusableAgent) computeAlphaValue() {
	if a.newState == nil {
		return
	}

	totalVisitCount := a.visitStats.Total()

	movingAlpha := 0.5
	if totalVisitCount > 20000 {
		movingAlpha -= 0.1
	}
	if totalVisitCount > 50000 {
		movingAlpha -= 0.1
	}
	if totalVisitCount > 100000 {
		movingAlpha -= 0.1
	}
	if totalVisitCount > 250000 {
		movingAlpha -= 0.1
	}

	a.alpha = math.Max(SarsaRLDefaultAlpha, movingAlpha)
}

func (a *ModelReusableAgent) computeRewardOfLatestAction() float64 {
	rewardValue := 0.0

	if a.newState == nil {
		logger.Error("computeReward: newState is null")
		return rewardValue
	}

	a.computeAlphaValue()
	visitedPages := a.visitStats.SnapshotPages()
	logger.Debugf("computeReward: visitedPages count %d", len(visitedPages))

	if len(a.previousActions) > 0 {
		lastSelectedAction := a.previousActions[len(a.previousActions)-1].(*types.StatefulAction)
		logger.Debugf("computeReward: lastSelectedAction %s %d", lastSelectedAction.GetId(), int(lastSelectedAction.GetVisitedCount()))

		rewardValue = a.probabilityOfVisitingNewActivities(lastSelectedAction, visitedPages)
		logger.Debugf("computeReward: probabilityOfVisitingNewActivities %f", rewardValue)

		if math.Abs(rewardValue-0.0) < 0.0001 {
			rewardValue = 1.0
			logger.Debugf("computeReward: New action detected, setting reward to 1.0")

		}

		rewardValue = rewardValue / math.Sqrt(float64(lastSelectedAction.GetVisitedCount())+1.0)
		logger.Debugf("computeReward: reward after visit count adjustment %f", rewardValue)

		rewardValue = rewardValue + a.getStateActionExpectationValue(a.newState, visitedPages)/math.Sqrt(float64(a.newState.GetVisitedCount())+1.0)
		logger.Debugf("computeReward: final reward after state expectation %f", rewardValue)

		logger.Debugf("total visited count %d", len(visitedPages))
	}

	logger.Infof("total visited ViewController count is %d", len(visitedPages))
	logger.Debugf("reuse-cov-opti action reward=%f", rewardValue)

	a.rewardCache = append(a.rewardCache, rewardValue)

	if len(a.rewardCache) > SarsaNStep {
		//removedReward := a.rewardCache[0]
		a.rewardCache = a.rewardCache[1:]
	}

	return rewardValue
}

func (a *ModelReusableAgent) probabilityOfVisitingNewActivities(action *types.StatefulAction, visitedActivities map[string]struct{}) float64 {
	value := 0.0
	total := 0
	unvisited := 0

	actionMapIterator := a.reuseModel[action.Hash()]
	if actionMapIterator != nil {
		for pageName, count := range actionMapIterator {
			total += count
			if _, exists := visitedActivities[pageName]; !exists {
				unvisited += count
			}
		}

		if total > 0 && unvisited > 0 {
			value = float64(unvisited) / float64(total)
		}
	}

	return value
}

func (a *ModelReusableAgent) getStateActionExpectationValue(state types.IState, visitedActivities map[string]struct{}) float64 {
	value := 0.0

	for _, action := range state.GetActions() {
		actionHash := action.Hash()

		if _, exists := a.reuseModel[actionHash]; !exists {
			value += 1.0
		} else if action.GetVisitedCount() >= 1 {
			value += 0.5
		}

		if action.GetTarget() != nil {
			value += a.probabilityOfVisitingNewActivities(action, visitedActivities)
		}
	}

	return value
}

func (a *ModelReusableAgent) getQValue(action types.IAction) float64 {
	if action == nil {
		return 0
	}
	return a.getQValueByHash(action.Hash())
}

func (a *ModelReusableAgent) setQValue(action types.IAction, qValue float64) {
	if action == nil {
		return
	}
	a.reuseQValue[action.Hash()] = qValue
}

func (a *ModelReusableAgent) getQValueByHash(actionHash uintptr) float64 {
	if a.reuseQValue == nil {
		return 0
	}
	if qv, ok := a.reuseQValue[actionHash]; ok {
		return qv
	}
	return 0
}

func (a *ModelReusableAgent) updateReuseModel() {
	if len(a.previousActions) == 0 {

		return
	}

	lastAction := a.previousActions[len(a.previousActions)-1]

	modelAction, ok := lastAction.(*types.StatefulAction)

	if !ok || a.newState == nil {

		return
	}

	hash := modelAction.Hash()
	pageName := a.newState.GetPageNameString()

	if pageName == "" {

		return
	}

	a.reuseModelLock.Lock()
	defer a.reuseModelLock.Unlock()

	entryMap := a.reuseModel[hash]
	if entryMap == nil {
		logger.Debugf("can not find action in reuse map")
		entryMap = make(PageVisitCount)
		a.reuseModel[hash] = entryMap
	} else {
		//entryMap[pageName]
		entryMap[pageName]++
	}

	a.reuseQValue[hash] = a.getQValueByHash(hash)
	logger.Debugf("Updated Q-value for action %s: %.6f",
		modelAction.GetId(),
		a.getQValueByHash(hash))
}

func (a *ModelReusableAgent) selectUnperformedActionNotInReuseModel() types.IAction {
	var actionsNotInModel []types.IAction

	for _, action := range a.newState.GetActions() {
		if action.IsModelAct() {
			actionHash := action.Hash()
			if _, exists := a.reuseModel[actionHash]; !exists {
				if action.GetVisitedCount() <= 0 {
					actionsNotInModel = append(actionsNotInModel, action)
				}
			}
		}
	}

	if len(actionsNotInModel) == 0 {
		return nil
	}

	totalWeight := 0
	for _, action := range actionsNotInModel {
		totalWeight += int(action.GetPriority())
	}

	if totalWeight <= 0 {
		logger.Errorf("total weights is 0")
		return nil
	}

	randI := tool.RandomInt(0, totalWeight)
	for _, action := range actionsNotInModel {
		priority := int(action.GetPriority())
		if randI < priority {
			return action
		}
		randI -= priority
	}

	logger.Errorf("rand a null action")
	return nil
}

func (a *ModelReusableAgent) selectUnperformedActionInReuseModel() types.IAction {
	var nextAction types.IAction
	maxValue := -math.MaxFloat64

	for _, action := range a.newState.TargetActions() {
		actionHash := action.Hash()

		if _, exists := a.reuseModel[actionHash]; exists {
			if action.GetVisitedCount() > 0 {
				logger.Debugf("action has been visited")
				continue
			}

			visitedActivities := a.visitStats.SnapshotPages()

			qualityValue := a.probabilityOfVisitingNewActivities(action, visitedActivities)

			if qualityValue > 1e-4 {
				qualityValue = 10.0 * qualityValue

				uniform := float64(tool.RandomInt(0, 10)) / 10.0
				if uniform < math.SmallestNonzeroFloat64 {
					uniform = math.SmallestNonzeroFloat64
				}

				qualityValue -= math.Log(-math.Log(uniform))

				if qualityValue > maxValue {
					maxValue = qualityValue
					nextAction = action
				}
			}
		}
	}

	return nextAction
}

func (a *ModelReusableAgent) selectActionByQValue() types.IAction {
	var returnAction types.IAction
	maxQ := -math.MaxFloat64

	visitedActivities := a.visitStats.SnapshotPages()

	for _, action := range a.newState.GetActions() {
		qv := 0.0
		actionHash := action.Hash()

		if action.GetVisitedCount() <= 0 {
			if _, exists := a.reuseModel[actionHash]; exists {
				qv += a.probabilityOfVisitingNewActivities(action, visitedActivities)
			} else {
				logger.Debugf("qvalue pick return a action: %s", action.String())
				return action
			}
		}

		qv += a.getQValue(action)
		qv /= EntropyAlpha

		uniform := float64(tool.RandomInt(0, 10)) / 10.0
		if uniform < math.SmallestNonzeroFloat64 {
			uniform = math.SmallestNonzeroFloat64
		}
		qv -= math.Log(-math.Log(uniform))

		if qv > maxQ {
			maxQ = qv
			returnAction = action
		}
	}

	return returnAction
}

func (a *ModelReusableAgent) selectNewActionEpsilonGreedyRandomly() types.IAction {
	if a.eGreedy() {
		logger.Debugf("Try to select the max value action")
		action := GreedyPickAction(a.newState, a.qValueFilter)
		if action != nil {

		} else {

		}
		return action
	}
	logger.Debugf("Try to randomly select a value action.")
	action := a.newStateRandomPickAction(a.qValueFilter)
	if action != nil {

	} else {

	}
	return action
}

func (a *ModelReusableAgent) eGreedy() bool {
	rand.Seed(time.Now().UnixNano())
	r := float64(rand.Intn(100)) / 100.0

	if r < a.epsilon {
		return false
	}
	return true
}

func (a *ModelReusableAgent) LoadReuseModel() {
	logger.Infof("begin load model: %s", a.modelSavePath)

	a.reuseModelLock.Lock()
	defer a.reuseModelLock.Unlock()

	data, err := os.ReadFile(a.modelSavePath)
	if err != nil {
		logger.Errorf("Failed to read reuse model file: %v", err)
		return
	}

	snapshot := &reuseModelSnapshot{}
	if err = msgpack.Unmarshal(data, snapshot); err != nil {
		logger.Errorf("Failed to unmarshal reuse model (msgpack): %v", err)
		return
	}

	a.reuseModel = make(ActionPageStatistics)
	a.reuseQValue = make(ActionQValue)

	for actionID, pageVisitCount := range snapshot.ActionStatistics {
		reuseEntryM := make(PageVisitCount)
		for pageID, count := range pageVisitCount {
			reuseEntryM[pageID] = count
		}
		a.reuseModel[uintptr(actionID)] = reuseEntryM
	}

	for actionID, qvalue := range snapshot.QvalueStatistics {
		a.reuseQValue[uintptr(actionID)] = qvalue
	}

	logger.Infof("Successfully loaded reuse model from %s", a.modelSavePath)
}

func (a *ModelReusableAgent) SaveReuseModel() error {
	outputFilePath := a.modelSavePath
	logger.Infof("save model to path: %s", outputFilePath)

	a.reuseModelLock.Lock()
	defer a.reuseModelLock.Unlock()

	snapshot := &reuseModelSnapshot{
		ActionStatistics: make(map[uint64]map[string]int),
		QvalueStatistics: make(map[uint64]float64),
	}

	for actionID, entryMap := range a.reuseModel {
		if entryMap == nil {
			continue
		}
		pageVisits := make(map[string]int)
		for pageID, count := range entryMap {
			pageVisits[pageID] = count
		}
		snapshot.ActionStatistics[uint64(actionID)] = pageVisits
	}

	for actionID, qvalue := range a.reuseQValue {
		snapshot.QvalueStatistics[uint64(actionID)] = qvalue
	}

	data, err := msgpack.Marshal(snapshot)
	if err != nil {
		logger.Errorf("Failed to marshal reuse model (msgpack): %v", err)
		return err
	}

	if err := os.WriteFile(outputFilePath, data, 0644); err != nil {
		logger.Errorf("Failed to save reuse model to file: %v", err)
		return err
	}

	logger.Infof("Successfully saved reuse model to %s", outputFilePath)
	return nil
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
