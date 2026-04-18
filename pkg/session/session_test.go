package session

import (
	"os"
	"path/filepath"
	"testing"
	"trek/internal/engine/core/types"
)

func TestSessionNextAction(t *testing.T) {
	session := NewSession(Config{
		PackageName: "com.demo",
		Algorithm:   types.Reuse,
		DeviceType:  types.Phone,
	})

	action, err := session.NextAction("LoginActivity", `
<hierarchy>
	<node class="android.widget.FrameLayout" resource-id="" content-desc="" text="" clickable="false" long-clickable="false" checkable="false" enabled="true" bounds="[0,0][1080,1920]">
		<node class="android.widget.Button" resource-id="com.demo:id/login" content-desc="鐧诲綍鎸夐挳" text="鐧诲綍" clickable="true" long-clickable="false" checkable="false" enabled="true" bounds="[10,20][110,120]"/>
	</node>
</hierarchy>`)
	if err != nil {
		t.Fatalf("鐢熸垚鍔ㄤ綔澶辫触: %v", err)
	}

	if action == nil {
		t.Fatalf("鍔ㄤ綔缁撴灉涓虹┖")
	}
}

func TestSessionCheckPointInBlackRects(t *testing.T) {
	session := NewSession(Config{
		PackageName: "com.demo",
	})

	configPath := filepath.Join(t.TempDir(), "mix.json")
	configContent := `{"black_rects":{"LoginActivity":[[0,0,100,100]]}}`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("鍐欏叆娴嬭瘯閰嶇疆澶辫触: %v", err)
	}
	if err := session.LoadConfigFile(configPath); err != nil {
		t.Fatalf("鍔犺浇閰嶇疆澶辫触: %v", err)
	}

	if !session.CheckPointInBlackRects("LoginActivity", types.Point{X: 50, Y: 50}) {
		t.Fatalf("榛戝悕鍗曞垽鏂粨鏋滀笉姝ｇ‘")
	}
}
