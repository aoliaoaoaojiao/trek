package types

import (
	"fmt"
	"sort"
	"strings"
	"trek/internal/engine/core/tool"
)

var _ IWidget = (*Widget)(nil)

// Widget Widget缁撴瀯锛屼娇鐢╓idget鍐呴儴鏂囨湰鏉ュ祵鍏ュ拰璇嗗埆Widget
type Widget struct {
	Hashcode uintptr
	Parent   IWidget
	Text     string

	path string

	Enabled     bool
	Editable    bool
	OperateMask OperateType
	Bounds      *Rect
	ContextDesc string
	Actions     map[ActionType]bool

	ScrollTypeField ScrollType // 婊氬姩绫诲瀷
	//ClassName       string
	//ResourceID      string
	elementSimpleIdentifier uintptr
}

// 鐘舵€佸鐞嗗父閲?

const (
	// STATE_WITH_TEXT 濡傛灉搴旇鍩轰簬鏂囨湰鐢熸垚鍝堝笇
	STATE_WITH_TEXT = false

	// STATE_TEXT_MAX_LEN 鍙備笌鎶借薄鐨勬枃鏈殑鏈€闀块暱搴︼紝閫氬父鏄?鐨勫€嶆暟
	// 瓒呰繃2涓腑鏂囧瓧绗﹀皢涓嶅弬涓庢娊璞★紝瀛楃闀垮害灏嗚鎴柇
	STATE_TEXT_MAX_LEN = 6 // 2*3

	// STATE_WITH_INDEX 濡傛灉搴旇鍩轰簬绱㈠紩鐢熸垚鍝堝笇
	STATE_WITH_INDEX = false

	// SCROLL_BOTTOM_UP_N_ENABLE 鏄惁鍚敤SCROLL_BOTTOM_UP_N鍔ㄤ綔
	SCROLL_BOTTOM_UP_N_ENABLE = false

	// FORCE_EDITTEXT_CLICK_TRUE 鏄惁寮哄埗涓篍ditText鍏冪礌娣诲姞鐐瑰嚮鍜岄暱鎸夊姛鑳?
	FORCE_EDITTEXT_CLICK_TRUE = true

	PARENT_CLICK_CHANGE_CHILDREN = true
)

// NewWidget 鍒涘缓鏂扮殑Widget
func NewWidget(parent IWidget, element IElement) *Widget {
	widget := &Widget{
		Parent:          parent,
		Actions:         make(map[ActionType]bool),
		OperateMask:     None,
		Bounds:          NewRect(0, 0, 0, 0),
		ScrollTypeField: NONE, // 鍒濆鍖栨粴鍔ㄧ被鍨?
	}

	if element != nil {
		widget.initFormElement(element)
		// 鏂囨湰棰勫鐞?
		widget.preprocessText()
	}

	return widget
}

// GetParent 鑾峰彇鐖禬idget
func (w *Widget) GetParent() IWidget {
	if w == nil || w.Parent == nil {
		return nil
	}
	return w.Parent
}

// GetBounds 鑾峰彇杈圭晫
func (w *Widget) GetBounds() *Rect {
	if w == nil {
		return nil
	}
	return w.Bounds
}

// GetActions 鑾峰彇鍔ㄤ綔闆嗗悎
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

// GetText 鑾峰彇鏂囨湰
func (w *Widget) GetText() string {
	if w == nil {
		return ""
	}
	return w.Text
}

// GetEnabled 鑾峰彇鏄惁鍚敤
func (w *Widget) GetEnabled() bool {
	if w == nil {
		return false
	}
	return w.Enabled
}

// HasOperate 妫€鏌ユ槸鍚︽湁鎸囧畾鎿嶄綔
func (w *Widget) HasOperate(opt OperateType) bool {
	if w == nil {
		return false
	}
	return w.OperateMask&opt != 0
}

// HasAction 妫€鏌ユ槸鍚︽湁鍔ㄤ綔
func (w *Widget) HasAction() bool {
	if w == nil {
		return false
	}
	return len(w.Actions) > 0
}

