package reuse

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"sync"
	"time"
	protoPkg "trek/internal/engine/algorithm/reuse/proto"
	"trek/internal/engine/core/model"
	"trek/internal/engine/core/types"
	"trek/internal/engine/tool"
	"trek/log"

	"google.golang.org/protobuf/proto"
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
	model                           *model.Model
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
	modelSavePath   string
	reuseModelLock  sync.Mutex

	stopChan chan struct{}
	stopOnce sync.Once
}

func (a *ModelReusableAgent) Stop() {
	a.SaveReuseModel() // 在停止前保存模型
	a.stopOnce.Do(func() {
		close(a.stopChan)
	})
}

var createReuseAgent = func(m *model.Model, deviceType types.DeviceType) (types.IAgent, error) {
	reuseAgent := NewModelReusableAgent(m)

	reuseAgent.LoadReuseModel()

	// 启动定时保存模型的goroutine
	go func() {
		ticker := time.NewTicker(10 * time.Minute) // 每10分钟保存一次
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if err := reuseAgent.SaveReuseModel(); err != nil {
					log.Errorf("Failed to auto-save reuse model: %v", err)
				}
			case <-reuseAgent.stopChan:
				log.Info("Stopping reuse model auto-save routine")
				return
			}
		}
	}()

	return reuseAgent, nil
}

func init() {
	model.RegisterAgentCreator(types.Reuse.String(), createReuseAgent)
}

func NewModelReusableAgent(model *model.Model) *ModelReusableAgent {
	agent := &ModelReusableAgent{
		validateFilter:                  types.NewActionFilterValidDatePriority(),
		graphStableCounter:              0,
		stateStableCounter:              0,
		pageNameStableCounter:           0,
		disableFuzz:                     false,
		requestRestart:                  false,
		appPageNameJustStartedFromClean: false,
		appPageNameJustStarted:          false,
		currentStateRecovered:           false,
		currentStateBlockTimes:          0,
		algorithmType:                   types.Reuse.String(),
		model:                           model,
		alpha:                           SarsaRLDefaultAlpha,
		epsilon:                         SarsaRLDefaultEpsilon,
		rewardCache:                     make([]float64, 0),
		previousActions:                 make([]types.IAction, 0),
		reuseModel:                      make(ActionPageStatistics),
		reuseQValue:                     make(ActionQValue),
		stopChan:                        make(chan struct{}),
	}
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
		log.Debugf("get action value failed!")
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

	//var lastStateHash, currentStateHash, newStateHash uintptr
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

func (a *ModelReusableAgent) GetModel() *model.Model {
	return a.model
}

func (a *ModelReusableAgent) SetModel(model *model.Model) {
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
	return a.newState.RandomPickAction(filter, true)
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
		log.Infof("select action not in reuse model")
		return action
	}

	action = a.selectUnperformedActionInReuseModel()
	if action != nil {
		log.Infof("select action in reuse model")
		return action
	}

	action = a.newState.RandomPickUnvisitedAction()
	if action != nil {
		log.Infof("select action in unvisited action")
		return action
	}

	action = a.selectActionByQValue()
	if action != nil {
		log.Infof("select action by qvalue")
		return action
	}

	action = a.selectNewActionEpsilonGreedyRandomly()
	if action != nil {
		log.Infof("select action by EpsilonGreedyRandom")
		return action
	}

	log.Errorf("null action happened , handle null action")
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

	graphRef := a.model.GetGraph()
	totalVisitCount := graphRef.GetTotalDistri()

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
		log.Error("computeReward: newState is null")
		return rewardValue
	}

	a.computeAlphaValue()
	graphRef := a.model.GetGraph()
	visitedPages := graphRef.GetVisitedPages()
	log.Debugf("computeReward: visitedPages count %d", len(visitedPages))

	if len(a.previousActions) > 0 {
		lastSelectedAction := a.previousActions[len(a.previousActions)-1].(*types.StatefulAction)
		log.Debugf("computeReward: lastSelectedAction %s %d", lastSelectedAction.GetId(), int(lastSelectedAction.GetVisitedCount()))

		rewardValue = a.probabilityOfVisitingNewActivities(lastSelectedAction, visitedPages)
		log.Debugf("computeReward: probabilityOfVisitingNewActivities %f", rewardValue)

		if math.Abs(rewardValue-0.0) < 0.0001 {
			rewardValue = 1.0
			log.Debugf("computeReward: New action detected, setting reward to 1.0")

		}

		rewardValue = rewardValue / math.Sqrt(float64(lastSelectedAction.GetVisitedCount())+1.0)
		log.Debugf("computeReward: reward after visit count adjustment %f", rewardValue)

		rewardValue = rewardValue + a.getStateActionExpectationValue(a.newState, visitedPages)/math.Sqrt(float64(a.newState.GetVisitedCount())+1.0)
		log.Debugf("computeReward: final reward after state expectation %f", rewardValue)

		log.Debugf("total visited count %d", len(visitedPages))
	}

	log.Infof("total visited ViewController count is %d", len(visitedPages))
	log.Debugf("reuse-cov-opti action reward=%f", rewardValue)

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
	return action.GetQValue()
}

