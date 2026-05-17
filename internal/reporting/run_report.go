package reporting

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/beevik/etree"

	"trek/pkg/driver/common"
	"trek/pkg/monkey"
)

var (
	widgetXPathRegex = regexp.MustCompile(`(?:^|[,{ ])xpath:([^,}]+)`)
	widgetPathRegex  = regexp.MustCompile(`(?:^|[,{ ])path:([^,}]+)`)
)

const (
	FormatJSON = "json"
	FormatMD   = "md"
)

const (
	defaultTopCount         = 10
	defaultPageControlLimit = 20
)

// RunMetadata 描述一次运行的关键上下文，便于报告复盘。
type RunMetadata struct {
	PackageName         string        `json:"package_name"`
	DeviceSerial        string        `json:"device_serial,omitempty"`
	Algorithm           string        `json:"algorithm"`
	MaxSteps            int           `json:"max_steps"`
	MaxDuration         time.Duration `json:"max_duration"`
	StepInterval        time.Duration `json:"step_interval"`
	PageSourceType      string        `json:"page_source_type"`
	PageControlStrategy string        `json:"page_control_strategy"`
	CaptureScreenshot   bool          `json:"capture_screenshot"`
	KeepStepRecords     bool          `json:"keep_step_records"`
	ConfigPath          string        `json:"config_path,omitempty"`
}

// RunReportEnvelope 是对外输出的稳定报告结构。
type RunReportEnvelope struct {
	GeneratedAt time.Time                      `json:"generated_at"`
	Metadata    RunMetadata                    `json:"metadata"`
	Summary     RunSummary                     `json:"summary"`
	Preflight   *common.EnvironmentCheckResult `json:"preflight,omitempty"`
	Pages       []PageSummary                  `json:"pages,omitempty"`
	Artifacts   *ArtifactSummary               `json:"artifacts,omitempty"`
	StepRecords []monkey.StepRecord            `json:"step_records,omitempty"`
}

// RunSummary 聚合运行阶段的关键指标，方便后续前端或脚本直接消费。
type RunSummary struct {
	StartedAt                   time.Time         `json:"started_at"`
	FinishedAt                  time.Time         `json:"finished_at"`
	DurationMs                  int64             `json:"duration_ms"`
	StopReason                  monkey.StopReason `json:"stop_reason"`
	PreflightError              string            `json:"preflight_error,omitempty"`
	StepsPlanned                int               `json:"steps_planned"`
	StepsTotal                  int               `json:"steps_total"`
	StepsSucceeded              int               `json:"steps_succeeded"`
	StepsFailed                 int               `json:"steps_failed"`
	ConsecutiveFailures         int               `json:"consecutive_failures"`
	OutOfAppRecoveries          int               `json:"out_of_app_recoveries"`
	RecoveryCooldownEnterCount  int               `json:"recovery_cooldown_enter_count"`
	RecoveryCooldownStepCount   int               `json:"recovery_cooldown_step_count"`
	CandidateEnhancementCalls   int               `json:"candidate_enhancement_calls"`
	CandidateEnhancementSelects int               `json:"candidate_enhancement_selects"`
	RecoveryLLMCalls            int               `json:"recovery_llm_calls"`
	RecoveryLLMBudgetDenied     int               `json:"recovery_llm_budget_denied"`
	EnhancementLLMBudgetDenied  int               `json:"enhancement_llm_budget_denied"`
	ActionCount                 map[string]int    `json:"action_count,omitempty"`
	PageVisitCount              map[string]int    `json:"page_visit_count,omitempty"`
}

type pair struct {
	Name  string
	Count int
}

// ArtifactSummary 描述本次导出的原始页面产物目录。
type ArtifactSummary struct {
	RootDir         string `json:"root_dir,omitempty"`
	PageCount       int    `json:"page_count,omitempty"`
	FileCount       int    `json:"file_count,omitempty"`
	ScreenshotCount int    `json:"screenshot_count,omitempty"`
	XMLCount        int    `json:"xml_count,omitempty"`
}

// PageSummary 表示页面级人工复盘摘要。
type PageSummary struct {
	PageName                 string           `json:"page_name"`
	VisitCount               int              `json:"visit_count"`
	ActionCount              int              `json:"action_count"`
	InteractableControlCount int              `json:"interactable_control_count"`
	ArtifactDir              string           `json:"artifact_dir,omitempty"`
	ControlsDetailFile       string           `json:"controls_detail_file,omitempty"`
	TopActions               map[string]int   `json:"top_actions,omitempty"`
	Controls                 []ControlSummary `json:"controls,omitempty"`
	TopControls              []ControlSummary `json:"top_controls,omitempty"`
}

