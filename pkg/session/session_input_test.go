package session

import (
	"testing"
	"trek/internal/engine/decision"
	"trek/internal/engine/decision/shared/types"
	"trek/logger"
)

func initSessionTestLogger(t *testing.T) {
	t.Helper()
	if err := logger.InitLogger("log"); err != nil {
		t.Fatalf("init logger failed: %v", err)
	}
}

func TestSessionSetObservationModeRoundTrip(t *testing.T) {
	session, err := NewSession(Config{PackageName: "com.demo"})
	if err != nil {
		t.Fatalf("创建会话失败: %v", err)
	}

	if err := session.SetObservationMode("hybrid"); err != nil {
		t.Fatalf("set hybrid mode failed: %v", err)
	}
	if got := session.GetObservationMode(); got != "hybrid" {
		t.Fatalf("unexpected observation mode: got=%s", got)
	}

	if err := session.SetObservationMode("xml-only"); err != nil {
		t.Fatalf("set xml-only mode failed: %v", err)
	}
}

func TestSessionNextActionWithInputValidateEmptyPayload(t *testing.T) {
	session, err := NewSession(Config{PackageName: "com.demo"})
	if err != nil {
		t.Fatalf("创建会话失败: %v", err)
	}

	_, err = session.NextActionWithInput("MainActivity", ActionInput{})
	if err == nil {
		t.Fatalf("expected error for empty action input")
	}
}

func TestSessionNextActionWithInputXMLCompatible(t *testing.T) {
	initSessionTestLogger(t)

	session, err := NewSession(Config{
		PackageName: "com.demo",
		Algorithm:   decision.AlgorithmReuse,
		DeviceType:  types.Phone,
	})
	if err != nil {
		t.Fatalf("创建会话失败: %v", err)
	}
	if err := session.SetObservationMode("xml-only"); err != nil {
		t.Fatalf("set xml-only failed: %v", err)
	}

	action, err := session.NextActionWithInput("LoginActivity", ActionInput{
		XMLDescOfGuiTree: `<hierarchy>
	<node class="android.widget.FrameLayout" resource-id="" content-desc="" text="" clickable="false" long-clickable="false" checkable="false" enabled="true" bounds="[0,0][1080,1920]">
		<node class="android.widget.Button" resource-id="com.demo:id/login" content-desc="鐧诲綍鎸夐挳" text="鐧诲綍" clickable="true" long-clickable="false" checkable="false" enabled="true" bounds="[10,20][110,120]"/>
	</node>
</hierarchy>`,
	})
	if err != nil {
		t.Fatalf("NextActionWithInput failed: %v", err)
	}
	if action == nil {
		t.Fatalf("expected non-nil action")
	}
}
