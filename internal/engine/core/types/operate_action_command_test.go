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
