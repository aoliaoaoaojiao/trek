package session

import (
	"os"
	"path/filepath"
	"testing"
	"trek/internal/engine/decision"
	types2 "trek/internal/engine/decision/shared/types"
)

func TestSessionNextAction(t *testing.T) {
	session := NewSession(Config{
		PackageName: "com.demo",
		Algorithm:   decision.AlgorithmReuse,
		DeviceType:  types2.Phone,
	})

	action, err := session.NextAction("LoginActivity", `
<hierarchy>
	<node class="android.widget.FrameLayout" resource-id="" content-desc="" text="" clickable="false" long-clickable="false" checkable="false" enabled="true" bounds="[0,0][1080,1920]">
		<node class="android.widget.Button" resource-id="com.demo:id/login" content-desc="鐧诲綍" text="鐧诲綍" clickable="true" long-clickable="false" checkable="false" enabled="true" bounds="[10,20][110,120]"/>
	</node>
</hierarchy>`)
	if err != nil {
		t.Fatalf("鑾峰彇涓嬩竴姝ュ姩浣滃け锟? %v", err)
	}

	if action == nil {
		t.Fatalf("棰勬湡杩斿洖闈炵┖鍔ㄤ綔")
	}
}

func TestSessionCheckPointInBlackRects(t *testing.T) {
	session := NewSession(Config{PackageName: "com.demo"})

	configPath := filepath.Join(t.TempDir(), "mix.js")
	configContent := `const config = {
  black_rects: {
    LoginActivity: [[0, 0, 100, 100]]
  }
};`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("鍐欏叆娴嬭瘯鏂囦欢澶辫触: %v", err)
	}
	if err := session.LoadConfigFile(configPath); err != nil {
		t.Fatalf("鍔犺浇閰嶇疆澶辫触: %v", err)
	}

	if !session.CheckPointInBlackRects("LoginActivity", types2.Point{X: 50, Y: 50}) {
		t.Fatalf("鐐逛綅搴斿懡涓粦鍚嶅崟鍖哄煙")
	}
}

func TestSessionTransformPageInfoWithInput(t *testing.T) {
	session := NewSession(Config{PackageName: "com.demo"})

	configPath := filepath.Join(t.TempDir(), "mix.js")
	configContent := `const plugin = {
  transformPage(ctx) {
    return {
      page_name: ctx.page.name + "_v2",
      xml: ctx.page.xml.replace("foo", "bar"),
    }
  }
};`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("鍐欏叆娴嬭瘯鏂囦欢澶辫触: %v", err)
	}
	if err := session.LoadConfigFile(configPath); err != nil {
		t.Fatalf("鍔犺浇閰嶇疆澶辫触: %v", err)
	}

	info, err := session.TransformPageInfoWithInput("MainActivity", ActionInput{XMLDescOfGuiTree: `<node text="foo"/>`})
	if err != nil {
		t.Fatalf("TransformPageInfoWithInput 澶辫触: %v", err)
	}
	if info.PageName != "MainActivity_v2" {
		t.Fatalf("椤甸潰鍚嶆敼閫犵粨鏋滀笉绗﹀悎棰勬湡: %s", info.PageName)
	}
	if info.XML != `<node text="bar"/>` {
		t.Fatalf("xml 鏀归€犵粨鏋滀笉绗﹀悎棰勬湡: %s", info.XML)
	}
}

func TestSessionBeforeDecideUsesGojaPluginAction(t *testing.T) {
	session := NewSession(Config{PackageName: "com.demo"})

	configPath := filepath.Join(t.TempDir(), "plugin.js")
	configContent := `const plugin = {
  beforeDecide(ctx) {
    return trek.action.back()
  }
};`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("鍐欏叆娴嬭瘯鏂囦欢澶辫触: %v", err)
	}
	if err := session.LoadConfigFile(configPath); err != nil {
		t.Fatalf("鍔犺浇閰嶇疆澶辫触: %v", err)
	}

	action, err := session.NextAction("MainActivity", `<hierarchy><node class="android.widget.TextView" bounds="[0,0][10,10]"/></hierarchy>`)
	if err != nil {
		t.Fatalf("鑾峰彇鍔ㄤ綔澶辫触: %v", err)
	}
	if action.Act != types2.BACK {
		t.Fatalf("鎻掍欢搴旇鐩栦负 BACK锛屽疄锟? %s", action.Act.String())
	}
}

func TestSessionOnStepResultFeedsGojaPluginState(t *testing.T) {
	session := NewSession(Config{PackageName: "com.demo"})

	configPath := filepath.Join(t.TempDir(), "plugin.js")
	configContent := `const plugin = {
  onStepResult(ctx) {
    if (ctx.result.crash && ctx.result.after.xml.indexOf("After") >= 0) {
      trek.state.set("should_back", true)
    }
  },
  beforeDecide(ctx) {
    if (trek.state.get("should_back")) return trek.action.back()
    return null
  }
};`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("鍐欏叆娴嬭瘯鏂囦欢澶辫触: %v", err)
	}
	if err := session.LoadConfigFile(configPath); err != nil {
		t.Fatalf("鍔犺浇閰嶇疆澶辫触: %v", err)
	}

	after := PageSnapshot{PageName: "After", XML: `<hierarchy text="After"/>`, Screenshot: []byte{1, 2, 3}}
	if err := session.OnStepResult(StepResultInput{
		Step:    1,
		Action:  &types2.ActionCommand{Act: types2.CLICK, Pos: *types2.NewRect(0, 0, 10, 10)},
		Success: false,
		Crash:   true,
		Before:  PageSnapshot{PageName: "Before", XML: `<hierarchy/>`},
		After:   &after,
	}); err != nil {
		t.Fatalf("OnStepResult 澶辫触: %v", err)
	}

	action, err := session.NextAction("MainActivity", `<hierarchy><node class="android.widget.TextView" bounds="[0,0][10,10]"/></hierarchy>`)
	if err != nil {
		t.Fatalf("鑾峰彇鍔ㄤ綔澶辫触: %v", err)
	}
	if action.Act != types2.BACK {
		t.Fatalf("onStepResult 鐘舵€佸簲椹卞姩涓嬩竴锟?BACK锛屽疄锟? %s", action.Act.String())
	}
}
