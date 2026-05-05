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
		t.Fatalf("beforeDecide 澶辫触: %v", err)
	}
	if !handled || cmd == nil || cmd.Act != types.CLICK {
		t.Fatalf("鍔ㄤ綔杞崲涓嶇鍚堥锟? handled=%v cmd=%+v", handled, cmd)
	}
	if cmd.Pos.Left != 10 || cmd.Pos.Top != 20 || cmd.Pos.Right != 110 || cmd.Pos.Bottom != 120 {
		t.Fatalf("bounds 杞崲涓嶇鍚堥锟? %+v", cmd.Pos)
	}

	cmd, handled, err = adapter.AfterDecide(PluginContext{}, cmd)
	if err != nil {
		t.Fatalf("afterDecide 澶辫触: %v", err)
	}
	if !handled || cmd == nil || cmd.Act != types.BACK {
		t.Fatalf("afterDecide 杞崲涓嶇鍚堥锟? handled=%v cmd=%+v", handled, cmd)
	}
}

func mustLoadScript(t *testing.T, source string) *scripting.Manager {
	t.Helper()
	manager, err := scripting.LoadScript(source)
	if err != nil {
		t.Fatalf("鍔犺浇鑴氭湰澶辫触: %v", err)
	}
	return manager
}
