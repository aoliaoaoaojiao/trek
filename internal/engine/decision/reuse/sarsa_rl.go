package reuse

import (
	"math"

	"trek/internal/engine/core/types"
	"trek/logger"
)

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

			value = currentRewardValue + a.gamma*value

			if i == 0 {
				a.setQValue(a.previousActions[i], currentQValue+a.alpha*(value-currentQValue))
			}
		}
	} else {
		logger.Debugf("get action value failed!")
	}

	a.previousActions = append(a.previousActions, a.newAction)

	if len(a.previousActions) > a.nStep {
		//a.previousActions[0]
		a.previousActions = a.previousActions[1:]

	}
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
			// probability = 0: genuinely new action (not in reuseModel) vs all pages visited
			if a.reuseModel[lastSelectedAction.Hash()] == nil {
				rewardValue = 1.0
				logger.Debugf("computeReward: New action detected (not in reuse model), setting reward to 1.0")
			} else {
				logger.Debugf("computeReward: all associated pages visited, no new action bonus")
			}
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

	if len(a.rewardCache) > a.nStep {
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
	}
	entryMap[pageName]++

	a.reuseQValue[hash] = a.getQValueByHash(hash)
	logger.Debugf("Updated Q-value for action %s: %.6f",
		modelAction.GetId(),
		a.getQValueByHash(hash))
}
