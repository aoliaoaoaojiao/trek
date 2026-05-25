package reporting

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"trek/pkg/driver/common"
	"trek/pkg/monkey"
)

func TestResolveFormat(t *testing.T) {
	tests := []struct {
		name    string
		format  string
		path    string
		want    string
		wantErr bool
	}{
		{name: "explicit json", format: "json", path: "report.md", want: FormatJSON},
		{name: "explicit markdown", format: "markdown", path: "report.json", want: FormatMD},
		{name: "infer markdown", path: "report.md", want: FormatMD},
		{name: "infer json", path: "report.json", want: FormatJSON},
		{name: "default json without ext", path: "report", want: FormatJSON},
		{name: "unsupported ext", path: "report.txt", wantErr: true},
		{name: "unsupported format", format: "html", path: "report.html", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveFormat(tt.format, tt.path)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("预期返回错误，实际成功: %s", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ResolveFormat 返回错误: %v", err)
			}
			if got != tt.want {
				t.Fatalf("格式解析错误: got=%s want=%s", got, tt.want)
			}
		})
	}
}

func TestRenderRunReportJSON(t *testing.T) {
	report := sampleReport(t)
	content, err := RenderRunReport(FormatJSON, sampleMetadata(), report)
	if err != nil {
		t.Fatalf("渲染 JSON 报告失败: %v", err)
	}

	var envelope RunReportEnvelope
	if err := json.Unmarshal(content, &envelope); err != nil {
		t.Fatalf("解析 JSON 报告失败: %v", err)
	}
	if envelope.Metadata.PackageName != "com.demo.app" {
		t.Fatalf("包名写入错误: %+v", envelope.Metadata)
	}
	if envelope.Summary.StopReason != monkey.StopCompleted {
		t.Fatalf("停止原因错误: %s", envelope.Summary.StopReason)
	}
	if len(envelope.StepRecords) != 2 {
		t.Fatalf("步骤记录数量错误: %d", len(envelope.StepRecords))
	}
}

func TestRenderRunReportMarkdown(t *testing.T) {
	report := sampleReport(t)
	content, err := RenderRunReport(FormatMD, sampleMetadata(), report)
	if err != nil {
		t.Fatalf("渲染 Markdown 报告失败: %v", err)
	}

	text := string(content)
	expected := []string{
		"# Trek 运行报告",
		"停止原因 | completed",
		"页面索引",
		"`HomePage`",
		"P1 —",
		"问题 & 警告",
		"click failed",
	}
	for _, item := range expected {
		if !strings.Contains(text, item) {
			t.Fatalf("Markdown 报告缺少内容 %q:\n%s", item, text)
		}
	}
}