// IsEditable 妫€鏌ユ槸鍚﹀彲缂栬緫
func (w *Widget) IsEditable() bool {
	if w == nil {
		return false
	}
	return w.Editable
}

// Hash 璁＄畻鍝堝笇鍊?

func (w *Widget) Hash() uintptr {
	if w == nil {
		return 0
	}
	if w.Hashcode == 0 {
		// 鍩虹鍝堝笇璁＄畻
		//hashcode1 := utils.HashString(w.elementSimpleIdentifier)
		//hashcode2 := utils.HashString(w.ResourceID)
		hashcode3 := tool.HashInt(int(w.OperateMask))
		hashcode4 := tool.HashInt(int(w.ScrollTypeField))

		w.Hashcode = (w.elementSimpleIdentifier >> 2) ^
			(((127 * hashcode3 << 1) ^ (256 * hashcode4 << 3)) >> 1)

		//w.Hashcode = (hashcode1 >> 2) ^
		//	(((127 * hashcode3 << 1) ^ (256 * hashcode4 << 3)) >> 1)
	}
	return w.Hashcode
}

func (w *Widget) GetElementIdentifier() uintptr {
	if w == nil {
		return 0
	}
	return w.elementSimpleIdentifier
}

// String 杩斿洖瀛楃涓茶〃绀?

func (w *Widget) String() string {
	if w == nil {
		return ""
	}
	return fmt.Sprintf("Widget{text:%s, bounds:%s, enabled:%t, path:%s}",
		w.Text, w.Bounds.String(), w.Enabled, w.path)
}

// BuildFullXpath 鏋勫缓瀹屾暣XPath
func (w *Widget) GetPath() string {
	if w == nil {
		return ""
	}
	return w.path
}

// ClearDetails 娓呴櫎璇︾粏淇℃伅
func (w *Widget) ClearDetails() {
	w.Text = ""
	w.ContextDesc = ""
	w.Hashcode = 0
}

// FillDetails 濉厖璇︾粏淇℃伅
func (w *Widget) FillDetails(copy *Widget) {
	if copy == nil {
		return
	}
	w.Text = copy.Text
	w.ContextDesc = copy.ContextDesc
	w.Hashcode = copy.Hashcode
}

// enableOperate 鍚敤鎿嶄綔
func (w *Widget) enableOperate(opt OperateType) {
	w.OperateMask |= opt
}

// preprocessText 鏂囨湰棰勫鐞?

func (w *Widget) preprocessText() {
	// 绉婚櫎鏁板瓧鍜岀┖鏍?

	var result strings.Builder
	for _, r := range w.Text {
		if !((r >= '0' && r <= '9') || r == ' ') {
			result.WriteRune(r)
		}
	}
	w.Text = result.String()

	// 鏂囨湰闀垮害澶勭悊
	if STATE_WITH_TEXT {
		overMaxLen := len(w.Text) > STATE_TEXT_MAX_LEN

		// 鍏堟埅鏂埌 STATE_TEXT_MAX_LEN * 4
		if len(w.Text) > STATE_TEXT_MAX_LEN*4 {
			w.Text = w.Text[:STATE_TEXT_MAX_LEN*4]
		}

		cutLength := STATE_TEXT_MAX_LEN

		// 澶勭悊涓枃瀛楃鎴柇
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

		// 鍝堝笇璁＄畻
		// 鍙湁鍦?!overMaxLen 鏃舵墠鏇存柊鍝堝笇
		if !overMaxLen {
			textHash := tool.HashString(w.Text)
			w.Hashcode ^= (0x79b9 + (textHash << 5))
		}
	}

	// 绱㈠紩鍝堝笇澶勭悊
	//if STATE_WITH_INDEX {
	//	indexHash := utils.HashInt(w.Index)
	//	w.Hashcode ^= ((0x79b9 + (indexHash << 6)) << 1)
	//}
}

// initFormElement 浠嶦lement鍒濆鍖?

