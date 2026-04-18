package xml

import (
	oldelements "trek/internal/engine/core/types/elements"
	"trek/internal/engine/core/types"
)

type ElementType = oldelements.ElementType

const (
	ANDROID_ELEMENT = oldelements.ANDROID_ELEMENT
)

type AndroidElement = oldelements.AndroidElement

func CreateAndroidElement(tag string) (types.IElement, error) {
	return oldelements.CreateAndroidElement(tag)
}

func CreateAndroidElementFromXml(xmlContent string) (types.IElement, error) {
	return oldelements.CreateAndroidElementFromXml(xmlContent)
}

func NewAndroidElement() *AndroidElement {
	return oldelements.NewAndroidElement()
}