// ControlSummary 表示一个可交互控件的聚合摘要。
type ControlSummary struct {
	Key           string         `json:"key,omitempty"`
	Label         string         `json:"label,omitempty"`
	ControlType   string         `json:"control_type,omitempty"`
	Bounds        string         `json:"bounds,omitempty"`
	Actions       []string       `json:"actions,omitempty"`
	SeenCount     int            `json:"seen_count"`
	ExecutedCount int            `json:"executed_count"`
	ExecutedBy    map[string]int `json:"executed_by,omitempty"`
}

// WriteRunReport 将运行报告写入指定路径。
func WriteRunReport(path string, format string, metadata RunMetadata, report *monkey.Report) error {
	return WriteRunReportWithArtifacts(path, format, "", metadata, report)
}

// WriteRunReportWithArtifacts 将运行报告和页面产物写入指定位置。
func WriteRunReportWithArtifacts(path string, format string, artifactDir string, metadata RunMetadata, report *monkey.Report) error {
	targetPath := strings.TrimSpace(path)
	if targetPath == "" {
		return fmt.Errorf("报告输出路径不能为空")
	}
	if report == nil {
		return fmt.Errorf("运行报告不能为空")
	}

	resolvedFormat, err := ResolveFormat(format, targetPath)
	if err != nil {
		return err
	}

	envelope, err := BuildRunReportEnvelope(metadata, report, artifactDir)
	if err != nil {
		return err
	}
	content, err := renderEnvelope(resolvedFormat, envelope)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return fmt.Errorf("创建报告目录失败: %w", err)
	}
	if err := os.WriteFile(targetPath, content, 0644); err != nil {
		return fmt.Errorf("写入报告文件失败: %w", err)
	}
	return nil
}

// ResolveFormat 解析报告格式；未显式指定时优先按扩展名推断。
func ResolveFormat(format string, path string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(format))
	switch normalized {
	case "":
		ext := strings.ToLower(strings.TrimSpace(filepath.Ext(path)))
		switch ext {
		case ".md", ".markdown":
			return FormatMD, nil
		case ".json", "":
			return FormatJSON, nil
		default:
			return "", fmt.Errorf("无法根据扩展名推断报告格式: %s，请显式指定 --report-format", ext)
		}
	case "json":
		return FormatJSON, nil
	case "md", "markdown":
		return FormatMD, nil
	default:
		return "", fmt.Errorf("不支持的报告格式: %s，仅支持 json / md", format)
	}
}

// RenderRunReport 按指定格式渲染运行报告。
func RenderRunReport(format string, metadata RunMetadata, report *monkey.Report) ([]byte, error) {
	envelope, err := BuildRunReportEnvelope(metadata, report, "")
	if err != nil {
		return nil, err
	}
	return renderEnvelope(format, envelope)
}

// BuildRunReportEnvelope 构建最终报告结构，并按需输出原始页面产物。
func BuildRunReportEnvelope(metadata RunMetadata, report *monkey.Report, artifactDir string) (RunReportEnvelope, error) {
	envelope := buildEnvelope(metadata, report)
	pages := buildPageSummaries(envelope.StepRecords)
	envelope.Pages = pages
	if strings.TrimSpace(artifactDir) == "" {
		return envelope, nil
	}
	summary, records, pages, err := writeArtifacts(artifactDir, envelope.StepRecords, pages)
	if err != nil {
		return RunReportEnvelope{}, err
	}
	envelope.Artifacts = summary
	envelope.StepRecords = records
	envelope.Pages = pages
	return envelope, nil
}

func renderEnvelope(format string, envelope RunReportEnvelope) ([]byte, error) {
	switch format {
	case FormatJSON:
		return json.MarshalIndent(envelope, "", "  ")
	case FormatMD:
		return []byte(renderMarkdown(envelope)), nil
	default:
		return nil, fmt.Errorf("不支持的报告格式: %s", format)
	}
}

func buildEnvelope(metadata RunMetadata, report *monkey.Report) RunReportEnvelope {
	return RunReportEnvelope{
		GeneratedAt: time.Now(),
		Metadata:    metadata,
		Summary: RunSummary{
			StartedAt:                   report.StartedAt,
			FinishedAt:                  report.FinishedAt,
			DurationMs:                  report.DurationMs,
			StopReason:                  report.StopReason,
			PreflightError:              report.PreflightError,
			StepsPlanned:                report.StepsPlanned,
			StepsTotal:                  report.StepsTotal,
			StepsSucceeded:              report.StepsSucceeded,
			StepsFailed:                 report.StepsFailed,
			ConsecutiveFailures:         report.ConsecutiveFailures,
			OutOfAppRecoveries:          report.OutOfAppRecoveries,
			RecoveryCooldownEnterCount:  report.RecoveryCooldownEnterCount,
			RecoveryCooldownStepCount:   report.RecoveryCooldownStepCount,
			CandidateEnhancementCalls:   report.CandidateEnhancementCalls,
			CandidateEnhancementSelects: report.CandidateEnhancementSelects,
			RecoveryLLMCalls:            report.RecoveryLLMCalls,
			RecoveryLLMBudgetDenied:     report.RecoveryLLMBudgetDenied,
			EnhancementLLMBudgetDenied:  report.EnhancementLLMBudgetDenied,
			ActionCount:                 cloneCountMap(report.ActionCount),
			PageVisitCount:              cloneCountMap(report.PageVisitCount),
		},
		Preflight:   report.Preflight,
		Pages:       nil,
		StepRecords: append([]monkey.StepRecord(nil), report.Records...),
	}
}

