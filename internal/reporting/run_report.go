package reporting

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/beevik/etree"

	"trek/internal/engine/core/types"
	"trek/logger"
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
	BlockDetectionCount         int               `json:"block_detection_count"`
	BlockReasonCount            map[string]int    `json:"block_reason_count,omitempty"`
}

type pair struct {
	Name  string
	Count int
}

// ArtifactSummary 描述本次导出的原始页面产物目录。
type ArtifactSummary struct {
	RootDir          string `json:"root_dir,omitempty"`
	PageCount        int    `json:"page_count,omitempty"`
	FileCount        int    `json:"file_count,omitempty"`
	ScreenshotCount  int    `json:"screenshot_count,omitempty"`
	XMLCount         int    `json:"xml_count,omitempty"`
	StepTimelineFile string `json:"step_timeline_file,omitempty"`
}

// PageSummary 表示页面级人工复盘摘要。
type PageSummary struct {
	PageName                 string           `json:"page_name"`
	Label                    string           `json:"label,omitempty"`
	VisitCount               int              `json:"visit_count"`
	ActionCount              int              `json:"action_count"`
	InteractableControlCount int              `json:"interactable_control_count"`
	BlockDetectionSteps      []int            `json:"block_detection_steps,omitempty"`
	ArtifactDir              string           `json:"artifact_dir,omitempty"`
	RepresentativeScreenshot string           `json:"representative_screenshot,omitempty"`
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
	content = sanitizeUTF8(content)
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
	if strings.TrimSpace(artifactDir) == "" {
		envelope.Pages = buildPageSummaries(envelope.StepRecords)
		return envelope, nil
	}
	// 先从 XML 构建页面摘要，再写产物并释放 XML 内存
	summary, records, pages, err := writeArtifacts(artifactDir, envelope.StepRecords)
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
			BlockDetectionCount:         report.BlockDetectionCount,
			BlockReasonCount:            cloneCountMap(report.BlockReasonCount),
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

	// ── 标题 & 运行概览 ──
	b.WriteString("# Trek 运行报告\n\n")

	stopReasonText := string(summary.StopReason)
	if summary.StopReason == "context_canceled" {
		stopReasonText = "context_canceled (手动中断)"
	}
	b.WriteString("| 项目 | 值 |\n|---|---|\n")
	b.WriteString(fmt.Sprintf("| 包名 | `%s` |\n", fallbackText(meta.PackageName, "-")))
	if s := strings.TrimSpace(meta.DeviceSerial); s != "" {
		b.WriteString(fmt.Sprintf("| 设备 | %s |\n", s))
	}
	b.WriteString(fmt.Sprintf("| 算法 | %s |\n", fallbackText(meta.Algorithm, "-")))
	b.WriteString(fmt.Sprintf("| 停止原因 | %s |\n", stopReasonText))
	b.WriteString(fmt.Sprintf("| 运行时长 | %s |\n", formatDuration(summary.DurationMs)))
	b.WriteString(fmt.Sprintf("| 步数 | %d / %d（成功 %d / 失败 %d） |\n", summary.StepsTotal, summary.StepsPlanned, summary.StepsSucceeded, summary.StepsFailed))
	b.WriteString(fmt.Sprintf("| 页面源 | %s |\n", fallbackText(meta.PageSourceType, "-")))
	b.WriteString(fmt.Sprintf("| 控件策略 | %s |\n", fallbackText(meta.PageControlStrategy, "raw")))
	if strings.TrimSpace(meta.ConfigPath) != "" {
		b.WriteString(fmt.Sprintf("| 配置文件 | `%s` |\n", filepath.Base(meta.ConfigPath)))
	}
	if envelope.Artifacts != nil {
		b.WriteString(fmt.Sprintf("| 产物目录 | `%s` |\n", fallbackText(envelope.Artifacts.RootDir, "-")))
	}
	b.WriteString(fmt.Sprintf("| 生成时间 | %s |\n", formatTime(envelope.GeneratedAt)))
	b.WriteString("\n")

	// ── 前置检查（仅在异常时展示） ──
	if envelope.Preflight != nil {
		allReady := envelope.Preflight.ADBReady && envelope.Preflight.DeviceReady && envelope.Preflight.PageSourceReady
		if !allReady || summary.PreflightError != "" {
			b.WriteString("## 前置检查\n\n")
			if summary.PreflightError != "" {
				b.WriteString(fmt.Sprintf("> **错误**: %s\n\n", summary.PreflightError))
			}
			b.WriteString(fmt.Sprintf("- ADB: %s | 设备: %s | 页面源: %s | UIA: %s\n",
				readyIcon(envelope.Preflight.ADBReady),
				readyIcon(envelope.Preflight.DeviceReady),
				readyIcon(envelope.Preflight.PageSourceReady),
				readyIcon(envelope.Preflight.UIAReady),
			))
			b.WriteString("\n")
		}
	}

	// ── 关键指标 ──
	if summary.RecoveryCooldownEnterCount > 0 || summary.OutOfAppRecoveries > 0 || summary.CandidateEnhancementCalls > 0 || summary.RecoveryLLMCalls > 0 || summary.BlockDetectionCount > 0 {
		b.WriteString("## 关键指标\n\n")
		if summary.BlockDetectionCount > 0 {
			b.WriteString(fmt.Sprintf("- 页面阻塞: **%d** 次", summary.BlockDetectionCount))
			if len(summary.BlockReasonCount) > 0 {
				reasonParts := make([]string, 0, len(summary.BlockReasonCount))
				for reason, count := range summary.BlockReasonCount {
					reasonParts = append(reasonParts, fmt.Sprintf("%s %d", reason, count))
				}
				b.WriteString(fmt.Sprintf("（%s）", strings.Join(reasonParts, "、")))
			}
			b.WriteString("\n")
		}
		if summary.OutOfAppRecoveries > 0 {
			b.WriteString(fmt.Sprintf("- 离开应用恢复: **%d** 次\n", summary.OutOfAppRecoveries))
		}
		if summary.RecoveryCooldownEnterCount > 0 {
			b.WriteString(fmt.Sprintf("- 恢复冷却: 进入 **%d** 次，命中 **%d** 步\n", summary.RecoveryCooldownEnterCount, summary.RecoveryCooldownStepCount))
		}
		if summary.CandidateEnhancementCalls > 0 {
			b.WriteString(fmt.Sprintf("- 候选增强: 调用 %d / 命中 %d\n", summary.CandidateEnhancementCalls, summary.CandidateEnhancementSelects))
		}
		if summary.RecoveryLLMCalls > 0 {
			b.WriteString(fmt.Sprintf("- 恢复 LLM: 调用 %d", summary.RecoveryLLMCalls))
			if summary.RecoveryLLMBudgetDenied > 0 {
				b.WriteString(fmt.Sprintf("（预算拒绝 %d）", summary.RecoveryLLMBudgetDenied))
			}
			b.WriteString("\n")
		}
		if summary.EnhancementLLMBudgetDenied > 0 {
			b.WriteString(fmt.Sprintf("- 增强 LLM 预算拒绝: %d\n", summary.EnhancementLLMBudgetDenied))
		}
		b.WriteString("\n")
	}

	// ── 产物导出 ──
	if envelope.Artifacts != nil && envelope.Artifacts.FileCount > 0 {
		b.WriteString("## 产物导出\n\n")
		b.WriteString(fmt.Sprintf("- %d 个页面目录，共 %d 个文件（截图 %d / XML %d）\n",
			envelope.Artifacts.PageCount, envelope.Artifacts.FileCount, envelope.Artifacts.ScreenshotCount, envelope.Artifacts.XMLCount))
		if envelope.Artifacts.StepTimelineFile != "" {
			b.WriteString(fmt.Sprintf("- [步骤时间线](%s)\n", envelope.Artifacts.StepTimelineFile))
		}
		b.WriteString("\n")
	}

	// ── 页面索引 ──
	pageIDMap := buildPageIDMap(envelope.Pages)
	if len(envelope.Pages) > 0 {
		b.WriteString("## 页面索引\n\n")

		// 图片网格：每行 5 张，可横向滚动
		hasImages := false
		for _, page := range envelope.Pages {
			if page.RepresentativeScreenshot != "" {
				hasImages = true
				break
			}
		}
		if hasImages {
			b.WriteString("<div style=\"overflow-x:auto; margin-bottom:16px;\">\n")
			b.WriteString("<table style=\"border-collapse:collapse;\">\n")
			perPageRow := 5
			for i := 0; i < len(envelope.Pages); i += perPageRow {
				end := i + perPageRow
				if end > len(envelope.Pages) {
					end = len(envelope.Pages)
				}
				// 图片行
				b.WriteString("<tr>\n")
				for _, page := range envelope.Pages[i:end] {
					id := pageIDMap[page.PageName]
					b.WriteString("<td style=\"padding:8px; text-align:center; vertical-align:top; width:180px;\">\n")
					b.WriteString(fmt.Sprintf("<a href=\"#page-%s\">\n", id))
					if page.RepresentativeScreenshot != "" {
						b.WriteString(fmt.Sprintf("<img src=\"%s\" style=\"max-width:160px; max-height:280px; border-radius:6px; border:1px solid #333;\" />\n", page.RepresentativeScreenshot))
					} else {
						b.WriteString("<div style=\"width:160px; height:280px; background:#1a1a2e; border-radius:6px; border:1px solid #333; display:flex; align-items:center; justify-content:center; color:#666; font-size:12px;\">无截图</div>\n")
					}
					b.WriteString("</a>\n</td>\n")
				}
				// 补齐空位
				for j := len(envelope.Pages[i:end]); j < perPageRow; j++ {
					b.WriteString("<td style=\"padding:8px; width:180px;\"></td>\n")
				}
				b.WriteString("</tr>\n")
				// 信息行
				b.WriteString("<tr>\n")
				for _, page := range envelope.Pages[i:end] {
					id := pageIDMap[page.PageName]
					label := escapeMarkdownCell(truncatePageName(page.PageName, 30))
					if page.Label != "" {
						label = escapeMarkdownCell(truncatePageName(page.Label, 20))
					}
					blockInfo := ""
					if len(page.BlockDetectionSteps) > 0 {
						blockInfo = fmt.Sprintf(" / <span style=\"color:#f59e0b;\">⚠ %d 阻塞</span>", len(page.BlockDetectionSteps))
					}
					b.WriteString(fmt.Sprintf("<td style=\"padding:4px 8px; text-align:center; vertical-align:top; font-size:13px;\">\n<a href=\"#page-%s\" style=\"color:inherit; text-decoration:none;\"><b>P%s</b> %s</a><br/><span style=\"color:#888; font-size:11px;\">%d 访问 / %d 操作%s</span>\n</td>\n",
						id, id, label, page.VisitCount, page.ActionCount, blockInfo))
				}
				for j := len(envelope.Pages[i:end]); j < perPageRow; j++ {
					b.WriteString("<td style=\"padding:4px 8px; width:180px;\"></td>\n")
				}
				b.WriteString("</tr>\n")
			}
			b.WriteString("</table>\n")
			b.WriteString("</div>\n\n")
		}
	}

	// ── 高频动作 ──
	if len(summary.ActionCount) > 0 {
		b.WriteString("## 动作统计\n\n")
		top := topPairs(summary.ActionCount, defaultTopCount)
		for _, item := range top {
			b.WriteString(fmt.Sprintf("- **%s**: %d 次\n", item.Name, item.Count))
		}
		b.WriteString("\n")
	}

	// ── 问题 & 警告 ──
	writeIssuesSection(&b, envelope.StepRecords)

	// ── 页面详情 ──
	writePageDetailsCompact(&b, envelope.Pages, pageIDMap)

	return b.String()
}

