package llm

import (
	"encoding/json"
	"fmt"
	"strings"
	"trek/internal/engine/candidate"
	"trek/internal/engine/decision/shared/types"
)

type pageControlResponse struct {
	Controls []llmControl `json:"controls"`
}

type llmControl struct {
	ControlType string    `json:"control_type"`
	Text        string    `json:"text"`
	Hint        string    `json:"hint"`
	Clickable   *bool     `json:"clickable,omitempty"`
	Confidence  float64   `json:"confidence"`
	Bounds      llmBounds `json:"bounds"`
}

type llmBounds struct {
	Left   float64 `json:"left"`
	Top    float64 `json:"top"`
	Right  float64 `json:"right"`
	Bottom float64 `json:"bottom"`
}

// UnmarshalJSON 兼容两类 bounds 输出：
// 1. 对象格式：{"left":...,"top":...,"right":...,"bottom":...}
// 2. 数组格式：[left, top, right, bottom]
func (b *llmBounds) UnmarshalJSON(data []byte) error {
	if b == nil {
		return fmt.Errorf("llmBounds 不能为空")
	}

	type boundsObject llmBounds
	var obj boundsObject
	if err := json.Unmarshal(data, &obj); err == nil {
		*b = llmBounds(obj)
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

func parsePageControlCandidates(output pageControlResponse) []candidate.Candidate {
	items := make([]candidate.Candidate, 0, len(output.Controls))
	for _, raw := range output.Controls {
		cmd, ok := toPageControlCommand(raw)
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
		item := candidate.NewCandidate(cmd, candidate.SourceLLM, intent, metadata)
		item.Confidence = raw.Confidence
		items = append(items, item)
	}
	return items
}

func toPageControlCommand(raw llmControl) (*types.ActionCommand, bool) {
	if raw.Bounds.Right <= raw.Bounds.Left || raw.Bounds.Bottom <= raw.Bounds.Top {
		return nil, false
	}
	cmd := types.NewActionCommand()
	cmd.Act = types.CLICK
	cmd.Pos = *types.NewRect(raw.Bounds.Left, raw.Bounds.Top, raw.Bounds.Right, raw.Bounds.Bottom)
	return cmd, true
}