func renderMarkdown(envelope RunReportEnvelope) string {
	var b strings.Builder
	summary := envelope.Summary
	meta := envelope.Metadata

	b.WriteString("# Trek 运行报告\n\n")
	b.WriteString("## 执行摘要\n\n")
	b.WriteString(fmt.Sprintf("- 生成时间：%s\n", formatTime(envelope.GeneratedAt)))
	b.WriteString(fmt.Sprintf("- 包名：%s\n", fallbackText(meta.PackageName, "-")))
	b.WriteString(fmt.Sprintf("- 设备序列号：%s\n", fallbackText(meta.DeviceSerial, "-")))
	b.WriteString(fmt.Sprintf("- 算法：%s\n", fallbackText(meta.Algorithm, "-")))
	b.WriteString(fmt.Sprintf("- 停止原因：%s\n", summary.StopReason))
	b.WriteString(fmt.Sprintf("- 运行时长：%d ms\n", summary.DurationMs))
	b.WriteString(fmt.Sprintf("- 执行步数：%d / %d\n", summary.StepsTotal, summary.StepsPlanned))
	b.WriteString(fmt.Sprintf("- 成功 / 失败：%d / %d\n", summary.StepsSucceeded, summary.StepsFailed))
	b.WriteString(fmt.Sprintf("- 页面源：%s\n", fallbackText(meta.PageSourceType, "-")))
	b.WriteString(fmt.Sprintf("- 页面控件策略：%s\n", fallbackText(meta.PageControlStrategy, "raw")))
	b.WriteString(fmt.Sprintf("- 截图采集：%t\n", meta.CaptureScreenshot))
	b.WriteString(fmt.Sprintf("- 保留步骤记录：%t\n", meta.KeepStepRecords))
	if strings.TrimSpace(meta.ConfigPath) != "" {
		b.WriteString(fmt.Sprintf("- 配置文件：%s\n", meta.ConfigPath))
	}
	if envelope.Artifacts != nil {
		b.WriteString(fmt.Sprintf("- 原始产物目录：%s\n", fallbackText(envelope.Artifacts.RootDir, "-")))
	}
	b.WriteString("\n")

	if envelope.Preflight != nil {
		b.WriteString("## 前置检查\n\n")
		b.WriteString(fmt.Sprintf("- ADB 就绪：%t\n", envelope.Preflight.ADBReady))
		b.WriteString(fmt.Sprintf("- 设备就绪：%t\n", envelope.Preflight.DeviceReady))
		b.WriteString(fmt.Sprintf("- 页面源就绪：%t\n", envelope.Preflight.PageSourceReady))
		b.WriteString(fmt.Sprintf("- UIA 就绪：%t\n", envelope.Preflight.UIAReady))
		b.WriteString(fmt.Sprintf("- 设备名称：%s\n", fallbackText(envelope.Preflight.DeviceName, "-")))
		b.WriteString(fmt.Sprintf("- 页面源类型：%s\n", fallbackText(envelope.Preflight.PageSourceType, "-")))
		b.WriteString(fmt.Sprintf("- 细节：%s\n", fallbackText(envelope.Preflight.Detail, "-")))
		if summary.PreflightError != "" {
			b.WriteString(fmt.Sprintf("- 前置检查错误：%s\n", summary.PreflightError))
		}
		b.WriteString("\n")
	}

	b.WriteString("## 关键统计\n\n")
	b.WriteString(fmt.Sprintf("- 离开应用自动拉回：%d\n", summary.OutOfAppRecoveries))
	b.WriteString(fmt.Sprintf("- 恢复冷却进入次数：%d\n", summary.RecoveryCooldownEnterCount))
	b.WriteString(fmt.Sprintf("- 恢复冷却命中步数：%d\n", summary.RecoveryCooldownStepCount))
	b.WriteString(fmt.Sprintf("- 候选增强调用 / 命中：%d / %d\n", summary.CandidateEnhancementCalls, summary.CandidateEnhancementSelects))
	b.WriteString(fmt.Sprintf("- 恢复 LLM 调用：%d\n", summary.RecoveryLLMCalls))
	b.WriteString(fmt.Sprintf("- 恢复 LLM 预算拒绝：%d\n", summary.RecoveryLLMBudgetDenied))
	b.WriteString(fmt.Sprintf("- 增强 LLM 预算拒绝：%d\n", summary.EnhancementLLMBudgetDenied))
	b.WriteString("\n")

	if envelope.Artifacts != nil {
		b.WriteString("## 产物导出\n\n")
		b.WriteString(fmt.Sprintf("- 页面目录数：%d\n", envelope.Artifacts.PageCount))
		b.WriteString(fmt.Sprintf("- 截图文件数：%d\n", envelope.Artifacts.ScreenshotCount))
		b.WriteString(fmt.Sprintf("- XML 文件数：%d\n", envelope.Artifacts.XMLCount))
		b.WriteString(fmt.Sprintf("- 总文件数：%d\n", envelope.Artifacts.FileCount))
		b.WriteString("\n")
	}

	writeTopSection(&b, "高频页面", summary.PageVisitCount)
	writeTopSection(&b, "高频动作", summary.ActionCount)
	writePageDetails(&b, envelope.Pages)
	writeRecentFailures(&b, envelope.StepRecords)

	return b.String()
}

