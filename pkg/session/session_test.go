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
		<node class="android.widget.Button" resource-id="com.demo:id/login" content-desc="登录" text="登录" clickable="true" long-clickable="false" checkable="false" enabled="true" bounds="[10,20][110,120]"/>
	</node>
</hierarchy>`)
	if err != nil {
		t.Fatalf("获取下一步动作失败: %v", err)
	}

	if action == nil {
		t.Fatalf("预期返回非空动作")
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
		t.Fatalf("写入测试文件失败: %v", err)
	}
	if err := session.LoadConfigFile(configPath); err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}

	if !session.CheckPointInBlackRects("LoginActivity", types.Point{X: 50, Y: 50}) {
		t.Fatalf("点位应命中黑名单区域")
	}
}

func TestSessionTransformPageInfo(t *testing.T) {
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
		t.Fatalf("写入测试文件失败: %v", err)
	}
	if err := session.LoadConfigFile(configPath); err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}

	info, err := session.TransformPageInfo("MainActivity", `<node text="foo"/>`)
	if err != nil {
		t.Fatalf("TransformPageInfo 失败: %v", err)
	}
	if info.PageName != "MainActivity_v2" {
		t.Fatalf("页面名改造结果不符合预期: %s", info.PageName)
	}
	if info.XML != `<node text="bar"/>` {
		t.Fatalf("xml 改造结果不符合预期: %s", info.XML)
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
		t.Fatalf("写入测试文件失败: %v", err)
	}
	if err := session.LoadConfigFile(configPath); err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}

	action, err := session.NextAction("MainActivity", `<hierarchy><node class="android.widget.TextView" bounds="[0,0][10,10]"/></hierarchy>`)
	if err != nil {
		t.Fatalf("获取动作失败: %v", err)
	}
	if action.Act != types.BACK {
		t.Fatalf("插件应覆盖为 BACK，实际: %s", action.Act.String())
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
		t.Fatalf("写入测试文件失败: %v", err)
	}
	if err := session.LoadConfigFile(configPath); err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}

	after := PageSnapshot{PageName: "After", XML: `<hierarchy text="After"/>`, Screenshot: []byte{1, 2, 3}}
	if err := session.OnStepResult(StepResultInput{
		Step:    1,
		Action:  &types.ActionCommand{Act: types.CLICK, Pos: *types.NewRect(0, 0, 10, 10)},
		Success: false,
		Crash:   true,
		Before:  PageSnapshot{PageName: "Before", XML: `<hierarchy/>`},
		After:   &after,
	}); err != nil {
		t.Fatalf("OnStepResult 失败: %v", err)
	}

	action, err := session.NextAction("MainActivity", `<hierarchy><node class="android.widget.TextView" bounds="[0,0][10,10]"/></hierarchy>`)
	if err != nil {
		t.Fatalf("获取动作失败: %v", err)
	}
	if action.Act != types.BACK {
		t.Fatalf("onStepResult 状态应驱动下一步 BACK，实际: %s", action.Act.String())
	}
}