func (a *ModelReusableAgent) setQValue(action types.IAction, qValue float64) {
	action.SetQValue(qValue)
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
		log.Debugf("can not find action in reuse map")
		entryMap = make(PageVisitCount)
		a.reuseModel[hash] = entryMap
	} else {
		//entryMap[pageName]
		entryMap[pageName]++
	}

	a.reuseQValue[hash] = modelAction.GetQValue()
	log.Debugf("Updated Q-value for action %s: %.6f",
		modelAction.GetId(),
		modelAction.GetQValue())
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
		log.Errorf("total weights is 0")
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

	log.Errorf("rand a null action")
	return nil
}

func (a *ModelReusableAgent) selectUnperformedActionInReuseModel() types.IAction {
	var nextAction types.IAction
	maxValue := -math.MaxFloat64

	for _, action := range a.newState.TargetActions() {
		actionHash := action.Hash()

		if _, exists := a.reuseModel[actionHash]; exists {
			if action.GetVisitedCount() > 0 {
				log.Debugf("action has been visited")
				continue
			}

			graphRef := a.model.GetGraph()
			visitedActivities := graphRef.GetVisitedPages()

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

	graphRef := a.model.GetGraph()
	visitedActivities := graphRef.GetVisitedPages()

	for _, action := range a.newState.GetActions() {
		qv := 0.0
		actionHash := action.Hash()

		if action.GetVisitedCount() <= 0 {
			if _, exists := a.reuseModel[actionHash]; exists {
				qv += a.probabilityOfVisitingNewActivities(action, visitedActivities)
			} else {
				log.Debugf("qvalue pick return a action: %s", action.String())
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
		log.Debugf("Try to select the max value action")
		action := a.newState.GreedyPickAction(types.EnableValidValuePriorityFilter)
		if action != nil {

		} else {

		}
		return action
	}
	log.Debugf("Try to randomly select a value action.")
	action := a.newStateRandomPickAction(types.EnableValidValuePriorityFilter)
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
	log.Infof("begin load model: %s", a.modelSavePath)

	a.reuseModelLock.Lock()
	defer a.reuseModelLock.Unlock()

	// 读取文件
	data, err := os.ReadFile(a.modelSavePath)
	if err != nil {
		log.Errorf("Failed to read reuse model file: %v", err)
		return
	}

	// 解析protobuf数据
	fullModel := &protoPkg.ReuseModel{}
	err = proto.Unmarshal(data, fullModel)
	if err != nil {
		log.Errorf("Failed to unmarshal reuse model: %v", err)
		return
	}

	// 将protobuf数据转换为内存数据结构
	if fullModel.ActionStatistics != nil {
		for actionId, pageVisitCount := range fullModel.ActionStatistics.ActionPages {
			if pageVisitCount != nil {
				reuseEntryM := make(PageVisitCount)
				for pageId, count := range pageVisitCount.PageVisits {
					reuseEntryM[pageId] = int(count)
				}
				a.reuseModel[uintptr(actionId)] = reuseEntryM
			}
		}
	}

	if fullModel.QvalueStatistics != nil {
		for actionId, qvalue := range fullModel.QvalueStatistics.ActionQvalues {
			a.reuseQValue[uintptr(actionId)] = qvalue
		}
	}

	log.Infof("Successfully loaded reuse model from %s", a.modelSavePath)
}

func (a *ModelReusableAgent) SaveReuseModel() error {

	outputFilePath := a.modelSavePath

	log.Infof("save model to path: %s", outputFilePath)

	a.reuseModelLock.Lock()
	defer a.reuseModelLock.Unlock()

	// 将内存数据结构转换为protobuf格式
	actionStatistics := &protoPkg.ActionPageStatistics{
		ActionPages: make(map[uint64]*protoPkg.PageVisitCount),
	}

	for actionId, entryMap := range a.reuseModel {
		if entryMap != nil {
			pageVisitCount := &protoPkg.PageVisitCount{
				PageVisits: make(map[string]int32),
			}
			for pageId, count := range entryMap {
				pageVisitCount.PageVisits[pageId] = int32(count)
			}
			actionStatistics.ActionPages[uint64(actionId)] = pageVisitCount
		}
	}

	qvalueStatistics := &protoPkg.ActionQValue{
		ActionQvalues: make(map[uint64]float64),
	}

	for actionId, qvalue := range a.reuseQValue {
		qvalueStatistics.ActionQvalues[uint64(actionId)] = qvalue
	}

	// 创建完整的模型
	fullModel := &protoPkg.ReuseModel{
		ActionStatistics: actionStatistics,
		QvalueStatistics: qvalueStatistics,
	}

	// 序列化为protobuf格式
	data, err := proto.Marshal(fullModel)
	if err != nil {
		log.Errorf("Failed to marshal reuse model: %v", err)
		return err
	}

	// 写入文件
	if err := os.WriteFile(outputFilePath, data, 0644); err != nil {
		log.Errorf("Failed to save reuse model to file: %v", err)
		return err
	}

	log.Infof("Successfully saved reuse model to %s", outputFilePath)
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
