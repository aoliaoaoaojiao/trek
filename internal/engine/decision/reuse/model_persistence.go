package reuse

import (
	"os"

	"trek/logger"

	"github.com/vmihailenco/msgpack/v5"
)

type reuseModelSnapshot struct {
	ActionStatistics map[uint64]map[string]int `msgpack:"action_statistics"`
	QvalueStatistics map[uint64]float64        `msgpack:"qvalue_statistics"`
}

func (a *ModelReusableAgent) LoadReuseModel() {
	if !a.enableModelPersistence {
		logger.Infof("reuse model persistence disabled, skip load")
		return
	}
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
	if !a.enableModelPersistence {
		return nil
	}
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
