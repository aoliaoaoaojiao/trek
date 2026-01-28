package reuse

import (
	types2 "trek/internal/core/types"
	"trek/log"
)

// RichWidget 使用动作、类名、资源ID、自身（或其子元素）的文本
// 来嵌入和生成哈希码，用于标识widget
type RichWidget struct {
	types2.Widget
	WidgetHashcode uintptr
}

// NewRichWidget 创建新的RichWidget
func NewRichWidget(parent *RichWidget, element types2.IElement) *RichWidget {
	// 首先创建基础Widget
	var parentWidget *types2.Widget
	if parent != nil {
		parentWidget = &parent.Widget
	}
	baseWidget := types2.NewWidget(parentWidget, element)

	rw := &RichWidget{
		Widget:         *baseWidget,
		WidgetHashcode: 0,
	}

	// 计算RichWidget的哈希码
	hashcode1 := rw.GetElementIdentifier()
	//hashcode2 := HashString(rw.ResourceID)

	// 计算动作哈希码
	hashcode3 := uintptr(0x1)
	actions := rw.GetActions()
	for _, action := range actions {
		actionHash := uintptr(HashInt(int(action)))
		hashcode3 ^= (uintptr(127) * actionHash)
	}

	// 计算widget哈希码
	//rw.WidgetHashcode = ((hashcode1 ^ (hashcode2 << 4)) >> 2) ^ ((uintptr(127) * hashcode3) << 1)
	rw.WidgetHashcode = (hashcode1 >> 2) ^ ((uintptr(127) * hashcode3) << 1)

	// 获取有效的文本
	elementText := rw.getValidTextFromWidgetAndChildren(element)
	if elementText != "" {
		textHash := HashString(elementText)
		rw.WidgetHashcode ^= (0x79b9 + (textHash << 1))
	}

	log.Debugf("RichWidget created with hashcode:%d, identifier:%s, text:%s",
		rw.WidgetHashcode, rw.GetElementIdentifier(), elementText)

	//rw.Widget.Hashcode = rw.Hashcode

	return rw
}

// getValidTextFromWidgetAndChildren 从widget及其子元素获取有效文本
// 如果父widget不可点击，则获取子元素的有效文本
func (rw *RichWidget) getValidTextFromWidgetAndChildren(element types2.IElement) string {
	txt := element.GetText()
	log.Debugf("Getting valid text from element, current text: %s", txt)
	if txt == "" {
		for _, child := range element.GetChildren() {
			txt = rw.getValidTextFromWidgetAndChildren(child)
			if txt != "" {
				log.Debugf("Found valid text in child: %s", txt)
				return txt
			}
		}
	}
	return txt
}

// Hash 重写Hash方法，返回RichWidget的哈希码
func (rw *RichWidget) Hash() uintptr {

	return rw.WidgetHashcode
}

// GetActHashCode 获取动作哈希码
func (rw *RichWidget) GetActHashCode() uintptr {
	return rw.WidgetHashcode
}