// renderStepTimeline 将步骤时间线渲染为独立 MD 内容，每步显示前后截图，10步一个章节。
func renderStepTimeline(records []monkey.StepRecord, pageIDMap map[string]string, targetRoot string) string {
	var b strings.Builder
	b.WriteString("# 步骤时间线\n\n")

	const stepsPerSection = 10
	for i, r := range records {
		// 每10步输出一个章节标题
		if r.Step == 1 || (r.Step-1)%stepsPerSection == 0 {
			sectionEnd := r.Step + stepsPerSection - 1
			if sectionEnd > len(records) {
				sectionEnd = len(records)
			}
			b.WriteString(fmt.Sprintf("## step %d-%d\n\n", r.Step, sectionEnd))
		}

		pageName := firstNonEmpty(r.BeforePageName, r.PageName)
		pageID := pageIDMap[pageName]
		if pageID == "" {
			pageID = "-"
		}
		action := fallbackText(r.Action, "-")
		result := "OK"
		if strings.TrimSpace(r.Err) != "" {
			result = "FAIL"
		}
		// 页面理解策略描述
		strategyDesc := "-"
		if r.ScriptTransformed {
			strategyDesc = "脚本转换"
		} else if strategy := strings.TrimSpace(r.PageControlStrategy); strategy != "" {
			if r.CacheHit {
				strategyDesc = "命中缓存"
			} else {
				switch strings.ToLower(strategy) {
				case "ocr":
					strategyDesc = "调用OCR"
				case "llm":
					strategyDesc = "调用LLM"
				case "raw":
					strategyDesc = "用户页面源"
				default:
					strategyDesc = strategy
				}
			}
		}
		// 结果文字
		resultText := result
		if result == "FAIL" {
			resultText = fmt.Sprintf("**FAIL** %s", truncateErr(r.Err, 60))
		}
		// 截图：仅点击/长按/输入类动作显示标注截图（操作前 + 标记）
		markedImg := ""
		if isTapAction(r.Action) && r.BeforeArtifactRef != nil && r.BeforeArtifactRef.ScreenshotFile != "" {
			imgSrc := r.BeforeArtifactRef.ScreenshotFile
			if marked := markedScreenshotPath(imgSrc); marked != "" {
				// 检查标注文件是否存在，不存在则用原图
				if _, err := os.Stat(filepath.Join(targetRoot, marked)); err == nil {
					imgSrc = marked
				}
			}
			fullPath := filepath.Join(targetRoot, imgSrc)
			style := imageStyleByAspect(fullPath)
			markedImg = fmt.Sprintf("<div style=\"padding:4px 10px; text-align:center;\"><img src=\"%s\" style=\"%s\" /></div>\n", imgSrc, style)
		}
		// 输出
		b.WriteString(fmt.Sprintf("### step%d\n", r.Step))
		if markedImg != "" {
			b.WriteString(markedImg)
		}
		b.WriteString(fmt.Sprintf("页面：P%s\n\n", pageID))
		b.WriteString(fmt.Sprintf("操作：%s\n\n", escapeMarkdownCell(action)))
		if r.TapPoint != "" {
			b.WriteString(fmt.Sprintf("触控坐标：%s\n\n", r.TapPoint))
		}
		b.WriteString(fmt.Sprintf("结果：%s\n\n", resultText))
		b.WriteString(fmt.Sprintf("页面理解：%s\n\n", strategyDesc))
		if r.BlockDetected {
			b.WriteString(fmt.Sprintf("⚠️ 阻塞检测：%s\n\n", r.BlockReason))
		}
		b.WriteString(fmt.Sprintf("耗时：%s\n", formatDuration(r.DurationMs)))
		// 章节之间空一行（除了最后一个）
		if i < len(records)-1 && (r.Step)%stepsPerSection == 0 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

// imageStyleByAspect 根据图片宽高比返回 img 样式。
// 如果图片宽小于高（竖屏）: width=100%, max-height=200px
// 如果图片高小于宽（横屏）: height=100%, max-width=200px
func imageStyleByAspect(imgPath string) string {
	file, err := os.Open(imgPath)
	if err != nil {
		return "max-height:100px; max-width:100%; object-fit:contain; border:1px solid #444; border-radius:3px; vertical-align:middle;"
	}
	defer file.Close()
	cfg, _, err := image.DecodeConfig(file)
	if err != nil {
		return "max-height:100px; max-width:100%; object-fit:contain; border:1px solid #444; border-radius:3px; vertical-align:middle;"
	}
	if cfg.Width < cfg.Height {
		// 竖屏：宽100%，高最多200px
		return "max-height:300px; width:100%; object-fit:contain; border:1px solid #444; border-radius:3px; vertical-align:middle;"
	}
	// 横屏：高100%，宽最多200px
	return "max-width:200px; height:100%; object-fit:contain; border:1px solid #444; border-radius:3px; vertical-align:middle;"
}

func writeIssuesSection(b *strings.Builder, records []monkey.StepRecord) {
	hasIssues := false
	for _, r := range records {
		if strings.TrimSpace(r.Err) != "" {
			hasIssues = true
			break
		}
	}
	if !hasIssues {
		return
	}
	b.WriteString("## 问题 & 警告\n\n")
	failures := make([]monkey.StepRecord, 0)
	for _, r := range records {
		if strings.TrimSpace(r.Err) != "" {
			failures = append(failures, r)
		}
	}
	for _, r := range failures {
		pageName := firstNonEmpty(r.BeforePageName, r.PageName)
		b.WriteString(fmt.Sprintf("- **Step %d** `%s` %s: %s\n",
			r.Step,
			escapeMarkdownCell(truncatePageName(pageName, 40)),
			fallbackText(r.Action, "-"),
			r.Err,
		))
	}
	b.WriteString("\n")
}

func writePageDetailsCompact(b *strings.Builder, pages []PageSummary, pageIDMap map[string]string) {
	if len(pages) == 0 {
		return
	}
	b.WriteString("## 页面详情\n\n")
	for _, page := range pages {
		id := pageIDMap[page.PageName]
		header := escapeMarkdownCell(truncatePageName(page.PageName, 80))
		if page.Label != "" {
			header = fmt.Sprintf("%s (%s)", escapeMarkdownCell(truncatePageName(page.Label, 40)), header)
		}
		b.WriteString(fmt.Sprintf("<a id=\"page-%s\"></a>\n\n### P%s — %s\n\n", id, id, header))
		if page.RepresentativeScreenshot != "" {
			b.WriteString(fmt.Sprintf("<img src=\"%s\" style=\"max-width:240px; max-height:420px; border-radius:6px; border:1px solid #333; margin-bottom:12px;\" />\n\n", page.RepresentativeScreenshot))
		}
		if page.ArtifactDir != "" {
			b.WriteString(fmt.Sprintf("- 产物: `%s`\n", page.ArtifactDir))
		}
		if len(page.TopActions) > 0 {
			b.WriteString(fmt.Sprintf("- 动作: %s\n", formatCountMapInline(page.TopActions)))
		}
		if len(page.BlockDetectionSteps) > 0 {
			stepList := formatIntList(page.BlockDetectionSteps)
			b.WriteString(fmt.Sprintf("- ⚠️ 阻塞检测: %d 次 (步骤 %s)\n", len(page.BlockDetectionSteps), stepList))
		}
		if len(page.TopControls) > 0 {
			b.WriteString("\n| 文本/描述 | 类型 | 动作 | 出现 | 执行 |\n")
			b.WriteString("|---|---|---|---:|---|\n")
			for _, c := range page.TopControls {
				b.WriteString(fmt.Sprintf("| %s | %s | %s | %d | %s |\n",
					escapeMarkdownCell(fallbackText(c.Label, "-")),
					escapeMarkdownCell(fallbackText(c.ControlType, "-")),
					escapeMarkdownCell(strings.Join(c.Actions, ", ")),
					c.SeenCount,
					escapeMarkdownCell(formatCountMapInline(c.ExecutedBy)),
				))
			}
		}
		b.WriteString("\n")
	}
}

// formatDuration 将毫秒转换为人类可读的时长格式。
func formatDuration(ms int64) string {
	if ms <= 0 {
		return "0s"
	}
	d := time.Duration(ms) * time.Millisecond
	if d < time.Second {
		return fmt.Sprintf("%dms", ms)
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", float64(ms)/1000)
	}
	minutes := int(d.Minutes())
	seconds := int(d.Seconds()) % 60
	if minutes < 60 {
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}
	hours := minutes / 60
	minutes = minutes % 60
	return fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)
}

func readyIcon(ready bool) string {
	if ready {
		return "OK"
	}
	return "MISSING"
}

func buildPageIDMap(pages []PageSummary) map[string]string {
	m := make(map[string]string, len(pages))
	for i, page := range pages {
		m[page.PageName] = strconv.Itoa(i + 1)
	}
	return m
}

// shortPageDirName 根据 pageIDMap 返回简短的页面目录名 "P1"、"P2" 等。
// 如果 pageName 不在映射中，回退到 sanitizePageDirName。
func shortPageDirName(pageName string, pageIDMap map[string]string) string {
	id, ok := pageIDMap[pageName]
	if !ok {
		dirName := sanitizePageDirName(pageName)
		if dirName == "" {
			return "UnknownPage"
		}
		return dirName
	}
	return "P" + id
}

func truncatePageName(name string, maxLen int) string {
	trimmed := strings.TrimSpace(name)
	if len(trimmed) <= maxLen {
		return trimmed
	}
	if maxLen <= 3 {
		return trimmed[:maxLen]
	}
	return trimmed[:maxLen-3] + "..."
}

func truncateErr(err string, maxLen int) string {
	trimmed := strings.TrimSpace(err)
	if len(trimmed) <= maxLen {
		return trimmed
	}
	if maxLen <= 3 {
		return trimmed[:maxLen]
	}
	return trimmed[:maxLen-3] + "..."
}

func isTapAction(action string) bool {
	return true // 所有步骤都显示截图
}

// buildPageLabel 从控件文本中提取人类可读的页面标签。
func buildPageLabel(controls []ControlSummary) string {
	var texts []string
	for _, c := range controls {
		label := strings.TrimSpace(c.Label)
		if label == "" || label == "<empty>" {
			continue
		}
		texts = append(texts, label)
	}
	if len(texts) == 0 {
		return ""
	}
	// 去重并限制数量
	seen := make(map[string]struct{}, len(texts))
	unique := texts[:0]
	for _, t := range texts {
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		unique = append(unique, t)
		if len(unique) >= 4 {
			break
		}
	}
	return strings.Join(unique, ", ")
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

func writeArtifacts(rootDir string, records []monkey.StepRecord) (*ArtifactSummary, []monkey.StepRecord, []PageSummary, error) {
	targetRoot := strings.TrimSpace(rootDir)
	if targetRoot == "" {
		return nil, records, nil, nil
	}
	if err := os.MkdirAll(targetRoot, 0755); err != nil {
		return nil, nil, nil, fmt.Errorf("创建产物目录失败: %w", err)
	}

	cloned := append([]monkey.StepRecord(nil), records...)

	// 在写产物之前先从 XML 构建页面摘要（XML 稍后会被释放）
	pages := buildPageSummaries(cloned)
	pageIDMap := buildPageIDMap(pages)

	// 实时写入阶段已按 P1/P2 目录写入截图+XML，不再重新写入。
	// 直接统计产物目录并写入控件明细和时间线。
	pageDirs := make(map[string]struct{})
	for _, p := range pages {
		dirName := shortPageDirName(p.PageName, pageIDMap)
		if dirName != "" {
			pageDirs[dirName] = struct{}{}
		}
	}

	// 写入每步的 XML/截图产物（截图在实时写入阶段已落盘，此处仅写 XML；
	// 若未经过实时写入，则直接从步记录写出）
	for index := range cloned {
		record := &cloned[index]
		beforeDirName := shortPageDirName(record.BeforePageName, pageIDMap)
		if beforeDirName != "" {
			existing := record.BeforeArtifactRef
			newRef := writeStepArtifact(targetRoot, *record, "before", beforeDirName, record.BeforeXML, record.BeforeScreenshot)
			if newRef != nil {
				// 保留实时路径中已有的截图路径（writeStepArtifact 拿不到 freed 的截图数据）
				if existing != nil && existing.ScreenshotFile != "" && newRef.ScreenshotFile == "" {
					newRef.ScreenshotFile = existing.ScreenshotFile
				}
				record.BeforeArtifactRef = newRef
			} else if existing != nil {
				record.BeforeArtifactRef = existing
			}
		}
		// 只有最后一步才写 After 产物
		if index == len(cloned)-1 {
			afterDirName := shortPageDirName(record.AfterPageName, pageIDMap)
			if afterDirName != "" {
				existing := record.AfterArtifactRef
				newRef := writeStepArtifact(targetRoot, *record, "after", afterDirName, record.AfterXML, record.AfterScreenshot)
				if newRef != nil {
					if existing != nil && existing.ScreenshotFile != "" && newRef.ScreenshotFile == "" {
						newRef.ScreenshotFile = existing.ScreenshotFile
					}
					record.AfterArtifactRef = newRef
				} else if existing != nil {
					record.AfterArtifactRef = existing
				}
			}
		}
	}

	// 为每个页面填充产物目录并写入控件明细
	for index := range pages {
		page := &pages[index]
		dirName := shortPageDirName(page.PageName, pageIDMap)
		if dirName == "" {
			dirName = "UnknownPage"
		}
		page.ArtifactDir = filepath.ToSlash(filepath.Join(targetRoot, dirName))
		page.RepresentativeScreenshot = findFirstScreenshot(targetRoot, dirName, filepath.Base(targetRoot))
		detailPath, err := writePageControlsDetail(targetRoot, dirName, *page)
		if err != nil {
			return nil, nil, nil, err
		}
		page.ControlsDetailFile = detailPath
		pageDirs[dirName] = struct{}{}
	}

	// 从磁盘统计产物文件数（截图由实时写入阶段已写到 P[N] 目录）
	screenshotCount, xmlCount := countDiskArtifactFiles(targetRoot, pageDirs)

	// 释放 XML 内存
	for i := range cloned {
		cloned[i].BeforeXML = ""
		cloned[i].AfterXML = ""
		cloned[i].BeforeScreenshot = nil
		cloned[i].AfterScreenshot = nil
	}

	// 写入步骤时间线独立文件
	var stepTimelineFile string
	if len(cloned) > 0 {
		pageIDMap := buildPageIDMap(pages)
		timelineContent := renderStepTimeline(cloned, pageIDMap, targetRoot)
		timelinePath := filepath.Join(targetRoot, "step-timeline.md")
		if err := os.WriteFile(timelinePath, []byte(timelineContent), 0644); err != nil {
			return nil, nil, nil, fmt.Errorf("写入步骤时间线失败: %w", err)
		}
		stepTimelineFile = filepath.ToSlash(timelinePath)
	}

	return &ArtifactSummary{
		RootDir:          targetRoot,
		PageCount:        len(pageDirs),
		FileCount:        screenshotCount + xmlCount,
		ScreenshotCount:  screenshotCount,
		XMLCount:         xmlCount,
		StepTimelineFile: stepTimelineFile,
	}, cloned, pages, nil
}

// countDiskArtifactFiles 从磁盘统计产物文件数。
func countDiskArtifactFiles(rootDir string, pageDirs map[string]struct{}) (screenshotCount, xmlCount int) {
	for dirName := range pageDirs {
		dirPath := filepath.Join(rootDir, dirName)
		entries, err := os.ReadDir(dirPath)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			ext := strings.ToLower(filepath.Ext(entry.Name()))
			switch ext {
			case ".png", ".jpg", ".jpeg":
				screenshotCount++
			case ".xml":
				xmlCount++
			}
		}
	}
	return
}

// writeStepArtifact 从步记录写入单步截图和 XML 产物（若数据为空则跳过）。
func writeStepArtifact(rootDir string, record monkey.StepRecord, phase string, pageDirName string, xmlText string, screenshot []byte) *monkey.StepArtifactRef {
	pageDirPath := filepath.Join(rootDir, pageDirName)
	if err := os.MkdirAll(pageDirPath, 0755); err != nil {
		return nil
	}
	ref := &monkey.StepArtifactRef{PageDir: pageDirName}
	prefix := "step-" + strconv.FormatInt(int64(record.Step), 10) + "-" + phase
	if action := strings.TrimSpace(record.Action); action != "" {
		prefix += "-" + sanitizePageDirName(action)
	}
	if len(screenshot) > 0 {
		ext := ".png"
		fileName := prefix + ext
		if err := os.WriteFile(filepath.Join(pageDirPath, fileName), screenshot, 0644); err == nil {
			ref.ScreenshotFile = filepath.ToSlash(filepath.Join(pageDirName, fileName))
		}
	}
	if strings.TrimSpace(xmlText) != "" {
		fileName := prefix + ".xml"
		if err := os.WriteFile(filepath.Join(pageDirPath, fileName), []byte(xmlText), 0644); err == nil {
			ref.XMLFile = filepath.ToSlash(filepath.Join(pageDirName, fileName))
		}
	}
	if ref.ScreenshotFile == "" && ref.XMLFile == "" {
		return nil
	}
	return ref
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

// findFirstScreenshot 在页面产物目录中找到代表截图。
// 优先级：original.png > 第一个 before 截图 > 第一个 after 截图。
// artifactDirName 是产物目录相对于 md 报告文件的路径前缀（如 "run-report_artifacts"）。
func findFirstScreenshot(rootDir string, pageDirName string, artifactDirName string) string {
	dirPath := filepath.Join(rootDir, pageDirName)
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return ""
	}
	// 优先使用 original.png（页面首次截图，最能代表该页面）
	origPath := filepath.ToSlash(filepath.Join(artifactDirName, pageDirName, "original.png"))
	if _, err := os.Stat(filepath.Join(dirPath, "original.png")); err == nil {
		return origPath
	}
	var fallback string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, "step-") {
			continue
		}
		ext := strings.ToLower(filepath.Ext(name))
		if ext != ".png" && ext != ".jpg" && ext != ".jpeg" {
			continue
		}
		if strings.Contains(name, "-before") {
			return filepath.ToSlash(filepath.Join(artifactDirName, pageDirName, name))
		}
		if fallback == "" && strings.Contains(name, "-after") {
			fallback = filepath.ToSlash(filepath.Join(artifactDirName, pageDirName, name))
		}
	}
	return fallback
}

