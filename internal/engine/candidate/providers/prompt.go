package providers

import (
	"encoding/base64"
	"fmt"
	"strings"

	enginestate "trek/internal/engine/state"
)

// RecoveryPrompt 是恢复阶段的语义提示，各 provider 将其映射到各自的请求格式。
//
// 设计参考 Midscene 的多模态消息架构：
//   - 语义提示一次构建，多 provider 复用
//   - Provider 只需将 Messages 映射到自己的 API 格式
//   - 截图通过 ImageContent 传递，Provider 决定编码方式（base64 URL / input_image / 等）
//
// OpenAI Chat Completions provider 映射：
//   SystemContent → {role: "system", content: "..."}
//   UserContent   → {role: "user", content: [{type: "text", ...}, {type: "image_url", ...}]}
//
// HTTP provider 映射：
//   SystemContent → instruction 字段
//   UserContent   → user_message 字段
//   ImageContent  → screenshot_base64 字段
//   ContextFields → context 字段
type RecoveryPrompt struct {
	// SystemContent 是系统级完整指令，包含角色定位、输出格式、坐标规则。
	SystemContent string
	// UserContent 是用户级结构化上下文描述。
	UserContent string
	// Screenshot 是当前屏幕截图的原始字节（PNG/JPEG）。
	// 为 nil 时表示无截图。Provider 根据 API 格式决定编码方式。
	Screenshot []byte
	// ScreenshotMediaType 是截图的 MIME 类型（如 "image/png"）。
	ScreenshotMediaType string
	// ContextFields 是结构化的上下文字段，供 HTTP 等 provider 按需序列化。
	ContextFields RecoveryContextFields
	// ResponseSchema 是期望的 JSON 响应 schema，供支持结构化输出的 provider 使用。
	ResponseSchema map[string]any
}

// RecoveryContextFields 是从 TraversalContext 提取的语义上下文字段，
// 供各 provider 按自身格式序列化。
type RecoveryContextFields struct {
	Step             int                        `json:"step"`
	Mode             string                     `json:"mode"`
	PageName         string                     `json:"page_name"`
	PageSignature    string                     `json:"page_signature"`
	ClusterSignature string                     `json:"cluster_signature"`
	BlockReason      string                     `json:"block_reason"`
	RecentTrace      []enginestate.ActionTrace  `json:"recent_trace,omitempty"`
	PageVisitCount   map[string]int             `json:"page_visit_count,omitempty"`
	ActionCount      map[string]int             `json:"action_count,omitempty"`
}

// ScreenshotBase64 返回截图的 base64 编码字符串，供 HTTP provider 使用。
func (p *RecoveryPrompt) ScreenshotBase64() string {
	if len(p.Screenshot) == 0 {
		return ""
	}
	return base64.StdEncoding.EncodeToString(p.Screenshot)
}

// buildRecoveryPrompt 从遍历上下文构建恢复阶段的语义提示。
//
// 设计原则（参考 Midscene）：
//   - 多模态优先：截图是恢复决策的核心输入，XML 作为辅助
//   - 坐标归一化：LLM 返回 (x, y) 归一化坐标 [0, 1]，引擎转换为像素坐标
//   - 结构化历史：RecentTrace 格式化为可读描述
func buildRecoveryPrompt(ctx enginestate.TraversalContext) RecoveryPrompt {
	systemInstruction := `你是一个专业的 Android UI 自动遍历恢复规划器。你将收到当前屏幕的截图和上下文信息，需要规划下一步恢复动作。

关键规则：
1. 仔细观察截图中的 UI 元素，返回可执行的操作
2. 对需要点击/长按的元素，返回其中心点的归一化坐标 (x, y)，取值范围 [0, 1]
3. 页面可能处于异常状态（弹窗、卡死、无响应），优先选择返回/关闭等恢复操作
4. 返回 JSON 格式的候选动作列表`

	userMessage := buildUserMessage(ctx)

	mediaType := "image/png"
	if len(ctx.Screenshot) == 0 {
		mediaType = ""
	}

	return RecoveryPrompt{
		SystemContent:       systemInstruction,
		UserContent:         userMessage,
		Screenshot:          cloneBytes(ctx.Screenshot),
		ScreenshotMediaType: mediaType,
		ContextFields: RecoveryContextFields{
			Step:             ctx.Step,
			Mode:             string(ctx.Mode),
			PageName:         ctx.PageName,
			PageSignature:    ctx.PageSignature,
			ClusterSignature: ctx.ClusterSignature,
			BlockReason:      ctx.BlockReason,
			RecentTrace:      cloneTrace(ctx.RecentTrace),
			PageVisitCount:   cloneIntMap(ctx.VisitStats.PageVisitCount),
			ActionCount:      cloneIntMap(ctx.VisitStats.ActionCount),
		},
		ResponseSchema: recoveryCandidateSchema(),
	}
}

