package types

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ActionCommand 是引擎输出给执行层的标准动作命令。
type ActionCommand struct {
	Act          ActionType `json:"act"`
	Pos          Rect       `json:"pos"`
	DragTo       *Point     `json:"drag_to,omitempty"`
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
	WidgetInfo   string     `json:"widget_info"`
}

// NewActionCommand 创建新的动作命令。
func NewActionCommand() *ActionCommand {
	return &ActionCommand{
		Act:          NOP,
		Pos:          *NewRect(0, 0, 0, 0),
		DragTo:       nil,
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

// NewActionCommandFromJSON 从 JSON 创建动作命令。
func NewActionCommandFromJSON(optJSONStr string) *ActionCommand {
	var cmd ActionCommand
	if err := json.Unmarshal([]byte(optJSONStr), &cmd); err != nil {
		return NewActionCommand()
	}
	return &cmd
}

// NewActionCommandCopy 复制动作命令。
func NewActionCommandCopy(opt *ActionCommand) *ActionCommand {
	if opt == nil {
		return NewActionCommand()
	}

	return &ActionCommand{
		Act:          opt.Act,
		Pos:          opt.Pos,
		DragTo:       clonePoint(opt.DragTo),
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
		WidgetInfo:   opt.WidgetInfo,
	}
}

func (cmd *ActionCommand) SetText(text string) string {
	cmd.Text = text
	return text
}

func (cmd *ActionCommand) GetText() string {
	return cmd.Text
}

func (cmd *ActionCommand) String() string {
	dragTo := "nil"
	if cmd.DragTo != nil {
		dragTo = cmd.DragTo.String()
	}
	return fmt.Sprintf("ActionCommand{act:%s, pos:%s, drag_to:%s, sid:%s, aid:%s, throttle:%.2f, waitTime:%d, editable:%t, allowFuzzing:%t, clear:%t, adbInput:%t, name:%s, text:%s}",
		cmd.Act.String(), cmd.Pos.String(), dragTo, cmd.Sid, cmd.Aid, cmd.Throttle, cmd.WaitTime, cmd.Editable, cmd.AllowFuzzing, cmd.Clear, cmd.AdbInput, cmd.Name, cmd.Text)
}

// DetailLogString 返回适合日志排障的动作详情。
func (cmd *ActionCommand) DetailLogString() string {
	if cmd == nil {
		return "nil"
	}
	widgetInfo := strings.TrimSpace(cmd.WidgetInfo)
	if widgetInfo == "" {
		widgetInfo = "n/a"
	}
	text := truncateLogText(cmd.Text, 80)
	dragTo := "n/a"
	if cmd.DragTo != nil {
		dragTo = cmd.DragTo.String()
	}
	return fmt.Sprintf("act=%s pos=%s drag_to=%s sid=%s aid=%s name=%s editable=%t text=%q wait_time=%d throttle=%.2f clear=%t adb_input=%t allow_fuzzing=%t raw_input=%t widget=%s",
		cmd.Act.String(), cmd.Pos.String(), dragTo, cmd.Sid, cmd.Aid, cmd.Name, cmd.Editable, text, cmd.WaitTime, cmd.Throttle, cmd.Clear, cmd.AdbInput, cmd.AllowFuzzing, cmd.RawInput, widgetInfo)
}

func (cmd *ActionCommand) ToJSON() string {
	jsonBytes, err := json.Marshal(cmd)
	if err != nil {
		return "{}"
	}
	return string(jsonBytes)
}

func (cmd *ActionCommand) FromJSON(jsonStr string) error {
	return json.Unmarshal([]byte(jsonStr), cmd)
}

func (cmd *ActionCommand) Equal(other *ActionCommand) bool {
	if other == nil {
		return false
	}

	return cmd.Act == other.Act &&
		cmd.Pos.Equal(&other.Pos) &&
		equalPoint(cmd.DragTo, other.DragTo) &&
		cmd.Sid == other.Sid &&
		cmd.Aid == other.Aid &&
		cmd.Throttle == other.Throttle &&
		cmd.WaitTime == other.WaitTime &&
		cmd.Editable == other.Editable &&
		cmd.AllowFuzzing == other.AllowFuzzing &&
		cmd.Clear == other.Clear &&
		cmd.AdbInput == other.AdbInput &&
		cmd.Name == other.Name &&
		cmd.Text == other.Text &&
		cmd.Extra0 == other.Extra0 &&
		cmd.JAction == other.JAction
}

func (cmd *ActionCommand) Clone() *ActionCommand {
	return NewActionCommandCopy(cmd)
}

func (cmd *ActionCommand) Reset() {
	cmd.Act = NOP
	cmd.Pos = *NewRect(0, 0, 0, 0)
	cmd.DragTo = nil
	cmd.Sid = ""
	cmd.Aid = ""
	cmd.Throttle = 0
	cmd.WaitTime = 0
	cmd.Editable = false
	cmd.AllowFuzzing = true
	cmd.Clear = false
	cmd.AdbInput = false
	cmd.Name = ""
	cmd.RawInput = false
	cmd.Text = ""
	cmd.Extra0 = ""
	cmd.JAction = ""
}

func (cmd *ActionCommand) IsValid() bool {
	if cmd.Act == NOP && cmd.Text == "" {
		return false
	}

	if cmd.Act >= CLICK && cmd.Act <= SCROLL_BOTTOM_UP_N {
		if cmd.Pos.IsEmpty() {
			return false
		}
	}

	return true
}

func (cmd *ActionCommand) GetActionName() string {
	if name, exists := actName[cmd.Act]; exists {
		return name
	}
	return "UNKNOWN"
}

func truncateLogText(text string, max int) string {
	if max <= 0 || len(text) <= max {
		return text
	}
	if max <= 3 {
		return text[:max]
	}
	return text[:max-3] + "..."
}

func clonePoint(src *Point) *Point {
	if src == nil {
		return nil
	}
	return NewPoint(src.X, src.Y)
}

func equalPoint(left *Point, right *Point) bool {
	if left == nil || right == nil {
		return left == right
	}
	return left.Equal(right)
}

func (cmd *ActionCommand) IsTextInput() bool {
	return cmd.Text != "" || cmd.Editable
}

func (cmd *ActionCommand) IsScrollAction() bool {
	return cmd.Act == SCROLL_TOP_DOWN ||
		cmd.Act == SCROLL_BOTTOM_UP ||
		cmd.Act == SCROLL_LEFT_RIGHT ||
		cmd.Act == SCROLL_RIGHT_LEFT ||
		cmd.Act == SCROLL_BOTTOM_UP_N
}

func (cmd *ActionCommand) IsClickAction() bool {
	return cmd.Act == CLICK || cmd.Act == LONG_CLICK
}

// ActionCommandNop 表示空动作命令。
var ActionCommandNop = NewActionCommand()

// OperateList 是动作命令列表。
type OperateList []*ActionCommand

func (ops OperateList) Add(op *ActionCommand) OperateList {
	return append(ops, op)
}

func (ops OperateList) Remove(index int) OperateList {
	if index < 0 || index >= len(ops) {
		return ops
	}
	return append(ops[:index], ops[index+1:]...)
}

func (ops OperateList) FilterValid() OperateList {
	result := make(OperateList, 0)
	for _, op := range ops {
		if op.IsValid() {
			result = append(result, op)
		}
	}
	return result
}

func (ops OperateList) FilterByActionType(actionType ActionType) OperateList {
	result := make(OperateList, 0)
	for _, op := range ops {
		if op.Act == actionType {
			result = append(result, op)
		}
	}
	return result
}

func (ops OperateList) ToJSON() string {
	jsonBytes, err := json.Marshal(ops)
	if err != nil {
		return "[]"
	}
	return string(jsonBytes)
}