func writeTopSection(b *strings.Builder, title string, counts map[string]int) {
	b.WriteString("## " + title + "\n\n")
	top := topPairs(counts, defaultTopCount)
	if len(top) == 0 {
		b.WriteString("- 无\n\n")
		return
	}
	for _, item := range top {
		b.WriteString(fmt.Sprintf("- %s：%d\n", fallbackText(item.Name, "<empty>"), item.Count))
	}
	b.WriteString("\n")
}

func writeRecentFailures(b *strings.Builder, records []monkey.StepRecord) {
	b.WriteString("## 最近失败步骤\n\n")
	failures := make([]monkey.StepRecord, 0, 5)
	for i := len(records) - 1; i >= 0; i-- {
		record := records[i]
		if strings.TrimSpace(record.Err) == "" {
			continue
		}
		failures = append(failures, record)
		if len(failures) >= 5 {
			break
		}
	}
	if len(failures) == 0 {
		b.WriteString("- 无\n")
		return
	}
	for _, record := range failures {
		b.WriteString(fmt.Sprintf("- Step %d | 前页面=%s | 操作=%s | 后页面=%s | 错误=%s | 耗时=%d ms\n",
			record.Step,
			fallbackText(firstNonEmpty(record.BeforePageName, record.PageName), "-"),
			fallbackText(record.Action, "-"),
			fallbackText(firstNonEmpty(record.AfterPageName, record.PageName), "-"),
			record.Err,
			record.DurationMs,
		))
	}
}

func writePageDetails(b *strings.Builder, pages []PageSummary) {
	b.WriteString("## 页面详情\n\n")
	if len(pages) == 0 {
		b.WriteString("- 无\n\n")
		return
	}
	for _, page := range pages {
		b.WriteString(fmt.Sprintf("### 页面：%s\n\n", fallbackText(page.PageName, "UnknownPage")))
		b.WriteString(fmt.Sprintf("- 出现次数：%d\n", page.VisitCount))
		b.WriteString(fmt.Sprintf("- 执行操作次数：%d\n", page.ActionCount))
		b.WriteString(fmt.Sprintf("- 可交互控件数：%d\n", page.InteractableControlCount))
		if page.ArtifactDir != "" {
			b.WriteString(fmt.Sprintf("- 产物目录：%s\n", page.ArtifactDir))
		}
		if page.ControlsDetailFile != "" {
			b.WriteString(fmt.Sprintf("- 完整控件明细：%s\n", page.ControlsDetailFile))
		}
		if len(page.TopActions) > 0 {
			b.WriteString(fmt.Sprintf("- 常见操作：%s\n", formatCountMapInline(page.TopActions)))
		}
		b.WriteString("\n")
		if len(page.TopControls) == 0 {
			b.WriteString("- 控件详情：无\n\n")
			continue
		}
		b.WriteString("| 文本/描述 | 类型 | 坐标 | 可执行动作 | 出现次数 | 实际执行 |\n")
		b.WriteString("|---|---|---|---|---:|---|\n")
		for _, control := range page.TopControls {
			b.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %d | %s |\n",
				escapeMarkdownCell(fallbackText(control.Label, "-")),
				escapeMarkdownCell(fallbackText(control.ControlType, "-")),
				escapeMarkdownCell(fallbackText(control.Bounds, "-")),
				escapeMarkdownCell(strings.Join(control.Actions, ", ")),
				control.SeenCount,
				escapeMarkdownCell(formatCountMapInline(control.ExecutedBy)),
			))
		}
		b.WriteString("\n")
	}
}

