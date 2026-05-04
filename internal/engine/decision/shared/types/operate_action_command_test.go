package types

import (
	"strings"
	"testing"
)

func TestActionCommandCompatibility(t *testing.T) {
	cmd := NewActionCommand()
	if cmd == nil {
		t.Fatalf("ActionCommand 不能为空")
	}

	cmd.Act = CLICK
	cmd.Pos = *NewRect(1, 2, 3, 4)
	cmd.DragTo = NewPoint(5, 6)
	cmd.Text = "hello"

	jsonText := cmd.ToJSON()
	restored := NewActionCommandFromJSON(jsonText)
	if restored == nil {
		t.Fatalf("反序列化后不能为空")
	}
	if !cmd.Equal(restored) {
		t.Fatalf("ActionCommand 序列化/反序列化结果不一致")
	}

	if ActionCommandNop == nil {
		t.Fatalf("ActionCommandNop 不能为空")
	}

	if got := cmd.String(); !strings.HasPrefix(got, "ActionCommand{") {
		t.Fatalf("String 语义名不正确，got=%s", got)
	}
}

func TestActionCommandDetailLogString(t *testing.T) {
	cmd := NewActionCommand()
	cmd.Act = CLICK
	cmd.Pos = *NewRect(1, 2, 3, 4)
	cmd.Sid = "state-1"
	cmd.Aid = "action-2"
	cmd.Name = "login_btn"
	cmd.Editable = true
	cmd.Text = strings.Repeat("a", 100)
	cmd.WaitTime = 500
	cmd.Throttle = 120
	cmd.Clear = true
	cmd.AdbInput = true
	cmd.AllowFuzzing = false
	cmd.RawInput = true
	cmd.DragTo = NewPoint(5, 6)
	cmd.WidgetInfo = "Widget{text:登录, bounds:[1,2,3,4], enabled:true}"

	detail := cmd.DetailLogString()
	expectedParts := []string{
		"act=CLICK",
		"pos=[1.000,2.000,3.000,4.000]",
		"drag_to=(5,6)",
		"sid=state-1",
		"aid=action-2",
		"name=login_btn",
		"wait_time=500",
		"throttle=120.00",
		"widget=Widget{text:登录, bounds:[1,2,3,4], enabled:true}",
	}
	for _, part := range expectedParts {
		if !strings.Contains(detail, part) {
			t.Fatalf("详情日志缺少字段: %s, detail=%s", part, detail)
		}
	}
	if !strings.Contains(detail, "...") {
		t.Fatalf("长文本应被截断: %s", detail)
	}
}