type pageAggregate struct {
	PageName            string
	VisitCount          int
	ActionCount         int
	Actions             map[string]int
	Controls            map[string]*controlAggregate
	BlockDetectionSteps []int
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
	Element     types.IElement
}

func buildPageSummaries(records []monkey.StepRecord) []PageSummary {
	pageMap := make(map[string]*pageAggregate)
	for i, record := range records {
		pageName := strings.TrimSpace(firstNonEmpty(record.BeforePageName, record.PageName))
		logger.Debugf("report buildPageSummaries step=%d pageName=%s action=%s widgetInfo=%s targetBounds=%s",
			i+1, pageName, record.Action, record.ActionWidgetInfo, record.ActionTargetBounds)
		if pageName != "" {
			page := ensurePageAggregate(pageMap, pageName)
			page.VisitCount++
			if action := strings.TrimSpace(record.Action); action != "" {
				page.ActionCount++
				page.Actions[action]++
			}
			if record.BlockDetected {
				page.BlockDetectionSteps = append(page.BlockDetectionSteps, record.Step)
			}
			// 优先使用 IElement，为空时回退到 XML 解析
			var xmlControls []interactableControl
			if record.BeforeElement != nil {
				xmlControls = extractControlsFromElement(record.BeforeElement)
			} else {
				xmlControls = extractInteractableControls(record.BeforeXML)
			}
			logger.Debugf("report buildPageSummaries step=%d beforeXML controls=%d", i+1, len(xmlControls))
			mergePageControls(page, xmlControls)
			markExecutedControl(page, record.Action, record.BeforeElement, record.BeforeXML, record.ActionWidgetInfo, record.ActionTargetBounds)
		}
		// After 页面发现：每步都处理，但不提取控件（下一步的 Before 会覆盖）
		afterPageName := strings.TrimSpace(record.AfterPageName)
		if afterPageName != "" && afterPageName != pageName {
			ensurePageAggregate(pageMap, afterPageName)
		}
	}
	// 最后一步的 After 控件单独提取（没有下一步来覆盖）
	if len(records) > 0 {
		last := records[len(records)-1]
		afterPageName := strings.TrimSpace(last.AfterPageName)
		beforePageName := strings.TrimSpace(firstNonEmpty(last.BeforePageName, last.PageName))
		if afterPageName != "" && afterPageName != beforePageName {
			page := ensurePageAggregate(pageMap, afterPageName)
			page.VisitCount++
			if last.AfterElement != nil {
				mergePageControls(page, extractControlsFromElement(last.AfterElement))
			} else {
				mergePageControls(page, extractInteractableControls(last.AfterXML))
			}
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
			Label:                    buildPageLabel(topControls),
			VisitCount:               page.VisitCount,
			ActionCount:              page.ActionCount,
			InteractableControlCount: len(topControls),
			BlockDetectionSteps:      page.BlockDetectionSteps,
			TopActions:               cloneCountMap(page.Actions),
			Controls:                 append([]ControlSummary(nil), topControls...),
			TopControls:              append([]ControlSummary(nil), topControls...),
		}
		if len(summary.TopControls) > defaultPageControlLimit {
			summary.TopControls = summary.TopControls[:defaultPageControlLimit]
		}
		pages = append(pages, summary)
	}
	// 按时序（首次出现）排序，而非按访问次数
	firstOrder := buildFirstEncounterOrder(records)
	slices.SortFunc(pages, func(a, b PageSummary) int {
		aOrder := firstOrder[a.PageName]
		bOrder := firstOrder[b.PageName]
		if aOrder != bOrder {
			return aOrder - bOrder
		}
		return strings.Compare(a.PageName, b.PageName)
	})
	return pages
}
func buildFirstEncounterOrder(records []monkey.StepRecord) map[string]int {
	order := make(map[string]int)
	next := 1
	for _, r := range records {
		if beforeName := strings.TrimSpace(r.BeforePageName); beforeName != "" {
			if _, ok := order[beforeName]; !ok {
				order[beforeName] = next
				next++
			}
		}
		if afterName := strings.TrimSpace(r.AfterPageName); afterName != "" {
			if afterName != strings.TrimSpace(r.BeforePageName) {
				if _, ok := order[afterName]; !ok {
					order[afterName] = next
					next++
				}
			}
		}
	}
	return order
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

func markExecutedControl(page *pageAggregate, action string, element types.IElement, xmlText string, widgetInfo string, targetBounds string) {
	if page == nil {
		return
	}
	action = strings.TrimSpace(action)
	if action == "" {
		return
	}
	targetXPath, _ := extractWidgetLocator(widgetInfo)
	logger.Debugf("report markExecutedControl page=%s action=%s targetXPath=%s targetBounds=%s controls=%d",
		page.PageName, action, targetXPath, targetBounds, len(page.Controls))
	// 优先使用 IElement，为空时回退到 XML 解析
	var xmlControls []interactableControl
	if element != nil {
		xmlControls = extractControlsFromElement(element)
	} else {
		xmlControls = extractInteractableControls(xmlText)
	}
	logger.Debugf("report markExecutedControl xmlControls=%d", len(xmlControls))
	for _, control := range xmlControls {
		item, ok := page.Controls[control.Key]
		if !ok {
			logger.Debugf("report markExecutedControl controlKey=%s NOT in page.Controls (xpath=%s bounds=%s)", control.Key, control.XPath, control.Bounds)
			continue
		}
		if !controlSupportsAction(control, action) {
			logger.Debugf("report markExecutedControl controlKey=%s supportsAction=false", control.Key)
			continue
		}
		// 优先使用 bounds 匹配，因为合成 XML 中所有控件的 path 相同无区分度
		if targetBounds != "" && sameBounds(control.Bounds, targetBounds) {
			logger.Debugf("report markExecutedControl MATCHED by bounds: controlKey=%s bounds=%s targetBounds=%s", control.Key, control.Bounds, targetBounds)
			item.ExecutedBy[action]++
			return
		}
		// 其次 xpath 匹配
		if targetXPath != "" && control.XPath == targetXPath {
			logger.Debugf("report markExecutedControl MATCHED by xpath: controlKey=%s xpath=%s", control.Key, control.XPath)
			item.ExecutedBy[action]++
			return
		}
	}
	logger.Debugf("report markExecutedControl NO MATCH found for action=%s targetBounds=%s targetXPath=%s", action, targetBounds, targetXPath)
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
	// 使用 XPath 作为唯一标识符，避免相同文本/类名/bounds 的控件被合并
	xpath := buildAbsoluteXPath(elem)
	key := xpath
	if key == "" {
		key = strings.Join([]string{label, className, bounds}, "|")
	}
	return interactableControl{
		Key:         key,
		Label:       label,
		ControlType: className,
		Bounds:      bounds,
		Path:        buildAbsoluteXPath(elem),
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

// extractControlsFromElement 从 IElement 树中提取可交互控件。
func extractControlsFromElement(elem types.IElement) []interactableControl {
	if elem == nil {
		return nil
	}
	controls := make([]interactableControl, 0)
	seen := make(map[string]struct{})
	var walk func(e types.IElement)
	walk = func(e types.IElement) {
		if e == nil {
			return
		}
		if control, ok := buildInteractableControlFromElement(e); ok {
			if _, exists := seen[control.Key]; !exists {
				controls = append(controls, control)
				seen[control.Key] = struct{}{}
			}
		}
		for _, child := range e.GetChildren() {
			walk(child)
		}
	}
	walk(elem)
	return controls
}

// buildInteractableControlFromElement 从 IElement 构建 interactableControl。
func buildInteractableControlFromElement(elem types.IElement) (interactableControl, bool) {
	if elem == nil {
		return interactableControl{}, false
	}
	actions := make([]string, 0, 4)
	if elem.GetClickable() {
		actions = append(actions, "CLICK")
	}
	if elem.GetLongClickable() {
		actions = append(actions, "LONG_CLICK")
	}
	if elem.GetEditable() || strings.Contains(strings.ToLower(getClassName(elem)), "edittext") {
		actions = append(actions, "INPUT")
		if !containsString(actions, "CLICK") {
			actions = append(actions, "CLICK")
		}
	}
	if elem.GetScrollType() != types.NONE {
		actions = append(actions, "SCROLL")
	}
	if len(actions) == 0 {
		return interactableControl{}, false
	}

	label := strings.TrimSpace(firstNonEmpty(elem.GetText(), getContentDesc(elem), getResourceID(elem), getClassName(elem)))
	className := strings.TrimSpace(getClassName(elem))
	if className == "" {
		className = getClassName(elem)
	}
	bounds := ""
	if rect := elem.GetBounds(); rect != nil {
		bounds = rect.String()
	}
	xpath := elem.GetXPath()
	path := elem.GetPath()

	key := xpath
	if key == "" {
		key = strings.Join([]string{label, className, bounds}, "|")
	}

	return interactableControl{
		Key:         key,
		Label:       label,
		ControlType: className,
		Bounds:      bounds,
		Path:        path,
		XPath:       xpath,
		Actions:     dedupeStrings(actions),
		Element:     elem,
	}, true
}

func getClassName(elem types.IElement) string {
	if v := elem.GetAttr("class"); v != nil {
		return fmt.Sprintf("%v", v)
	}
	return ""
}

func getContentDesc(elem types.IElement) string {
	if v := elem.GetAttr("content-desc"); v != nil {
		return fmt.Sprintf("%v", v)
	}
	return ""
}

func getResourceID(elem types.IElement) string {
	if v := elem.GetAttr("resource-id"); v != nil {
		return fmt.Sprintf("%v", v)
	}
	return ""
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
		logger.Debugf("report extractWidgetLocator widgetInfo is empty")
		return "", ""
	}
	if match := widgetXPathRegex.FindStringSubmatch(text); len(match) >= 2 {
		xpath = strings.TrimSpace(match[1])
	}
	if match := widgetPathRegex.FindStringSubmatch(text); len(match) >= 2 {
		path = strings.TrimSpace(match[1])
	}
	logger.Debugf("report extractWidgetLocator xpath=%s path=%s (widgetInfo=%s)", xpath, path, text)
	return xpath, path
}

func sameBounds(left string, right string) bool {
	leftNorm := normalizeBoundsString(left)
	rightNorm := normalizeBoundsString(right)
	matched := leftNorm != "" && leftNorm == rightNorm
	if !matched {
		logger.Debugf("report sameBounds NO MATCH left=%q -> %q right=%q -> %q", left, leftNorm, right, rightNorm)
	}
	return matched
}

func normalizeBoundsString(value string) string {
	text := strings.TrimSpace(value)
	if text == "" {
		return ""
	}
	// 统一处理两种 bounds 格式：
	// 1. XML 格式: "[10,20][100,60]" (整数，两个括号对)
	// 2. Rect.String() 格式: "[10.000,20.000,100.000,60.000]" (浮点数，一个括号对)
	replacer := strings.NewReplacer(" ", "", "][", ",", "[", "", "]", "")
	normalized := replacer.Replace(text)
	// 移除所有浮点数的小数部分，统一为整数比较
	parts := strings.Split(normalized, ",")
	for i, part := range parts {
		if idx := strings.Index(part, "."); idx != -1 {
			parts[i] = part[:idx]
		}
	}
	return strings.Join(parts, ",")
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

// markedScreenshotPath 根据原始截图路径返回标注截图路径（去掉扩展名加 -marked.png）。
func markedScreenshotPath(screenshotFile string) string {
	ext := filepath.Ext(screenshotFile)
	base := strings.TrimSuffix(screenshotFile, ext)
	return base + "-marked.png"
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

// sanitizeUTF8 清理无效的 UTF-8 序列，替换为 Unicode 替换字符。
func sanitizeUTF8(data []byte) []byte {
	if utf8.Valid(data) {
		return data
	}
	var buf bytes.Buffer
	buf.Grow(len(data))
	for len(data) > 0 {
		r, size := utf8.DecodeRune(data)
		if r == utf8.RuneError && size <= 1 {
			buf.WriteRune(utf8.RuneError)
			data = data[1:]
		} else {
			buf.WriteRune(r)
			data = data[size:]
		}
	}
	return buf.Bytes()
}

func formatIntList(nums []int) string {
	parts := make([]string, len(nums))
	for i, n := range nums {
		parts[i] = fmt.Sprintf("%d", n)
	}
	return strings.Join(parts, ", ")
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
	pageDir := filepath.Join(rootDir, pageDirName)
	if err := os.MkdirAll(pageDir, 0755); err != nil {
		return "", fmt.Errorf("创建页面目录失败(%s): %w", pageDir, err)
	}
	outputPath := filepath.Join(pageDir, "controls.md")
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
