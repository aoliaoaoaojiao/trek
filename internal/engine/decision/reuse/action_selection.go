package reuse

import (
	"math"
	"math/rand"

	"trek/internal/engine/core/types"
	"trek/internal/engine/decision/shared/tool"
	"trek/logger"
)

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

	visitedActivities := a.visitStats.SnapshotPages()

	for _, action := range a.newState.TargetActions() {
		actionHash := action.Hash()

		if _, exists := a.reuseModel[actionHash]; exists {
			if action.GetVisitedCount() > 0 {
				logger.Debugf("action has been visited")
				continue
			}

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
	r := float64(rand.Intn(100)) / 100.0

	if r < a.epsilon {
		return false
	}
	return true
}