func topPairs(counts map[string]int, limit int) []pair {
	if len(counts) == 0 || limit <= 0 {
		return nil
	}
	items := make([]pair, 0, len(counts))
	for name, count := range counts {
		items = append(items, pair{Name: name, Count: count})
	}
	slices.SortFunc(items, func(a pair, b pair) int {
		if a.Count != b.Count {
			return b.Count - a.Count
		}
		return strings.Compare(a.Name, b.Name)
	})
	if len(items) > limit {
		items = items[:limit]
	}
	return items
}

func cloneCountMap(src map[string]int) map[string]int {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]int, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return "-"
	}
	return value.Format(time.RFC3339)
}

func fallbackText(text string, fallback string) string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return fallback
	}
	return trimmed
}

func writeArtifacts(rootDir string, records []monkey.StepRecord, pages []PageSummary) (*ArtifactSummary, []monkey.StepRecord, []PageSummary, error) {
	targetRoot := strings.TrimSpace(rootDir)
	if targetRoot == "" {
		return nil, records, pages, nil
	}
	if err := os.MkdirAll(targetRoot, 0755); err != nil {
		return nil, nil, nil, fmt.Errorf("创建产物目录失败: %w", err)
	}

	cloned := append([]monkey.StepRecord(nil), records...)
	updatedPages := append([]PageSummary(nil), pages...)
	pageDirs := make(map[string]struct{})
	screenshotCount := 0
	xmlCount := 0

	for index := range cloned {
		record := &cloned[index]
		beforeRef, beforeFiles, err := writeStepSnapshotArtifacts(targetRoot, *record, "before", record.BeforePageName, record.BeforeXML, record.BeforeScreenshot)
		if err != nil {
			return nil, nil, nil, err
		}
		record.BeforeArtifactRef = beforeRef
		if beforeRef != nil {
			pageDirs[beforeRef.PageDir] = struct{}{}
		}
		screenshotCount += beforeFiles.screenshotCount
		xmlCount += beforeFiles.xmlCount

		afterRef, afterFiles, err := writeStepSnapshotArtifacts(targetRoot, *record, "after", record.AfterPageName, record.AfterXML, record.AfterScreenshot)
		if err != nil {
			return nil, nil, nil, err
		}
		record.AfterArtifactRef = afterRef
		if afterRef != nil {
			pageDirs[afterRef.PageDir] = struct{}{}
		}
		screenshotCount += afterFiles.screenshotCount
		xmlCount += afterFiles.xmlCount
	}

	for index := range updatedPages {
		page := &updatedPages[index]
		dirName := sanitizePageDirName(page.PageName)
		if dirName == "" {
			dirName = "UnknownPage"
		}
		page.ArtifactDir = filepath.ToSlash(filepath.Join(targetRoot, dirName))
		detailPath, err := writePageControlsDetail(targetRoot, dirName, *page)
		if err != nil {
			return nil, nil, nil, err
		}
		page.ControlsDetailFile = detailPath
		pageDirs[dirName] = struct{}{}
	}

	return &ArtifactSummary{
		RootDir:         targetRoot,
		PageCount:       len(pageDirs),
		FileCount:       screenshotCount + xmlCount,
		ScreenshotCount: screenshotCount,
		XMLCount:        xmlCount,
	}, cloned, updatedPages, nil
}

type artifactFileCounter struct {
	screenshotCount int
	xmlCount        int
}

func writeStepSnapshotArtifacts(rootDir string, record monkey.StepRecord, phase string, pageName string, xmlText string, screenshot []byte) (*monkey.StepArtifactRef, artifactFileCounter, error) {
	pageDirName := sanitizePageDirName(pageName)
	if strings.TrimSpace(pageDirName) == "" {
		pageDirName = "UnknownPage"
	}
	pageDirPath := filepath.Join(rootDir, pageDirName)
	ref := &monkey.StepArtifactRef{PageDir: pageDirName}
	counter := artifactFileCounter{}
	needWrite := len(screenshot) > 0 || strings.TrimSpace(xmlText) != ""
	if !needWrite {
		return nil, counter, nil
	}
	if err := os.MkdirAll(pageDirPath, 0755); err != nil {
		return nil, counter, fmt.Errorf("创建页面产物目录失败(%s): %w", pageDirName, err)
	}

	prefix := buildArtifactFilePrefix(record, phase)
	if len(screenshot) > 0 {
		ext := detectImageExt(screenshot)
		fileName := prefix + ext
		if err := os.WriteFile(filepath.Join(pageDirPath, fileName), screenshot, 0644); err != nil {
			return nil, counter, fmt.Errorf("写入截图产物失败(%s): %w", fileName, err)
		}
		ref.ScreenshotFile = filepath.ToSlash(filepath.Join(pageDirName, fileName))
		counter.screenshotCount++
	}
	if strings.TrimSpace(xmlText) != "" {
		fileName := prefix + ".xml"
		if err := os.WriteFile(filepath.Join(pageDirPath, fileName), []byte(xmlText), 0644); err != nil {
			return nil, counter, fmt.Errorf("写入 XML 产物失败(%s): %w", fileName, err)
		}
		ref.XMLFile = filepath.ToSlash(filepath.Join(pageDirName, fileName))
		counter.xmlCount++
	}
	if ref.ScreenshotFile == "" && ref.XMLFile == "" {
		return nil, counter, nil
	}
	return ref, counter, nil
}

