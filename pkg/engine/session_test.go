package engine

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
		<node class="android.widget.Button" resource-id="com.demo:id/login" content-desc="登录按钮" text="登录" clickable="true" long-clickable="false" checkable="false" enabled="true" bounds="[10,20][110,120]"/>
	</node>
</hierarchy>`)
	if err != nil {
		t.Fatalf("生成动作失败: %v", err)
	}

	if action == nil {
		t.Fatalf("动作结果为空")
	}
}

func TestSessionCheckPointInBlackRects(t *testing.T) {
	session := NewSession(Config{
		PackageName: "com.demo",
	})

	configPath := filepath.Join(t.TempDir(), "mix.json")
	configContent := `{"black_rects":{"LoginActivity":[[0,0,100,100]]}}`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("写入测试配置失败: %v", err)
	}
	if err := session.LoadPreferenceFile(configPath); err != nil {
		t.Fatalf("加载偏好配置失败: %v", err)
	}

	if !session.CheckPointInBlackRects("LoginActivity", types.Point{X: 50, Y: 50}) {
		t.Fatalf("黑名单判断结果不正确")
	}
}
