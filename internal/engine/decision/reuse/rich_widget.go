package reuse

import (
	"trek/internal/engine/core/types"
	"trek/internal/engine/decision/shared/tool"
	"trek/logger"
)

type RichWidget struct {
	types.Widget
	WidgetHashcode uintptr
}

func NewRichWidget(parent *RichWidget, element types.IElement) *RichWidget {
	var parentWidget *types.Widget
	if parent != nil {
		parentWidget = &parent.Widget
	}
	baseWidget := types.NewWidget(parentWidget, element)

	rw := &RichWidget{
		Widget:         *baseWidget,
		WidgetHashcode: 0,
	}

	hashcode1 := rw.GetElementIdentifier()
	hashcode3 := uintptr(0x1)
	for _, action := range rw.GetActions() {
		hashcode3 ^= (uintptr(127) * uintptr(tool.HashInt(int(action))))
	}

	rw.WidgetHashcode = (hashcode1 >> 2) ^ ((uintptr(127) * hashcode3) << 1)

	elementText := rw.getValidTextFromWidgetAndChildren(element)
	if elementText != "" {
		textHash := tool.HashString(elementText)
		rw.WidgetHashcode ^= (0x79b9 + (textHash << 1))
	}

	logger.Debugf("RichWidget created with hashcode:%d, identifier:%s, text:%s",
		rw.WidgetHashcode, rw.GetElementIdentifier(), elementText)
	return rw
}

func (rw *RichWidget) getValidTextFromWidgetAndChildren(element types.IElement) string {
	txt := element.GetText()
	if txt == "" {
		for _, child := range element.GetChildren() {
			txt = rw.getValidTextFromWidgetAndChildren(child)
			if txt != "" {
				return txt
			}
		}
	}
	return txt
}

func (rw *RichWidget) Hash() uintptr {
	return rw.WidgetHashcode
}

func (rw *RichWidget) GetActHashCode() uintptr {
	return rw.WidgetHashcode
}
