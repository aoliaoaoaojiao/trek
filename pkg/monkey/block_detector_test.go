package monkey

import (
	"testing"
	"trek/internal/engine/core/types"
)

func TestBlockDetectorSameActionNoChange(t *testing.T) {
	detector := newBlockDetector(3)
	cmd := &types.ActionCommand{Act: types.CLICK, Pos: *types.NewRect(0.1, 0.2, 0.3, 0.4)}

	for i := 0; i < 2; i++ {
		if detector.Observe("PageA", buildActionKey(cmd), cmd.Act, false) {
			t.Fatalf("第 %d 步不应触发阻塞", i+1)
		}
	}
	if !detector.Observe("PageA", buildActionKey(cmd), cmd.Act, false) {
		t.Fatal("连续 3 次同页面同操作无变化应触发阻塞")
	}
	if detector.LastReason() != blockReasonSameActionNoChange {
		t.Fatalf("预期 reason=%s，实际: %s", blockReasonSameActionNoChange, detector.LastReason())
	}
}

func TestBlockDetectorSamePageNoChange(t *testing.T) {
	detector := newBlockDetector(3)
	cmd1 := &types.ActionCommand{Act: types.CLICK, Pos: *types.NewRect(0.1, 0.2, 0.3, 0.4)}
	cmd2 := &types.ActionCommand{Act: types.CLICK, Pos: *types.NewRect(0.5, 0.6, 0.7, 0.8)}
	cmd3 := &types.ActionCommand{Act: types.SCROLL_BOTTOM_UP, Pos: *types.NewRect(0, 0, 1, 1)}

	actions := []struct {
		pageName string
		cmd      *types.ActionCommand
	}{
		{"PageA", cmd1},
		{"PageA", cmd2},
	}

	for i, a := range actions {
		if detector.Observe(a.pageName, buildActionKey(a.cmd), a.cmd.Act, false) {
			t.Fatalf("第 %d 步不应触发阻塞", i+1)
		}
	}
	// 第 3 步：同页面不同操作都没变化
	if !detector.Observe("PageA", buildActionKey(cmd3), cmd3.Act, false) {
		t.Fatal("连续 3 次同页面不同操作无变化应触发阻塞")
	}
	if detector.LastReason() != blockReasonSamePageNoChange {
		t.Fatalf("预期 reason=%s，实际: %s", blockReasonSamePageNoChange, detector.LastReason())
	}
}

func TestBlockDetectorResetsOnEscape(t *testing.T) {
	detector := newBlockDetector(3)
	cmd := &types.ActionCommand{Act: types.CLICK, Pos: *types.NewRect(0.1, 0.2, 0.3, 0.4)}

	for i := 0; i < 4; i++ {
		detector.Observe("PageA", buildActionKey(cmd), cmd.Act, false)
	}
	// 第 5 步页面跳转了
	if detector.Observe("PageA", buildActionKey(cmd), cmd.Act, true) {
		t.Fatal("页面发生变化不应触发阻塞")
	}
	// 计数器应重置
	if detector.Observe("PageA", buildActionKey(cmd), cmd.Act, false) {
		t.Fatal("escape 后计数器应重置，不应触发阻塞")
	}
}

func TestBlockDetectorResetsOnPageChange(t *testing.T) {
	detector := newBlockDetector(3)
	cmd := &types.ActionCommand{Act: types.CLICK, Pos: *types.NewRect(0.1, 0.2, 0.3, 0.4)}

	for i := 0; i < 4; i++ {
		detector.Observe("PageA", buildActionKey(cmd), cmd.Act, false)
	}
	// 页面变了
	detector.Observe("PageB", buildActionKey(cmd), cmd.Act, false)
	if detector.Observe("PageA", buildActionKey(cmd), cmd.Act, false) {
		t.Fatal("页面切换后计数器应重置")
	}
}

func TestBlockDetectorIgnoreSystemActions(t *testing.T) {
	detector := newBlockDetector(3)
	cmd := &types.ActionCommand{Act: types.BACK, Pos: *types.NewRect(0, 0, 0, 0)}

	for i := 0; i < 5; i++ {
		if detector.Observe("PageA", buildActionKey(cmd), cmd.Act, false) {
			t.Fatalf("系统动作不应触发阻塞，第 %d 步", i+1)
		}
	}
}

func TestBlockDetectorThreshold1(t *testing.T) {
	detector := newBlockDetector(1)
	cmd := &types.ActionCommand{Act: types.CLICK, Pos: *types.NewRect(0.1, 0.2, 0.3, 0.4)}

	if !detector.Observe("PageA", buildActionKey(cmd), cmd.Act, false) {
		t.Fatal("阈值为 1 时，首次即应触发阻塞")
	}
}