func (w *Widget) initFormElement(element IElement) {
	// 鍏堣缃熀鏈睘鎬?
	w.Text = element.GetText()
	w.elementSimpleIdentifier = element.GetIdentifierHash()
	w.path = element.GetPath()
	//w.Index = element.GetIndex()
	w.Enabled = element.GetEnable()
	w.Editable = element.GetEditable()
	w.Bounds = element.GetBounds()
	//w.ContextDesc = element.GetContentDesc()
	w.ScrollTypeField = element.GetScrollType()

	// 璁剧疆鎿嶄綔鎺╃爜
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

	// 澶勭悊婊氬姩鍔ㄤ綔
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
		// 涓嶆坊鍔犳粴鍔ㄥ姩浣?
	}

	//// 绮剧‘鐨勭紪杈戞鍒ゆ柇
	//w.IsEditableField = (w.Clazz == "uia.widget.EditText" ||
	//	w.Clazz == "uia.inputmethodservice.ExtractEditText" ||
	//	w.Clazz == "uia.widget.AutoCompleteTextView" ||
	//	w.Clazz == "uia.widget.MultiAutoCompleteTextView")

	// 鐗规畩澶勭悊锛氬己鍒朵负EditText鍏冪礌娣诲姞鐐瑰嚮鍜岄暱鎸夊姛鑳斤紙绫讳技C++鐗堟湰鐨凢ORCE_EDITTEXT_CLICK_TRUE锛?
	if FORCE_EDITTEXT_CLICK_TRUE && w.Editable {
		// 寮哄埗璁剧疆鎿嶄綔鎺╃爜鍜屽姩浣?
		w.enableOperate(Clickable)
		w.enableOperate(LongClickable)
		w.Actions[CLICK] = true
		w.Actions[LONG_CLICK] = true
	}

	//if w.HasAction() {
	//	// 鐗规畩鐨凷CROLL_BOTTOM_UP_N鍔ㄤ綔
	//	if SCROLL_BOTTOM_UP_N_ENABLE && (w.Clazz == "uia.widget.ListView" ||
	//		w.Clazz == "uia.support.v7.widget.RecyclerView" ||
	//		w.Clazz == "androidx.recyclerview.widget.RecyclerView") {
	//		w.Actions[SCROLL_BOTTOM_UP_N] = true
	//	}
	//}

	w.Hash()

}

// WidgetList Widget鎸囬拡鍒囩墖
type WidgetList []IWidget

// WidgetSet Widget鎸囬拡闆嗗悎
type WidgetSet map[uintptr]IWidget

// WidgetListMap Widget鎸囬拡鍚戦噺鏄犲皠
type WidgetListMap map[uintptr]WidgetList

// Add 娣诲姞鍒伴泦鍚?

func (s WidgetSet) Add(widget IWidget) {
	if widget != nil {
		s[widget.Hash()] = widget
	}
}

// Remove 浠庨泦鍚堜腑绉婚櫎
func (s WidgetSet) Remove(widget IWidget) {
	if widget != nil {
		delete(s, widget.Hash())
	}
}

// Contains 妫€鏌ユ槸鍚﹀寘鍚?

func (s WidgetSet) Contains(widget IWidget) bool {
	if widget == nil {
		return false
	}
	_, exists := s[widget.Hash()]
	return exists
}

// ToSlice 杞崲涓哄垏鐗?

func (s WidgetSet) ToSlice() WidgetList {
	result := make(WidgetList, 0, len(s))
	for _, widget := range s {
		result = append(result, widget)
	}
	return result
}

// Add 娣诲姞鍒版槧灏?

func (m WidgetListMap) Add(hash uintptr, widget IWidget) {
	if m[hash] == nil {
		m[hash] = make(WidgetList, 0)
	}
	m[hash] = append(m[hash], widget)
}

// Get 鑾峰彇鏄犲皠鍊?

func (m WidgetListMap) Get(hash uintptr) WidgetList {
	return m[hash]
}