func TestWriteRunReport(t *testing.T) {
	report := sampleReport(t)
	outputDir := t.TempDir()
	outputPath := filepath.Join(outputDir, "run-report.md")

	if err := WriteRunReport(outputPath, "", sampleMetadata(), report); err != nil {
		t.Fatalf("写入报告失败: %v", err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("读取报告失败: %v", err)
	}
	if !strings.Contains(string(data), "Trek 运行报告") {
		t.Fatalf("报告内容不符合预期: %s", string(data))
	}
}

func TestWriteRunReportWithArtifactsGroupedByPage(t *testing.T) {
	report := sampleReport(t)
	artifactDir := filepath.Join(t.TempDir(), "artifacts")

	envelope, err := BuildRunReportEnvelope(sampleMetadata(), report, artifactDir)
	if err != nil {
		t.Fatalf("构建带产物的报告失败: %v", err)
	}
	if envelope.Artifacts == nil {
		t.Fatal("预期生成产物摘要")
	}
	if envelope.Artifacts.PageCount != 2 {
		t.Fatalf("页面目录数错误: %+v", envelope.Artifacts)
	}
	// 方案B：只写 Before 产物，最后一步才写 After
	if envelope.Artifacts.ScreenshotCount != 1 || envelope.Artifacts.XMLCount != 1 {
		t.Fatalf("产物计数错误: %+v", envelope.Artifacts)
	}
	if len(envelope.Pages) == 0 || envelope.Pages[0].InteractableControlCount == 0 {
		t.Fatalf("预期生成页面控件摘要: %+v", envelope.Pages)
	}

	step := envelope.StepRecords[0]
	if step.BeforeArtifactRef == nil {
		t.Fatalf("预期步骤记录包含 Before 产物引用: %+v", step)
	}
	// 中间步骤不再写 After 产物
	if step.AfterArtifactRef != nil {
		t.Fatalf("中间步骤不应有 After 产物引用: %+v", step)
	}
	if step.BeforeArtifactRef.PageDir != "HomePage" {
		t.Fatalf("页面目录命名错误: %+v", step.BeforeArtifactRef)
	}
	for _, relativePath := range []string{
		step.BeforeArtifactRef.ScreenshotFile,
		step.BeforeArtifactRef.XMLFile,
		envelope.Pages[0].ControlsDetailFile,
	} {
		fullPath := relativePath
		if !filepath.IsAbs(fullPath) {
			fullPath = filepath.Join(artifactDir, filepath.FromSlash(relativePath))
		}
		if _, err := os.Stat(fullPath); err != nil {
			t.Fatalf("预期产物文件存在 %s: %v", fullPath, err)
		}
	}
}

func sampleMetadata() RunMetadata {
	return RunMetadata{
		PackageName:         "com.demo.app",
		DeviceSerial:        "emulator-5554",
		Algorithm:           "reuse",
		MaxSteps:            20,
		MaxDuration:         2 * time.Minute,
		StepInterval:        300 * time.Millisecond,
		PageSourceType:      "uia",
		PageControlStrategy: "raw",
		CaptureScreenshot:   true,
		KeepStepRecords:     true,
		ConfigPath:          "./config.js",
	}
}

func sampleReport(t *testing.T) *monkey.Report {
	t.Helper()
	startedAt := time.Date(2026, 5, 17, 20, 0, 0, 0, time.FixedZone("CST", 8*3600))
	finishedAt := startedAt.Add(5 * time.Second)
	return &monkey.Report{
		StartedAt:                   startedAt,
		FinishedAt:                  finishedAt,
		DurationMs:                  finishedAt.Sub(startedAt).Milliseconds(),
		StopReason:                  monkey.StopCompleted,
		Preflight:                   &common.EnvironmentCheckResult{ADBReady: true, DeviceReady: true, PageSourceReady: true, UIAReady: true, PageSourceType: "uia", DeviceName: "Pixel", Detail: "ok"},
		StepsPlanned:                20,
		StepsTotal:                  2,
		StepsSucceeded:              1,
		StepsFailed:                 1,
		ActionCount:                 map[string]int{"CLICK": 1, "BACK": 1},
		PageVisitCount:              map[string]int{"HomePage": 2, "DetailPage": 1},
		OutOfAppRecoveries:          1,
		RecoveryCooldownEnterCount:  2,
		RecoveryCooldownStepCount:   3,
		CandidateEnhancementCalls:   4,
		CandidateEnhancementSelects: 1,
		RecoveryLLMCalls:            0,
		RecoveryLLMBudgetDenied:     0,
		EnhancementLLMBudgetDenied:  0,
		Records: []monkey.StepRecord{
			{
				Step:               1,
				PageName:           "HomePage",
				Action:             "CLICK",
				ActionTargetBounds: "[10.000,10.000,100.000,60.000]",
				ActionWidgetInfo:   `Widget{path:/hierarchy/node, xpath:/hierarchy[1]/node[1], bounds:[10,10][100,60], actions:[CLICK]}`,
				DurationMs:         120,
				BeforePageName:     "HomePage",
				AfterPageName:      "DetailPage",
				BeforeXML:          `<hierarchy><node text="登录按钮" class="android.widget.Button" clickable="true" enabled="true" bounds="[10,10][100,60]"/></hierarchy>`,
				AfterXML:           `<hierarchy><node text="详情按钮" class="android.widget.Button" clickable="true" enabled="true" bounds="[20,20][120,80]"/></hierarchy>`,
				BeforeScreenshot:   samplePNG(t, color.RGBA{R: 255, A: 255}),
				AfterScreenshot:    samplePNG(t, color.RGBA{G: 255, A: 255}),
			},
			{Step: 2, PageName: "HomePage", Action: "BACK", DurationMs: 180, Err: "click failed"},
		},
	}
}

func samplePNG(t *testing.T, fill color.RGBA) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			img.SetRGBA(x, y, fill)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("编码 png 失败: %v", err)
	}
	return buf.Bytes()
}