func buildArtifactFilePrefix(record monkey.StepRecord, phase string) string {
	var b strings.Builder
	b.WriteString("step-")
	b.WriteString(strconv.FormatInt(int64(record.Step), 10))
	b.WriteString("-")
	b.WriteString(phase)
	if action := strings.TrimSpace(record.Action); action != "" {
		b.WriteString("-")
		b.WriteString(sanitizePageDirName(action))
	}
	return b.String()
}

func sanitizePageDirName(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range trimmed {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_' || r == '.':
			b.WriteRune(r)
		case r == '/' || r == '\\' || r == ':' || r == '*' || r == '?' || r == '"' || r == '<' || r == '>' || r == '|':
			b.WriteRune('_')
		case r == ' ' || r == '\t':
			b.WriteRune('_')
		default:
			b.WriteRune(r)
		}
	}
	return strings.Trim(b.String(), "._")
}

func detectImageExt(data []byte) string {
	if len(data) == 0 {
		return ".png"
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err == nil && cfg.Width > 0 && cfg.Height > 0 {
		switch strings.ToLower(strings.TrimSpace(format)) {
		case "jpeg":
			return ".jpg"
		case "png":
			return ".png"
		}
	}
	return ".png"
}

type pageAggregate struct {
	PageName    string
	VisitCount  int
	ActionCount int
	Actions     map[string]int
	Controls    map[string]*controlAggregate
}

type controlAggregate struct {
	Key         string
	Label       string
	ControlType string
	Bounds      string
	Path        string
	XPath       string
	Actions     map[string]struct{}
	SeenCount   int
	ExecutedBy  map[string]int
}

type interactableControl struct {
	Key         string
	Label       string
	ControlType string
	Bounds      string
	Path        string
	XPath       string
	Actions     []string
}

func buildPageSummaries(records []monkey.StepRecord) []PageSummary {
	pageMap := make(map[string]*pageAggregate)
	for _, record := range records {
		pageName := strings.TrimSpace(firstNonEmpty(record.BeforePageName, record.PageName))
		if pageName != "" {
			page := ensurePageAggregate(pageMap, pageName)
			page.VisitCount++
			if action := strings.TrimSpace(record.Action); action != "" {
				page.ActionCount++
				page.Actions[action]++
			}
			mergePageControls(page, extractInteractableControls(record.BeforeXML))
			markExecutedControl(page, record.Action, record.BeforeXML, record.ActionWidgetInfo, record.ActionTargetBounds)
		}
		afterPageName := strings.TrimSpace(record.AfterPageName)
		if afterPageName != "" && afterPageName != pageName {
			page := ensurePageAggregate(pageMap, afterPageName)
			page.VisitCount++
			mergePageControls(page, extractInteractableControls(record.AfterXML))
		}
	}

	pages := make([]PageSummary, 0, len(pageMap))
	for _, page := range pageMap {
		topControls := make([]ControlSummary, 0, len(page.Controls))
		for _, control := range page.Controls {
			topControls = append(topControls, ControlSummary{
				Key:           control.Key,
				Label:         control.Label,
				ControlType:   control.ControlType,
				Bounds:        control.Bounds,
				Actions:       sortedKeys(control.Actions),
				SeenCount:     control.SeenCount,
				ExecutedCount: sumCountMap(control.ExecutedBy),
				ExecutedBy:    cloneCountMap(control.ExecutedBy),
			})
		}
		slices.SortFunc(topControls, func(a, b ControlSummary) int {
			if a.ExecutedCount != b.ExecutedCount {
				return b.ExecutedCount - a.ExecutedCount
			}
			if a.SeenCount != b.SeenCount {
				return b.SeenCount - a.SeenCount
			}
			return strings.Compare(a.Key, b.Key)
		})
		summary := PageSummary{
			PageName:                 page.PageName,
			VisitCount:               page.VisitCount,
			ActionCount:              page.ActionCount,
			InteractableControlCount: len(topControls),
			TopActions:               cloneCountMap(page.Actions),
			Controls:                 append([]ControlSummary(nil), topControls...),
			TopControls:              append([]ControlSummary(nil), topControls...),
		}
		if len(summary.TopControls) > defaultPageControlLimit {
			summary.TopControls = summary.TopControls[:defaultPageControlLimit]
		}
		pages = append(pages, summary)
	}
	slices.SortFunc(pages, func(a, b PageSummary) int {
		if a.VisitCount != b.VisitCount {
			return b.VisitCount - a.VisitCount
		}
		return strings.Compare(a.PageName, b.PageName)
	})
	return pages
}

func ensurePageAggregate(pageMap map[string]*pageAggregate, pageName string) *pageAggregate {
	if page, ok := pageMap[pageName]; ok {
		return page
	}
	page := &pageAggregate{
		PageName: pageName,
		Actions:  make(map[string]int),
		Controls: make(map[string]*controlAggregate),
	}
	pageMap[pageName] = page
	return page
}

func mergePageControls(page *pageAggregate, controls []interactableControl) {
	if page == nil {
		return
	}
	for _, control := range controls {
		item, ok := page.Controls[control.Key]
		if !ok {
			item = &controlAggregate{
				Key:         control.Key,
				Label:       control.Label,
				ControlType: control.ControlType,
				Bounds:      control.Bounds,
				Path:        control.Path,
				XPath:       control.XPath,
				Actions:     make(map[string]struct{}),
				ExecutedBy:  make(map[string]int),
			}
			page.Controls[control.Key] = item
		}
		item.SeenCount++
		for _, action := range control.Actions {
			item.Actions[action] = struct{}{}
		}
	}
}

func markExecutedControl(page *pageAggregate, action string, xmlText string, widgetInfo string, targetBounds string) {
	if page == nil {
		return
	}
	action = strings.TrimSpace(action)
	if action == "" {
		return
	}
	targetXPath, targetPath := extractWidgetLocator(widgetInfo)
	for _, control := range extractInteractableControls(xmlText) {
		item, ok := page.Controls[control.Key]
		if !ok {
			continue
		}
		if !controlSupportsAction(control, action) {
			continue
		}
		if targetXPath != "" && control.XPath == targetXPath {
			item.ExecutedBy[action]++
			return
		}
		if targetPath != "" && control.Path == targetPath {
			item.ExecutedBy[action]++
			return
		}
		if targetBounds != "" && sameBounds(control.Bounds, targetBounds) {
			item.ExecutedBy[action]++
			return
		}
	}
}

func extractInteractableControls(xmlText string) []interactableControl {
	if strings.TrimSpace(xmlText) == "" {
		return nil
	}
	doc := etree.NewDocument()
	if err := doc.ReadFromString(xmlText); err != nil {
		return nil
	}
	root := doc.Root()
	if root == nil {
		return nil
	}
	controls := make([]interactableControl, 0)
	seen := make(map[string]struct{})
	var walk func(elem *etree.Element)
	walk = func(elem *etree.Element) {
		if elem == nil {
			return
		}
		if control, ok := buildInteractableControl(elem); ok {
			if _, exists := seen[control.Key]; !exists {
				controls = append(controls, control)
				seen[control.Key] = struct{}{}
			}
		}
		for _, child := range elem.ChildElements() {
			walk(child)
		}
	}
	walk(root)
	return controls
}

func buildInteractableControl(elem *etree.Element) (interactableControl, bool) {
	if elem == nil {
		return interactableControl{}, false
	}
	actions := make([]string, 0, 4)
	if attrBool(elem, "clickable") {
		actions = append(actions, "CLICK")
	}
	if attrBool(elem, "long-clickable") {
		actions = append(actions, "LONG_CLICK")
	}
	if attrBool(elem, "editable") || strings.Contains(strings.ToLower(elem.SelectAttrValue("class", "")), "edittext") {
		actions = append(actions, "INPUT")
		if !containsString(actions, "CLICK") {
			actions = append(actions, "CLICK")
		}
	}
	if attrBool(elem, "scrollable") {
		actions = append(actions, "SCROLL")
	}
	if len(actions) == 0 {
		return interactableControl{}, false
	}
	label := strings.TrimSpace(firstNonEmpty(
		elem.SelectAttrValue("text", ""),
		elem.SelectAttrValue("content-desc", ""),
		elem.SelectAttrValue("resource-id", ""),
		elem.Tag,
	))
	className := strings.TrimSpace(elem.SelectAttrValue("class", ""))
	if className == "" {
		className = elem.Tag
	}
	bounds := strings.TrimSpace(elem.SelectAttrValue("bounds", ""))
	key := strings.Join([]string{label, className, bounds}, "|")
	return interactableControl{
		Key:         key,
		Label:       label,
		ControlType: className,
		Bounds:      bounds,
		Path:        elem.GetPath(),
		XPath:       buildAbsoluteXPath(elem),
		Actions:     dedupeStrings(actions),
	}, true
}

func buildAbsoluteXPath(node *etree.Element) string {
	if node == nil {
		return ""
	}
	segments := make([]string, 0, 8)
	for current := node; current != nil; current = current.Parent() {
		if strings.TrimSpace(current.Tag) == "" {
			break
		}
		index := 1
		if parent := current.Parent(); parent != nil {
			seen := 0
			for _, sibling := range parent.ChildElements() {
				if sibling.Tag == current.Tag {
					seen++
				}
				if sibling == current {
					index = seen
					break
				}
			}
		}
		segments = append(segments, current.Tag+"["+strconv.Itoa(index)+"]")
	}
	for i, j := 0, len(segments)-1; i < j; i, j = i+1, j-1 {
		segments[i], segments[j] = segments[j], segments[i]
	}
	return "/" + strings.Join(segments, "/")
}

func attrBool(elem *etree.Element, name string) bool {
	if elem == nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(elem.SelectAttrValue(name, "")), "true")
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func dedupeStrings(items []string) []string {
	seen := make(map[string]struct{}, len(items))
	result := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		result = append(result, item)
	}
	return result
}