// buildUserMessage 构建用户级结构化上下文描述。
func buildUserMessage(ctx enginestate.TraversalContext) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("当前步数: %d\n", ctx.Step))
	sb.WriteString(fmt.Sprintf("遍历模式: %s\n", ctx.Mode))
	if ctx.PageName != "" {
		sb.WriteString(fmt.Sprintf("当前页面: %s\n", ctx.PageName))
	}
	if ctx.BlockReason != "" {
		sb.WriteString(fmt.Sprintf("阻塞原因: %s\n", ctx.BlockReason))
	}

	// 页面访问统计
	if len(ctx.VisitStats.PageVisitCount) > 0 {
		sb.WriteString("页面访问次数:\n")
		for page, count := range ctx.VisitStats.PageVisitCount {
			sb.WriteString(fmt.Sprintf("  %s: %d次\n", page, count))
		}
	}
	if len(ctx.VisitStats.ActionCount) > 0 {
		sb.WriteString("动作执行次数:\n")
		for action, count := range ctx.VisitStats.ActionCount {
			sb.WriteString(fmt.Sprintf("  %s: %d次\n", action, count))
		}
	}

	// 最近操作轨迹（格式化而非 Go 默认 %v）
	if len(ctx.RecentTrace) > 0 {
		sb.WriteString("最近操作轨迹:\n")
		for i := len(ctx.RecentTrace) - 1; i >= 0; i-- {
			trace := ctx.RecentTrace[i]
			sb.WriteString(fmt.Sprintf("  %s → %s\n", trace.PageSignature, trace.ActionKey))
		}
	}

	// 如果有 XML 页面源，附加为参考
	if ctx.XML != "" {
		xmlSnippet := ctx.XML
		const maxXMLLen = 4000
		if len(xmlSnippet) > maxXMLLen {
			xmlSnippet = xmlSnippet[:maxXMLLen] + "\n... (截断)"
		}
		sb.WriteString("\n页面结构（XML）:\n")
		sb.WriteString(xmlSnippet)
		sb.WriteString("\n")
	}

	return sb.String()
}

// recoveryCandidateSchema 返回恢复候选的 JSON Schema 定义。
//
// 坐标系设计（参考 Midscene）：
//   - point 使用归一化坐标 [0, 1]，x 为水平（左→右），y 为垂直（上→下）
//   - 引擎层将归一化坐标转换为像素坐标
//   - BACK、ACTIVATE 等无位置动作不需要 point
func recoveryCandidateSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"candidates": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"intent":       map[string]any{"type": "string", "description": "动作意图描述"},
						"action_type":  map[string]any{"type": "string", "enum": []string{"BACK", "CLICK", "LONG_CLICK", "SCROLL_TOP_DOWN", "SCROLL_BOTTOM_UP", "SCROLL_LEFT_RIGHT", "SCROLL_RIGHT_LEFT", "SCROLL_BOTTOM_UP_N", "ACTIVATE"}},
						"point": map[string]any{
							"type":        "object",
							"description": "目标元素中心点的归一化坐标，x 水平 [0,1]，y 垂直 [0,1]",
							"properties": map[string]any{
								"x": map[string]any{"type": "number", "description": "归一化 x 坐标 [0,1]，左为 0，右为 1"},
								"y": map[string]any{"type": "number", "description": "归一化 y 坐标 [0,1]，上为 0，下为 1"},
							},
							"required": []string{"x", "y"},
						},
						"confidence":   map[string]any{"type": "number", "description": "动作置信度 [0,1]"},
						"escape_score": map[string]any{"type": "number", "description": "逃逸分数，越高表示越能脱离当前阻塞"},
						"reason":       map[string]any{"type": "string", "description": "选择此动作的理由"},
						"target_hint":  map[string]any{"type": "string", "description": "目标元素的描述性提示"},
					},
					"required": []string{"action_type", "confidence"},
				},
			},
		},
		"required": []string{"candidates"},
	}
}

// cloneBytes 深拷贝字节切片，防止共享状态泄漏。
func cloneBytes(src []byte) []byte {
	if len(src) == 0 {
		return nil
	}
	result := make([]byte, len(src))
	copy(result, src)
	return result
}

// cloneTrace 深拷贝 ActionTrace 切片，防止共享状态泄漏。
func cloneTrace(src []enginestate.ActionTrace) []enginestate.ActionTrace {
	if len(src) == 0 {
		return nil
	}
	result := make([]enginestate.ActionTrace, len(src))
	copy(result, src)
	return result
}

// cloneIntMap 深拷贝 map[string]int，防止共享状态泄漏。
func cloneIntMap(src map[string]int) map[string]int {
	if len(src) == 0 {
		return nil
	}
	result := make(map[string]int, len(src))
	for key, value := range src {
		result[key] = value
	}
	return result
}