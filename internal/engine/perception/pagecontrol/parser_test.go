package pagecontrol

import (
	"testing"

	"trek/internal/engine/core/types"
)

func TestParseCandidatesMapsBasicActionTypes(t *testing.T) {
	output := Response{
		Controls: []Control{
			{ActionType: "click", Text: "确认", Confidence: 0.9, Bounds: Bounds{Left: 0.1, Top: 0.2, Right: 0.3, Bottom: 0.4}},
			{ActionType: "input", Text: "搜索框", Confidence: 0.8, Bounds: Bounds{Left: 0.2, Top: 0.3, Right: 0.6, Bottom: 0.4}},
			{ActionType: "swipe_up", Hint: "列表区域", Confidence: 0.7, Bounds: Bounds{Left: 0.0, Top: 0.2, Right: 1.0, Bottom: 0.9}},
			{ActionType: "swipe_left", Hint: "轮播区域", Confidence: 0.7, Bounds: Bounds{Left: 0.1, Top: 0.1, Right: 0.9, Bottom: 0.5}},
		},
	}

	items := ParseCandidates(output)
	if len(items) != 4 {
		t.Fatalf("候选数量错误: %d", len(items))
	}
	if items[0].Command == nil || items[0].Command.Act != types.CLICK {
		t.Fatalf("click 应映射为 CLICK，实际: %+v", items[0].Command)
	}
	if items[1].Command == nil || items[1].Command.Act != types.INPUT {
		t.Fatalf("input 应映射为 INPUT，实际: %+v", items[1].Command)
	}
	if items[2].Command == nil || items[2].Command.Act != types.SCROLL_BOTTOM_UP {
		t.Fatalf("swipe_up 应映射为 SCROLL_BOTTOM_UP，实际: %+v", items[2].Command)
	}
	if items[3].Command == nil || items[3].Command.Act != types.SCROLL_RIGHT_LEFT {
		t.Fatalf("swipe_left 应映射为 SCROLL_RIGHT_LEFT，实际: %+v", items[3].Command)
	}
}

func TestParseCandidatesInfersLegacyActionType(t *testing.T) {
	output := Response{
		Controls: []Control{
			{ControlType: "input", Text: "手机号", Confidence: 0.8, Bounds: Bounds{Left: 0.2, Top: 0.3, Right: 0.6, Bottom: 0.4}},
			{ControlType: "button", Text: "登录", Confidence: 0.9, Bounds: Bounds{Left: 0.1, Top: 0.2, Right: 0.3, Bottom: 0.4}},
		},
	}

	items := ParseCandidates(output)
	if len(items) != 2 {
		t.Fatalf("候选数量错误: %d", len(items))
	}
	if items[0].Metadata["llm_action_type"] != "input" || items[0].Command.Act != types.INPUT {
		t.Fatalf("legacy input 推断错误: metadata=%+v command=%+v", items[0].Metadata, items[0].Command)
	}
	if items[1].Metadata["llm_action_type"] != "click" || items[1].Command.Act != types.CLICK {
		t.Fatalf("legacy button 推断错误: metadata=%+v command=%+v", items[1].Metadata, items[1].Command)
	}
}

func TestParseCandidatesMapsDragToScrollableAction(t *testing.T) {
	output := Response{
		Controls: []Control{
			{
				ActionType: "drag",
				Hint:       "向右拖动滑块",
				Confidence: 0.8,
				Bounds:     Bounds{Left: 0.2, Top: 0.2, Right: 0.4, Bottom: 0.4},
				DragTarget: types.NewPoint(0.9, 0.3),
			},
		},
	}

	items := ParseCandidates(output)
	if len(items) != 1 {
		t.Fatalf("候选数量错误: %d", len(items))
	}
	if items[0].Command == nil || items[0].Command.Act != types.SCROLL_LEFT_RIGHT {
		t.Fatalf("drag 应先按方向提示映射到现有滚动动作，实际: %+v", items[0].Command)
	}
	if items[0].Command.DragTo == nil || items[0].Command.DragTo.X != 0.9 || items[0].Command.DragTo.Y != 0.3 {
		t.Fatalf("drag_target 应保留到动作命令，实际: %+v", items[0].Command)
	}
}

func TestParseCandidatesRejectsDragWithoutTarget(t *testing.T) {
	output := Response{
		Controls: []Control{
			{ActionType: "drag", Hint: "拖动滑块", Confidence: 0.8, Bounds: Bounds{Left: 0.2, Top: 0.2, Right: 0.4, Bottom: 0.4}},
		},
	}

	items := ParseCandidates(output)
	if len(items) != 0 {
		t.Fatalf("缺少 drag_target 的 drag 不应进入候选，实际: %+v", items)
	}
}