func controlSupportsAction(control interactableControl, action string) bool {
	for _, candidate := range control.Actions {
		if candidate == action {
			return true
		}
		if candidate == "SCROLL" && strings.HasPrefix(action, "SCROLL_") {
			return true
		}
	}
	return false
}

func extractWidgetLocator(widgetInfo string) (xpath string, path string) {
	text := strings.TrimSpace(widgetInfo)
	if text == "" {
		return "", ""
	}
	if match := widgetXPathRegex.FindStringSubmatch(text); len(match) >= 2 {
		xpath = strings.TrimSpace(match[1])
	}
	if match := widgetPathRegex.FindStringSubmatch(text); len(match) >= 2 {
		path = strings.TrimSpace(match[1])
	}
	return xpath, path
}

func sameBounds(left string, right string) bool {
	return normalizeBoundsString(left) != "" && normalizeBoundsString(left) == normalizeBoundsString(right)
}

func normalizeBoundsString(value string) string {
	text := strings.TrimSpace(value)
	if text == "" {
		return ""
	}
	replacer := strings.NewReplacer(" ", "", "][", ",", "[", "", "]", "")
	return replacer.Replace(text)
}

func sortedKeys(set map[string]struct{}) []string {
	if len(set) == 0 {
		return nil
	}
	items := make([]string, 0, len(set))
	for key := range set {
		items = append(items, key)
	}
	slices.Sort(items)
	return items
}

