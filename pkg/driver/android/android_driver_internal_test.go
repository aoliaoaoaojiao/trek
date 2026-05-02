package android

import "testing"

func TestParseScreenRotationPrefersInputSurfaceOrientation(t *testing.T) {
	rotation, source, err := parseScreenRotation(
		"SurfaceOrientation: 3\nPhysicalWidth: 1080",
		"mCurrentRotation=ROTATION_1",
		"DisplayFrames w=1080 h=1920 r=1",
	)
	if err != nil {
		t.Fatalf("预期优先命中 dumpsys input，实际报错: %v", err)
	}
	if rotation != 3 || source != "input" {
		t.Fatalf("预期 rotation=3 source=input，实际 rotation=%d source=%s", rotation, source)
	}
}

func TestParseScreenRotationFallbackToDisplay(t *testing.T) {
	rotation, source, err := parseScreenRotation(
		"",
		"DisplayViewport{valid=true, orientation=2, deviceWidth=1080, deviceHeight=2400}",
		"DisplayFrames w=1080 h=1920 r=1",
	)
	if err != nil {
		t.Fatalf("预期回退命中 dumpsys display，实际报错: %v", err)
	}
	if rotation != 2 || source != "display" {
		t.Fatalf("预期 rotation=2 source=display，实际 rotation=%d source=%s", rotation, source)
	}
}

func TestParseScreenRotationFallbackToWindow(t *testing.T) {
	rotation, source, err := parseScreenRotation(
		"",
		"",
		"WindowFrames:\n  mCurrentRotation=1\n",
	)
	if err != nil {
		t.Fatalf("预期回退命中 dumpsys window，实际报错: %v", err)
	}
	if rotation != 1 || source != "window" {
		t.Fatalf("预期 rotation=1 source=window，实际 rotation=%d source=%s", rotation, source)
	}
}

func TestParseScreenRotationSupportsRotationEnumText(t *testing.T) {
	rotation, source, err := parseScreenRotation(
		"",
		"something mCurrentRotation=ROTATION_2 something",
		"",
	)
	if err != nil {
		t.Fatalf("预期支持 ROTATION_n 文本，实际报错: %v", err)
	}
	if rotation != 2 || source != "display" {
		t.Fatalf("预期 rotation=2 source=display，实际 rotation=%d source=%s", rotation, source)
	}
}

func TestParseScreenRotationReturnsErrorWhenNoSignalFound(t *testing.T) {
	_, _, err := parseScreenRotation("", "", "")
	if err == nil {
		t.Fatalf("预期无任何信号时返回错误")
	}
}
