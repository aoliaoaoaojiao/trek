package types

import (
	"encoding/json"
	"fmt"
)

// DeviceOperateWrapper 设备操作包装器，用于将模型生成的操作转换为设备可理解的操作
type DeviceOperateWrapper struct {
	Act          ActionType `json:"act"`
	Pos          Rect       `json:"pos"`
	Sid          string     `json:"sid"`
	Aid          string     `json:"aid"`
	Throttle     float32    `json:"throttle"`
	WaitTime     int        `json:"wait_time"`
	Editable     bool       `json:"editable"`
	AllowFuzzing bool       `json:"allow_fuzzing"`
	Clear        bool       `json:"clear"`
	AdbInput     bool       `json:"adb_input"`
	Name         string     `json:"name"`
	RawInput     bool       `json:"raw_input"`
	Text         string     `json:"text"`
	Extra0       string     `json:"extra0"`
	JAction      string     `json:"j_action"`
}

// NewDeviceOperateWrapper 创建新的设备操作包装器
func NewDeviceOperateWrapper() *DeviceOperateWrapper {
	return &DeviceOperateWrapper{
		Act:          NOP,
		Pos:          *NewRect(0, 0, 0, 0),
		Sid:          "",
		Aid:          "",
		Throttle:     0,
		WaitTime:     0,
		Editable:     false,
		AllowFuzzing: true,
		Clear:        false,
		AdbInput:     false,
		Name:         "",
		RawInput:     false,
		Text:         "",
		Extra0:       "",
		JAction:      "",
	}
}

// NewDeviceOperateWrapperFromJSON 从JSON创建设备操作包装器
func NewDeviceOperateWrapperFromJSON(optJsonStr string) *DeviceOperateWrapper {
	var wrapper DeviceOperateWrapper
	if err := json.Unmarshal([]byte(optJsonStr), &wrapper); err != nil {
		return NewDeviceOperateWrapper()
	}
	return &wrapper
}

// NewDeviceOperateWrapperCopy 复制设备操作包装器
func NewDeviceOperateWrapperCopy(opt *DeviceOperateWrapper) *DeviceOperateWrapper {
	if opt == nil {
		return NewDeviceOperateWrapper()
	}

	return &DeviceOperateWrapper{
		Act:          opt.Act,
		Pos:          opt.Pos,
		Sid:          opt.Sid,
		Aid:          opt.Aid,
		Throttle:     opt.Throttle,
		WaitTime:     opt.WaitTime,
		Editable:     opt.Editable,
		AllowFuzzing: opt.AllowFuzzing,
		Clear:        opt.Clear,
		AdbInput:     opt.AdbInput,
		Name:         opt.Name,
		RawInput:     opt.RawInput,
		Text:         opt.Text,
		Extra0:       opt.Extra0,
		JAction:      opt.JAction,
	}
}

// SetText 设置文本
func (dow *DeviceOperateWrapper) SetText(text string) string {
	dow.Text = text
	return text
}

// GetText 获取文本
func (dow *DeviceOperateWrapper) GetText() string {
	return dow.Text
}

// String 返回字符串表示
func (dow *DeviceOperateWrapper) String() string {
	return fmt.Sprintf("DeviceOperateWrapper{act:%s, pos:%s, sid:%s, aid:%s, throttle:%.2f, waitTime:%d, editable:%t, allowFuzzing:%t, clear:%t, adbInput:%t, name:%s, text:%s}",
		dow.Act.String(), dow.Pos.String(), dow.Sid, dow.Aid, dow.Throttle, dow.WaitTime, dow.Editable, dow.AllowFuzzing, dow.Clear, dow.AdbInput, dow.Name, dow.Text)
}

// ToJSON 转换为JSON
func (dow *DeviceOperateWrapper) ToJSON() string {
	jsonBytes, err := json.Marshal(dow)
	if err != nil {
		return "{}"
	}
	return string(jsonBytes)
}

// FromJSON 从JSON加载
func (dow *DeviceOperateWrapper) FromJSON(jsonStr string) error {
	return json.Unmarshal([]byte(jsonStr), dow)
}