func sumCountMap(counts map[string]int) int {
	total := 0
	for _, value := range counts {
		total += value
	}
	return total
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func formatCountMapInline(counts map[string]int) string {
	top := topPairs(counts, defaultTopCount)
	if len(top) == 0 {
		return "-"
	}
	parts := make([]string, 0, len(top))
	for _, item := range top {
		parts = append(parts, fmt.Sprintf("%s %d 次", item.Name, item.Count))
	}
	return strings.Join(parts, "，")
}

func escapeMarkdownCell(text string) string {
	if strings.TrimSpace(text) == "" {
		return "-"
	}
	replacer := strings.NewReplacer("|", "\\|", "\n", "<br/>", "\r", "")
	return replacer.Replace(text)
}

func writePageControlsDetail(rootDir string, pageDirName string, page PageSummary) (string, error) {
	outputPath := filepath.Join(rootDir, pageDirName, "controls.md")
	var b strings.Builder
	b.WriteString("# 页面控件明细\n\n")
	b.WriteString(fmt.Sprintf("- 页面名：%s\n", fallbackText(page.PageName, "UnknownPage")))
	b.WriteString(fmt.Sprintf("- 出现次数：%d\n", page.VisitCount))
	b.WriteString(fmt.Sprintf("- 执行操作次数：%d\n", page.ActionCount))
	b.WriteString(fmt.Sprintf("- 可交互控件数：%d\n\n", page.InteractableControlCount))
	b.WriteString("| 文本/描述 | 类型 | 坐标 | 可执行动作 | 出现次数 | 实际执行 |\n")
	b.WriteString("|---|---|---|---|---:|---|\n")
	controls := page.Controls
	if len(controls) == 0 {
		b.WriteString("| - | - | - | - | 0 | - |\n")
	} else {
		for _, control := range controls {
			b.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %d | %s |\n",
				escapeMarkdownCell(fallbackText(control.Label, "-")),
				escapeMarkdownCell(fallbackText(control.ControlType, "-")),
				escapeMarkdownCell(fallbackText(control.Bounds, "-")),
				escapeMarkdownCell(strings.Join(control.Actions, ", ")),
				control.SeenCount,
				escapeMarkdownCell(formatCountMapInline(control.ExecutedBy)),
			))
		}
	}
	if err := os.WriteFile(outputPath, []byte(b.String()), 0644); err != nil {
		return "", fmt.Errorf("写入控件明细失败(%s): %w", outputPath, err)
	}
	return filepath.ToSlash(outputPath), nil
}
