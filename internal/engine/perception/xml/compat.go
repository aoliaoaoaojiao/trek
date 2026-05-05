package xml

import (
	"trek/internal/engine/core/types"
	"trek/internal/engine/decision/shared/elements"
)

type ElementType = elements.ElementType

const (
	ANDROID_ELEMENT = elements.ANDROID_ELEMENT
)

type AndroidElement = elements.AndroidElement

func CreateAndroidElement(tag string) (types.IElement, error) {
	return elements.CreateAndroidElement(tag)
}

func CreateAndroidElementFromXml(xmlContent string) (types.IElement, error) {
	return elements.CreateAndroidElementFromXml(xmlContent)
}

func NewAndroidElement() *AndroidElement {
	return elements.NewAndroidElement()
}
