package types

import (
	"fmt"
	"sort"
	"strings"
	"trek/internal/engine/core/tool"
)

var _ IWidget = (*Widget)(nil)

type Widget struct {
	Hashcode uintptr
	Parent   IWidget
	Text     string

	path  string
	xpath string

	Enabled     bool
	Editable    bool
	OperateMask OperateType
	Bounds      *Rect
	ContextDesc string
	Actions     map[ActionType]bool

	ScrollTypeField         ScrollType
	elementSimpleIdentifier uintptr
}

const (
	STATE_WITH_TEXT = false

	STATE_TEXT_MAX_LEN = 6

	STATE_WITH_INDEX = false

	SCROLL_BOTTOM_UP_N_ENABLE = false

	FORCE_EDITTEXT_CLICK_TRUE = true

	PARENT_CLICK_CHANGE_CHILDREN = true
)

func NewWidget(parent IWidget, element IElement) *Widget {
	widget := &Widget{
		Parent:          parent,
		Actions:         make(map[ActionType]bool),
		OperateMask:     None,
		Bounds:          NewRect(0, 0, 0, 0),
		ScrollTypeField: NONE,
	}

	if element != nil {
		widget.initFormElement(element)
		widget.preprocessText()
	}

	return widget
}

func (w *Widget) GetParent() IWidget {
	if w == nil || w.Parent == nil {
		return nil
	}
	return w.Parent
}

func (w *Widget) GetBounds() *Rect {
	if w == nil {
		return nil
	}
	return w.Bounds
}

func (w *Widget) GetActions() []ActionType {
	actions := make([]ActionType, 0, len(w.Actions))
	for action := range w.Actions {
		actions = append(actions, action)
	}
	sort.Slice(actions, func(i, j int) bool {
		return int(actions[i]) < int(actions[j])
	})
	return actions
}

func (w *Widget) GetText() string {
	if w == nil {
		return ""
	}
	return w.Text
}

func (w *Widget) GetEnabled() bool {
	if w == nil {
		return false
	}
	return w.Enabled
}

func (w *Widget) HasOperate(opt OperateType) bool {
	if w == nil {
		return false
	}
	return w.OperateMask&opt != 0
}

func (w *Widget) HasAction() bool {
	if w == nil {
		return false
	}
	return len(w.Actions) > 0
}

func (w *Widget) IsEditable() bool {
	if w == nil {
		return false
	}
	return w.Editable
}

func (w *Widget) Hash() uintptr {
	if w == nil {
		return 0
	}
	if w.Hashcode == 0 {
		hashcode3 := tool.HashInt(int(w.OperateMask))
		hashcode4 := tool.HashInt(int(w.ScrollTypeField))

		w.Hashcode = (w.elementSimpleIdentifier >> 2) ^
			(((127 * hashcode3 << 1) ^ (256 * hashcode4 << 3)) >> 1)

	}
	return w.Hashcode
}

func (w *Widget) GetElementIdentifier() uintptr {
	if w == nil {
		return 0
	}
	return w.elementSimpleIdentifier
}

func (w *Widget) String() string {
	if w == nil {
		return ""
	}
	text := strings.TrimSpace(w.Text)
	if text == "" {
		text = "<empty>"
	}
	contentDesc := strings.TrimSpace(w.ContextDesc)
	if contentDesc == "" {
		contentDesc = "<empty>"
	}
	if len(text) > 40 {
		text = text[:37] + "..."
	}
	if len(contentDesc) > 40 {
		contentDesc = contentDesc[:37] + "..."
	}
	actions := w.GetActions()
	actionNames := make([]string, 0, len(actions))
	for _, action := range actions {
		actionNames = append(actionNames, action.String())
	}
	return fmt.Sprintf("Widget{hash:%d, text:%q, contentDesc:%q, bounds:%s, enabled:%t, editable:%t, path:%s, xpath:%s, actions:[%s]}",
		w.Hash(), text, contentDesc, w.Bounds.String(), w.Enabled, w.Editable, w.path, w.xpath, strings.Join(actionNames, ","))
}

func (w *Widget) GetPath() string {
	if w == nil {
		return ""
	}
	return w.path
}

func (w *Widget) GetXPath() string {
	if w == nil {
		return ""
	}
	return w.xpath
}

func (w *Widget) ClearDetails() {
	w.Text = ""
	w.ContextDesc = ""
	w.Hashcode = 0
}

func (w *Widget) FillDetails(copy *Widget) {
	if copy == nil {
		return
	}
	w.Text = copy.Text
	w.ContextDesc = copy.ContextDesc
	w.Hashcode = copy.Hashcode
}

