package types

import (
	"Trek/internal/fastbot/tool"
	"Trek/log"
	"fmt"
	"sort"
	"strings"
)

// Widget Widget结构，使用Widget内部文本来嵌入和识别Widget
type Widget struct {
	Hashcode        uintptr
	Parent          *Widget
	Text            string
	Index           int
	Clazz           string
	ResourceID      string
	Enabled         bool
	Editable        bool
	OperateMask     OperateType
	Bounds          *Rect
	ContextDesc     string
	Actions         map[ActionType]bool
	IsEditableField bool       // 更精确的编辑框判断（避免与方法名冲突）
	ScrollTypeField ScrollType // 滚动类型
}

// 状态处理常量
const (
	// STATE_WITH_TEXT 如果应该基于文本生成哈希
	STATE_WITH_TEXT = false

	// STATE_TEXT_MAX_LEN 参与抽象的文本的最长长度，通常是3的倍数
	// 超过2个中文字符将不参与抽象，字符长度将被截断
	STATE_TEXT_MAX_LEN = 6 // 2*3

	// STATE_WITH_INDEX 如果应该基于索引生成哈希
	STATE_WITH_INDEX = false

	// SCROLL_BOTTOM_UP_N_ENABLE 是否启用SCROLL_BOTTOM_UP_N动作
	SCROLL_BOTTOM_UP_N_ENABLE = false

	// FORCE_EDITTEXT_CLICK_TRUE 是否强制为EditText元素添加点击和长按功能
	FORCE_EDITTEXT_CLICK_TRUE = true
)

// NewWidget 创建新的Widget
func NewWidget(parent *Widget, element *Element) *Widget {
	widget := &Widget{
		Parent:          parent,
		Actions:         make(map[ActionType]bool),
		OperateMask:     None,
		Bounds:          NewRect(0, 0, 0, 0),
		ScrollTypeField: NONE, // 初始化滚动类型
	}

	if element != nil {
		widget.initFormElement(element)
		// 文本预处理
		widget.preprocessText()
	}

	return widget
}

// GetParent 获取父Widget
func (w *Widget) GetParent() *Widget {
	return w.Parent
}

// GetBounds 获取边界
func (w *Widget) GetBounds() *Rect {
	return w.Bounds
}

// GetActions 获取动作集合
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

// GetText 获取文本
func (w *Widget) GetText() string {
	return w.Text
}

// GetEnabled 获取是否启用
func (w *Widget) GetEnabled() bool {
	return w.Enabled
}

// HasOperate 检查是否有指定操作
func (w *Widget) HasOperate(opt OperateType) bool {
	return w.OperateMask&opt != 0
}

// HasAction 检查是否有动作
func (w *Widget) HasAction() bool {
	return len(w.Actions) > 0
}

// IsEditable 检查是否可编辑
func (w *Widget) IsEditable() bool {
	return w.IsEditableField
}

// Hash 计算哈希值
func (w *Widget) Hash() uintptr {
	if w.Hashcode == 0 {
		// 基础哈希计算
		hashcode1 := tool.HashString(w.Clazz)
		hashcode2 := tool.HashString(w.ResourceID)
		hashcode3 := tool.HashInt(int(w.OperateMask))
		hashcode4 := tool.HashInt(int(w.ScrollTypeField))

		w.Hashcode = ((hashcode1 ^ (hashcode2 << 4)) >> 2) ^
			(((127 * hashcode3 << 1) ^ (256 * hashcode4 << 3)) >> 1)
	}
	return w.Hashcode
}

// String 返回字符串表示
func (w *Widget) String() string {
	return fmt.Sprintf("Widget{text:%s, class:%s, resource-id:%s, bounds:%s, enabled:%t}",
		w.Text, w.Clazz, w.ResourceID, w.Bounds.String(), w.Enabled)
}

// BuildFullXpath 构建完整XPath
func (w *Widget) BuildFullXpath() string {
	xpath := w.toXPath()
	if w.Parent != nil {
		return w.Parent.BuildFullXpath() + "/" + xpath
	}
	return "/" + xpath
}

// ClearDetails 清除详细信息
func (w *Widget) ClearDetails() {
	w.Text = ""
	w.ContextDesc = ""
	w.Hashcode = 0
}

// FillDetails 填充详细信息
func (w *Widget) FillDetails(copy *Widget) {
	if copy == nil {
		return
	}
	w.Text = copy.Text
	w.ContextDesc = copy.ContextDesc
	w.Hashcode = copy.Hashcode
}

// enableOperate 启用操作
func (w *Widget) enableOperate(opt OperateType) {
	w.OperateMask |= opt
}

// preprocessText 文本预处理
func (w *Widget) preprocessText() {
	// 移除数字和空格
	var result strings.Builder
	for _, r := range w.Text {
		if !((r >= '0' && r <= '9') || r == ' ') {
			result.WriteRune(r)
		}
	}
	w.Text = result.String()

	// 文本长度处理
	if STATE_WITH_TEXT {
		overMaxLen := len(w.Text) > STATE_TEXT_MAX_LEN

		// 先截断到 STATE_TEXT_MAX_LEN * 4
		if len(w.Text) > STATE_TEXT_MAX_LEN*4 {
			w.Text = w.Text[:STATE_TEXT_MAX_LEN*4]
		}

		cutLength := STATE_TEXT_MAX_LEN

		// 处理中文字符截断
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

		// 哈希计算
		// 只有在 !overMaxLen 时才更新哈希
		if !overMaxLen {
			textHash := tool.HashString(w.Text)
			w.Hashcode ^= (0x79b9 + (textHash << 5))
		}
	}

	// 索引哈希处理
	if STATE_WITH_INDEX {
		indexHash := tool.HashInt(w.Index)
		w.Hashcode ^= ((0x79b9 + (indexHash << 6)) << 1)
	}
}

