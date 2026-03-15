package run

import (
	"trek/internal/engine/core/model"
	types2 "trek/internal/engine/core/types"
	"trek/internal/engine/core/types/elements"
)

const FASTBOT_VERSION = "1.0.0-go"

var _fastbotModel *model.Model

func GetAction(activity string, xmlDescOfGuiTree string) string {

	if _fastbotModel == nil {
		_fastbotModel = model.NewModel(activity)
	}

	operationString := _fastbotModel.GetOperate(string(elements.ANDROID_ELEMENT), xmlDescOfGuiTree, activity, "")
	return operationString
}

func InitAgent(agentType types2.AlgorithmType, packageName string, deviceType types2.DeviceType) {

	if _fastbotModel == nil {
		_fastbotModel = model.NewModel(packageName)
	}

	_fastbotModel.AddAgent(model.DefaultDeviceID, agentType.String(), deviceType)
	_fastbotModel.SetPackageName(packageName)

}

func LoadResMapping(resMappingFilepath string) {

	if _fastbotModel == nil {

		_fastbotModel = model.NewModel("")
	}

	preference := _fastbotModel.GetPreference()
	if preference != nil {
		preference.LoadMixResMapping(resMappingFilepath)
	}

}

func CheckPointIsInBlackRects(activity string, pointX float32, pointY float32) bool {

	if _fastbotModel == nil {
		return false
	}

	preference := _fastbotModel.GetPreference()
	if preference == nil {
		return false
	}

	isShield := preference.CheckPointIsInBlackRects(activity, int(pointX), int(pointY))
	return isShield
}

func GetNativeVersion() string {

	return FASTBOT_VERSION
}

func GetModel() *model.Model {
	return _fastbotModel
}

func ResetModel() {
	_fastbotModel = nil
}