func (w *Widget) enableOperate(opt OperateType) {
	w.OperateMask |= opt
}

func (w *Widget) preprocessText() {

	var result strings.Builder
	for _, r := range w.Text {
		if !((r >= '0' && r <= '9') || r == ' ') {
			result.WriteRune(r)
		}
	}
	w.Text = result.String()

	if STATE_WITH_TEXT {
		overMaxLen := len(w.Text) > STATE_TEXT_MAX_LEN

		if len(w.Text) > STATE_TEXT_MAX_LEN*4 {
			w.Text = w.Text[:STATE_TEXT_MAX_LEN*4]
		}

		cutLength := STATE_TEXT_MAX_LEN

		if len(w.Text) > cutLength && cutLength < len(w.Text) && tool.IsZhCnByte(w.Text[STATE_TEXT_MAX_LEN]) {
			ci := 0
			for ci < cutLength {
				if tool.IsZhCnByte(w.Text[ci]) {
					ci += 2
				} else {
					ci += 1
				}
			}
			cutLength = ci
		}

		if cutLength < len(w.Text) {
			w.Text = w.Text[:cutLength]
		}

		if !overMaxLen {
			textHash := tool.HashString(w.Text)
			w.Hashcode ^= (0x79b9 + (textHash << 5))
		}
	}

}

func (w *Widget) initFormElement(element IElement) {
	w.Text = element.GetText()
	if contentDescCarrier, ok := element.(interface{ GetContentDesc() string }); ok {
		w.ContextDesc = contentDescCarrier.GetContentDesc()
	}
	w.elementSimpleIdentifier = element.GetIdentifierHash()
	w.path = element.GetPath()
	w.xpath = element.GetXPath()
	w.Enabled = element.GetEnable()
	w.Editable = element.GetEditable()
	w.Bounds = element.GetBounds()
	w.ScrollTypeField = element.GetScrollType()

	if element.GetCheckBoxable() {
		w.enableOperate(Checkable)
	}
	if element.GetEnable() {
		w.enableOperate(Enable)
	}
	if element.GetClickable() {
		w.enableOperate(Clickable)
	}
	if element.GetScrollType() != NONE {
		w.enableOperate(Scrollable)
	}
	if element.GetLongClickable() {
		w.enableOperate(LongClickable)
		w.Actions[LONG_CLICK] = true
	}
	if w.HasOperate(Checkable) || w.HasOperate(Clickable) {
		w.Actions[CLICK] = true
	}

	switch w.ScrollTypeField {
	case ALL:
		w.Actions[SCROLL_BOTTOM_UP] = true
		w.Actions[SCROLL_TOP_DOWN] = true
		w.Actions[SCROLL_LEFT_RIGHT] = true
		w.Actions[SCROLL_RIGHT_LEFT] = true
	case Horizontal:
		w.Actions[SCROLL_LEFT_RIGHT] = true
		w.Actions[SCROLL_RIGHT_LEFT] = true
	case Vertical:
		w.Actions[SCROLL_BOTTOM_UP] = true
		w.Actions[SCROLL_TOP_DOWN] = true
	case NONE:
	}

	if FORCE_EDITTEXT_CLICK_TRUE && w.Editable {
		w.enableOperate(Clickable)
		w.enableOperate(LongClickable)
		w.Actions[CLICK] = true
		w.Actions[LONG_CLICK] = true
	}

	w.Hash()

}

type WidgetList []IWidget

type WidgetSet map[uintptr]IWidget

type WidgetListMap map[uintptr]WidgetList

func (s WidgetSet) Add(widget IWidget) {
	if widget != nil {
		s[widget.Hash()] = widget
	}
}

func (s WidgetSet) Remove(widget IWidget) {
	if widget != nil {
		delete(s, widget.Hash())
	}
}

func (s WidgetSet) Contains(widget IWidget) bool {
	if widget == nil {
		return false
	}
	_, exists := s[widget.Hash()]
	return exists
}

func (s WidgetSet) ToSlice() WidgetList {
	result := make(WidgetList, 0, len(s))
	for _, widget := range s {
		result = append(result, widget)
	}
	return result
}

func (m WidgetListMap) Add(hash uintptr, widget IWidget) {
	if m[hash] == nil {
		m[hash] = make(WidgetList, 0)
	}
	m[hash] = append(m[hash], widget)
}

func (m WidgetListMap) Get(hash uintptr) WidgetList {
	return m[hash]
}