// initFormElement 从Element初始化
func (w *Widget) initFormElement(element *Element) {
	// 先设置基本属性
	w.Text = element.GetText()
	w.Clazz = element.GetClassname()
	w.ResourceID = element.GetResourceID()
	w.Index = element.GetIndex()
	w.Enabled = element.GetEnable()
	w.Editable = element.IsEditText()
	w.Bounds = element.GetBounds()
	w.ContextDesc = element.GetContentDesc()
	w.ScrollTypeField = element.GetScrollType()

	// 设置操作掩码
	if element.GetCheckable() {
		w.enableOperate(Checkable)
	}
	if element.GetEnable() {
		w.enableOperate(Enable)
	}
	if element.GetClickable() {
		w.enableOperate(Clickable)
	}
	if element.GetScrollable() {
		w.enableOperate(Scrollable)
	}
	if element.GetLongClickable() {
		w.enableOperate(LongClickable)
		w.Actions[LONG_CLICK] = true
	}
	if w.HasOperate(Checkable) || w.HasOperate(Clickable) {
		w.Actions[CLICK] = true
	}

	// 处理滚动动作
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
		// 不添加滚动动作
	}

	// 精确的编辑框判断
	w.IsEditableField = (w.Clazz == "android.widget.EditText" ||
		w.Clazz == "android.inputmethodservice.ExtractEditText" ||
		w.Clazz == "android.widget.AutoCompleteTextView" ||
		w.Clazz == "android.widget.MultiAutoCompleteTextView")

	// 特殊处理：强制为EditText元素添加点击和长按功能（类似C++版本的FORCE_EDITTEXT_CLICK_TRUE）
	if FORCE_EDITTEXT_CLICK_TRUE && w.IsEditableField {
		// 强制设置操作掩码和动作
		w.enableOperate(Clickable)
		w.enableOperate(LongClickable)
		w.Actions[CLICK] = true
		w.Actions[LONG_CLICK] = true
	}

	if w.HasAction() {
		// 特殊的SCROLL_BOTTOM_UP_N动作
		if SCROLL_BOTTOM_UP_N_ENABLE && (w.Clazz == "android.widget.ListView" ||
			w.Clazz == "android.support.v7.widget.RecyclerView" ||
			w.Clazz == "androidx.recyclerview.widget.RecyclerView") {
			w.Actions[SCROLL_BOTTOM_UP_N] = true
		}
	}

	w.Hash()

}

// toXPath 转换为XPath
func (w *Widget) toXPath() string {
	var xpath strings.Builder

	if w.Clazz == "" && w.Text == "" && w.ResourceID == "" {
		log.Debugf("widget detail has been clear")
		xpath.WriteString("*")
	} else if w.Clazz != "" {
		xpath.WriteString(fmt.Sprintf("*[@class='%s'", w.Clazz))
	} else {
		xpath.WriteString("*")
	}

	if w.Text != "" {
		xpath.WriteString(fmt.Sprintf(" and @text='%s'", w.Text))
	}

	if w.ResourceID != "" {
		xpath.WriteString(fmt.Sprintf(" and @resource-id='%s'", w.ResourceID))
	}

	if w.Index >= 0 {
		xpath.WriteString(fmt.Sprintf(" and @index='%d'", w.Index))
	}

	xpath.WriteString("]")

	return xpath.String()
}

// WidgetList Widget指针切片
type WidgetList []*Widget

// WidgetSet Widget指针集合
type WidgetSet map[uintptr]*Widget

// WidgetListMap Widget指针向量映射
type WidgetListMap map[uintptr]WidgetList

// Add 添加到集合
func (s WidgetSet) Add(widget *Widget) {
	if widget != nil {
		s[widget.Hash()] = widget
	}
}

// Remove 从集合中移除
func (s WidgetSet) Remove(widget *Widget) {
	if widget != nil {
		delete(s, widget.Hash())
	}
}

// Contains 检查是否包含
func (s WidgetSet) Contains(widget *Widget) bool {
	if widget == nil {
		return false
	}
	_, exists := s[widget.Hash()]
	return exists
}

// ToSlice 转换为切片
func (s WidgetSet) ToSlice() WidgetList {
	result := make(WidgetList, 0, len(s))
	for _, widget := range s {
		result = append(result, widget)
	}
	return result
}

// Add 添加到映射
func (m WidgetListMap) Add(hash uintptr, widget *Widget) {
	if m[hash] == nil {
		m[hash] = make(WidgetList, 0)
	}
	m[hash] = append(m[hash], widget)
}

// Get 获取映射值
func (m WidgetListMap) Get(hash uintptr) WidgetList {
	return m[hash]
}
