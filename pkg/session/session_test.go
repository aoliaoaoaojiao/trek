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
