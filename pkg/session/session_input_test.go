package session

import (
	"testing"
	"trek/internal/engine/core/types"
	"trek/logger"
)

func initSessionTestLogger(t *testing.T) {
	t.Helper()
	if err := logger.InitLogger("log"); err != nil {
		t.Fatalf("鍒濆鍖栨祴璇曟棩蹇楀け璐? %v", err)
	}
}

func TestSessionSetObservationModeRoundTrip(t *testing.T) {
	session := NewSession(Config{PackageName: "com.demo"})

	if err := session.SetObservationMode("hybrid"); err != nil {
		t.Fatalf("璁剧疆 hybrid 妯″紡澶辫触: %v", err)
	}
	if got := session.GetObservationMode(); got != "hybrid" {
		t.Fatalf("妯″紡璇诲彇涓嶇鍚堥鏈? got=%s", got)
	}

	if err := session.SetObservationMode("xml-only"); err != nil {
		t.Fatalf("鎭㈠ xml-only 妯″紡澶辫触: %v", err)
	}
}

func TestSessionNextActionWithInputValidateEmptyPayload(t *testing.T) {
	session := NewSession(Config{PackageName: "com.demo"})

	_, err := session.NextActionWithInput("MainActivity", ActionInput{})
	if err == nil {
		t.Fatalf("棰勬湡绌鸿緭鍏ュ簲杩斿洖閿欒")
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
		t.Fatalf("璁剧疆 xml-only 澶辫触: %v", err)
	}

	action, err := session.NextActionWithInput("LoginActivity", ActionInput{
		XMLDescOfGuiTree: `<hierarchy>
	<node class="android.widget.FrameLayout" resource-id="" content-desc="" text="" clickable="false" long-clickable="false" checkable="false" enabled="true" bounds="[0,0][1080,1920]">
		<node class="android.widget.Button" resource-id="com.demo:id/login" content-desc="鐧诲綍鎸夐挳" text="鐧诲綍" clickable="true" long-clickable="false" checkable="false" enabled="true" bounds="[10,20][110,120]"/>
	</node>
</hierarchy>`,
	})
	if err != nil {
		t.Fatalf("NextActionWithInput 鎵ц澶辫触: %v", err)
	}
	if action == nil {
		t.Fatalf("鍔ㄤ綔缁撴灉涓嶈兘涓虹┖")
	}
}
