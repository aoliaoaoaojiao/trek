package engine

import (
	"testing"
	"trek/internal/engine/core/types"
	"trek/logger"
)

func initSessionTestLogger(t *testing.T) {
	t.Helper()
	if err := logger.InitLogger("log"); err != nil {
		t.Fatalf("初始化测试日志失败: %v", err)
	}
}

func TestSessionSetObservationModeRoundTrip(t *testing.T) {
	session := NewSession(Config{PackageName: "com.demo"})

	if err := session.SetObservationMode("hybrid"); err != nil {
		t.Fatalf("设置 hybrid 模式失败: %v", err)
	}
	if got := session.GetObservationMode(); got != "hybrid" {
		t.Fatalf("模式读取不符合预期, got=%s", got)
	}

	if err := session.SetObservationMode("xml-only"); err != nil {
		t.Fatalf("恢复 xml-only 模式失败: %v", err)
	}
}

func TestSessionNextActionWithInputValidateEmptyPayload(t *testing.T) {
	session := NewSession(Config{PackageName: "com.demo"})

	_, err := session.NextActionWithInput("MainActivity", ActionInput{})
	if err == nil {
		t.Fatalf("预期空输入应返回错误")
	}
}

func TestSessionNextActionWithInputXMLCompatible(t *testing.T) {
	initSessionTestLogger(t)

	session := NewSession(Config{
		PackageName: "com.demo",
		Algorithm:   types.Reuse,
		DeviceType:  types.Phone,
	})
	if err := session.SetObservationMode("xml-only"); err != nil {
		t.Fatalf("设置 xml-only 失败: %v", err)
	}

	action, err := session.NextActionWithInput("LoginActivity", ActionInput{
		XMLDescOfGuiTree: `<hierarchy>
	<node class="android.widget.FrameLayout" resource-id="" content-desc="" text="" clickable="false" long-clickable="false" checkable="false" enabled="true" bounds="[0,0][1080,1920]">
		<node class="android.widget.Button" resource-id="com.demo:id/login" content-desc="登录按钮" text="登录" clickable="true" long-clickable="false" checkable="false" enabled="true" bounds="[10,20][110,120]"/>
	</node>
</hierarchy>`,
	})
	if err != nil {
		t.Fatalf("NextActionWithInput 执行失败: %v", err)
	}
	if action == nil {
		t.Fatalf("动作结果不能为空")
	}
}
