package plugin

import (
	"testing"

	"trek/internal/engine/core/types"
	"trek/internal/scripting"
)

func TestAdapterConvertsScriptActionToActionCommand(t *testing.T) {
	adapter := NewAdapterFromManager(mustLoadScript(t, `const plugin = {
  beforeDecide() {
    return trek.action.click([10, 20, 110, 120])
  },
  afterDecide(ctx, action) {
    return trek.action.back()
  },
}`))

	cmd, handled, err := adapter.BeforeDecide(PluginContext{
		Page: PageSnapshot{Name: "Login", XML: `<node/>`},
	})
	if err != nil {
		t.Fatalf("beforeDecide 失败: %v", err)
	}
	if !handled || cmd == nil || cmd.Act != types.CLICK {
		t.Fatalf("动作转换不符合预期: handled=%v cmd=%+v", handled, cmd)
	}
	if cmd.Pos.Left != 10 || cmd.Pos.Top != 20 || cmd.Pos.Right != 110 || cmd.Pos.Bottom != 120 {
		t.Fatalf("bounds 转换不符合预期: %+v", cmd.Pos)
	}

	cmd, handled, err = adapter.AfterDecide(PluginContext{}, cmd)
	if err != nil {
		t.Fatalf("afterDecide 失败: %v", err)
	}
	if !handled || cmd == nil || cmd.Act != types.BACK {
		t.Fatalf("afterDecide 转换不符合预期: handled=%v cmd=%+v", handled, cmd)
	}
}

func mustLoadScript(t *testing.T, source string) *scripting.Manager {
	t.Helper()
	manager, err := scripting.LoadScript(source)
	if err != nil {
		t.Fatalf("加载脚本失败: %v", err)
	}
	return manager
}
