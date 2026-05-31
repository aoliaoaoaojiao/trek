package pagecontrol

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"trek/internal/engine/core/types"
	"trek/internal/engine/perception"
)

// Response 是页面控件检测的结构化响应。
type Response struct {
	Controls []Control `json:"controls"`
}

// Control 是单个控件输出。
type Control struct {
	ActionType  string       `json:"action_type"`
	ControlType string       `json:"control_type"`
	Text        string       `json:"text"`
	Hint        string       `json:"hint"`
	Clickable   *bool        `json:"clickable,omitempty"`
	Confidence  float64      `json:"confidence"`
	DragTarget  *types.Point `json:"drag_target,omitempty"`
	Bounds      Bounds       `json:"bounds"`
}

// Bounds 是控件区域边界。
type Bounds struct {
	Left   float64 `json:"left"`
	Top    float64 `json:"top"`
	Right  float64 `json:"right"`
	Bottom float64 `json:"bottom"`
}

// UnmarshalJSON 兼容对象格式与四元数组格式。
func (b *Bounds) UnmarshalJSON(data []byte) error {
	if b == nil {
		return fmt.Errorf("Bounds 不能为空")
	}

	type boundsObject Bounds
	var obj boundsObject
	if err := json.Unmarshal(data, &obj); err == nil {
		*b = Bounds(obj)
		return nil
	}

	var arr []float64
	if err := json.Unmarshal(data, &arr); err == nil {
		if len(arr) != 4 {
			return fmt.Errorf("bounds 数组长度错误: %d", len(arr))
		}
		b.Left = arr[0]
		b.Top = arr[1]
		b.Right = arr[2]
		b.Bottom = arr[3]
		return nil
	}

	return fmt.Errorf("bounds 格式非法: %s", string(data))
}

// ParseCandidates 将页面控件响应转换为统一候选列表。
// shotWidth/shotHeight 是发送给 LLM 的截图尺寸，用于像素坐标归一化。
func ParseCandidates(output Response, shotWidth, shotHeight int) []perception.Candidate {
	items := make([]perception.Candidate, 0, len(output.Controls))
	for _, raw := range output.Controls {
		// 坐标格式适配：统一归一化到 [0,1]
		nb := normalizeBounds(raw.Bounds.Left, raw.Bounds.Top, raw.Bounds.Right, raw.Bounds.Bottom, shotWidth, shotHeight)
		raw.Bounds.Left, raw.Bounds.Top, raw.Bounds.Right, raw.Bounds.Bottom = nb[0], nb[1], nb[2], nb[3]
		if raw.DragTarget != nil {
			ndr := normalizeBounds(raw.DragTarget.X, raw.DragTarget.Y, raw.DragTarget.X, raw.DragTarget.Y, shotWidth, shotHeight)
			raw.DragTarget = &types.Point{X: ndr[0], Y: ndr[1]}
		}
		cmd, ok := toCommand(raw)
		if !ok || cmd == nil || !cmd.IsValid() {
			continue
		}
		label := strings.TrimSpace(raw.Text)
		if label == "" {
			label = strings.TrimSpace(raw.Hint)
		}
		intent := "llm_control_detected"
		if label != "" {
			intent = "llm_control:" + label
		}
		metadata := map[string]string{
			"llm_action_type":  normalizeActionType(raw),
			"llm_control_type": strings.TrimSpace(raw.ControlType),
			"llm_control_text": strings.TrimSpace(raw.Text),
			"llm_target_hint":  strings.TrimSpace(raw.Hint),
		}
		if raw.Clickable != nil {
			if *raw.Clickable {
				metadata["llm_clickable"] = "true"
			} else {
				metadata["llm_clickable"] = "false"
			}
		}
		item := perception.NewCandidate(cmd, perception.SourceLLM, intent, metadata)
		item.Confidence = raw.Confidence
		items = append(items, item)
	}
	return items
}

// normalizeBounds 将不同格式的坐标统一归一化到 [0,1]。
func normalizeBounds(left, top, right, bottom float64, shotWidth, shotHeight int) [4]float64 {
	// 已经是 [0,1] → 直接返回
	if right <= 1 && bottom <= 1 {
		return [4]float64{left, top, right, bottom}
	}
	// 0-1000 格式 → 除以 1000
	if left <= 1000 && top <= 1000 && right <= 1000 && bottom <= 1000 {
		return [4]float64{left / 1000, top / 1000, right / 1000, bottom / 1000}
	}
	// 像素坐标 → 除以截图尺寸
	if shotWidth > 0 && shotHeight > 0 {
		return [4]float64{
			left / float64(shotWidth),
			top / float64(shotHeight),
			right / float64(shotWidth),
			bottom / float64(shotHeight),
		}
	}
	return [4]float64{left, top, right, bottom}
}

func toCommand(raw Control) (*types.ActionCommand, bool) {
	if raw.Bounds.Right <= raw.Bounds.Left || raw.Bounds.Bottom <= raw.Bounds.Top {
		return nil, false
	}
	act, ok := toActionType(raw)
	if !ok {
		return nil, false
	}
	cmd := types.NewActionCommand()
	cmd.Act = act
	cmd.Pos = *types.NewRect(raw.Bounds.Left, raw.Bounds.Top, raw.Bounds.Right, raw.Bounds.Bottom)
	if normalizeActionType(raw) == "drag" && raw.DragTarget != nil {
		cmd.DragTo = types.NewPoint(raw.DragTarget.X, raw.DragTarget.Y)
	}
	return cmd, true
}

func toActionType(raw Control) (types.ActionType, bool) {
	switch normalizeActionType(raw) {
	case "click":
		return types.CLICK, true
	case "input":
		return types.INPUT, true
	case "swipe_up":
		return types.SCROLL_BOTTOM_UP, true
	case "swipe_down":
		return types.SCROLL_TOP_DOWN, true
	case "swipe_left":
		return types.SCROLL_RIGHT_LEFT, true
	case "swipe_right":
		return types.SCROLL_LEFT_RIGHT, true
	case "drag":
		return inferDragAction(raw)
	default:
		return types.NOP, false
	}
}

func normalizeActionType(raw Control) string {
	actionType := strings.ToLower(strings.TrimSpace(raw.ActionType))
	if actionType != "" {
		return actionType
	}
	switch strings.ToLower(strings.TrimSpace(raw.ControlType)) {
	case "input":
		return "input"
	default:
		return "click"
	}
}

func inferDragAction(raw Control) (types.ActionType, bool) {
	if raw.DragTarget == nil {
		return types.NOP, false
	}
	centerX := (raw.Bounds.Left + raw.Bounds.Right) / 2
	centerY := (raw.Bounds.Top + raw.Bounds.Bottom) / 2
	deltaX := raw.DragTarget.X - centerX
	deltaY := raw.DragTarget.Y - centerY
	if deltaX == 0 && deltaY == 0 {
		return types.NOP, false
	}
	if math.Abs(deltaX) >= math.Abs(deltaY) {
		if deltaX >= 0 {
			return types.SCROLL_LEFT_RIGHT, true
		}
		return types.SCROLL_RIGHT_LEFT, true
	}
	if deltaY >= 0 {
		return types.SCROLL_TOP_DOWN, true
	}
	return types.SCROLL_BOTTOM_UP, true
}