// Equal 判断是否相等
func (dow *DeviceOperateWrapper) Equal(other *DeviceOperateWrapper) bool {
	if other == nil {
		return false
	}

	return dow.Act == other.Act &&
		dow.Pos.Equal(&other.Pos) &&
		dow.Sid == other.Sid &&
		dow.Aid == other.Aid &&
		dow.Throttle == other.Throttle &&
		dow.WaitTime == other.WaitTime &&
		dow.Editable == other.Editable &&
		dow.AllowFuzzing == other.AllowFuzzing &&
		dow.Clear == other.Clear &&
		dow.AdbInput == other.AdbInput &&
		dow.Name == other.Name &&
		dow.Text == other.Text &&
		dow.Extra0 == other.Extra0 &&
		dow.JAction == other.JAction
}

// Clone 克隆
func (dow *DeviceOperateWrapper) Clone() *DeviceOperateWrapper {
	return NewDeviceOperateWrapperCopy(dow)
}

// Reset 重置
func (dow *DeviceOperateWrapper) Reset() {
	dow.Act = NOP
	dow.Pos = *NewRect(0, 0, 0, 0)
	dow.Sid = ""
	dow.Aid = ""
	dow.Throttle = 0
	dow.WaitTime = 0
	dow.Editable = false
	dow.AllowFuzzing = true
	dow.Clear = false
	dow.AdbInput = false
	dow.Name = ""
	dow.RawInput = false
	dow.Text = ""
	dow.Extra0 = ""
	dow.JAction = ""
}

// IsValid 检查是否有效
func (dow *DeviceOperateWrapper) IsValid() bool {
	// 基本有效性检查
	if dow.Act == NOP && dow.Text == "" {
		return false
	}

	// 如果需要目标但位置为空，则无效
	if dow.Act >= CLICK && dow.Act <= SCROLL_BOTTOM_UP_N {
		if dow.Pos.IsEmpty() {
			return false
		}
	}

	return true
}

// GetActionName 获取动作名称
func (dow *DeviceOperateWrapper) GetActionName() string {
	if name, exists := actName[dow.Act]; exists {
		return name
	}
	return "UNKNOWN"
}

// IsTextInput 是否为文本输入
func (dow *DeviceOperateWrapper) IsTextInput() bool {
	return dow.Text != "" || dow.Editable
}

// IsScrollAction 是否为滚动动作
func (dow *DeviceOperateWrapper) IsScrollAction() bool {
	return dow.Act == SCROLL_TOP_DOWN ||
		dow.Act == SCROLL_BOTTOM_UP ||
		dow.Act == SCROLL_LEFT_RIGHT ||
		dow.Act == SCROLL_RIGHT_LEFT ||
		dow.Act == SCROLL_BOTTOM_UP_N
}

// IsClickAction 是否为点击动作
func (dow *DeviceOperateWrapper) IsClickAction() bool {
	return dow.Act == CLICK || dow.Act == LONG_CLICK
}

// OperateNop 无操作实例
var OperateNop = NewDeviceOperateWrapper()

// OperateList 操作指针切片
type OperateList []*DeviceOperateWrapper

// Add 添加操作
func (ops OperateList) Add(op *DeviceOperateWrapper) OperateList {
	return append(ops, op)
}

// Remove 移除操作
func (ops OperateList) Remove(index int) OperateList {
	if index < 0 || index >= len(ops) {
		return ops
	}
	return append(ops[:index], ops[index+1:]...)
}

// FilterValid 过滤有效操作
func (ops OperateList) FilterValid() OperateList {
	result := make(OperateList, 0)
	for _, op := range ops {
		if op.IsValid() {
			result = append(result, op)
		}
	}
	return result
}

// FilterByActionType 按动作类型过滤
func (ops OperateList) FilterByActionType(actionType ActionType) OperateList {
	result := make(OperateList, 0)
	for _, op := range ops {
		if op.Act == actionType {
			result = append(result, op)
		}
	}
	return result
}

// ToJSON 转换为JSON数组
func (ops OperateList) ToJSON() string {
	jsonBytes, err := json.Marshal(ops)
	if err != nil {
		return "[]"
	}
	return string(jsonBytes)
}
