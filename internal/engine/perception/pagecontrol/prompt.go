package pagecontrol

import (
	"encoding/base64"
	"fmt"
	"strings"

	enginestate "trek/internal/engine/state"
)

// Prompt 是页面控件检测专用提示，供视觉模型输出控件区域。
type Prompt struct {
	SystemContent       string
	UserContent         string
	Screenshot          []byte
	ScreenshotMediaType string
	ResponseSchema      map[string]any
}

// ScreenshotBase64 返回截图的 base64 编码字符串。
func (p *Prompt) ScreenshotBase64() string {
	if len(p.Screenshot) == 0 {
		return ""
	}
	return base64.StdEncoding.EncodeToString(p.Screenshot)
}

// BuildPrompt 构建页面控件检测提示。
func BuildPrompt(ctx enginestate.TraversalContext) Prompt {
	systemInstruction := `你是一个专业的 Android UI 视觉控件检测器。你将收到当前屏幕截图，需要识别页面中的可交互控件区域，并返回结构化控件列表。

关键规则：
1. 只返回当前截图中真正可操作或高价值的控件区域，不要输出整页背景
2. bounds 必须使用归一化坐标，取值范围 [0,1]；优先返回对象格式 {"left","top","right","bottom"}，也可返回四元数组 [left, top, right, bottom]
3. 优先识别按钮、输入框、标签页、列表项、关闭按钮、返回按钮、弹窗主按钮等关键控件
4. 若控件文字可见，请尽量填写 text；若无法确认文字，可填写 hint
5. 输出必须是 JSON，且仅返回符合 schema 的 controls 数组`

	userMessage := buildUserMessage(ctx)
	mediaType := "image/png"
	if len(ctx.Screenshot) == 0 {
		mediaType = ""
	}
	return Prompt{
		SystemContent:       systemInstruction,
		UserContent:         userMessage,
		Screenshot:          cloneBytes(ctx.Screenshot),
		ScreenshotMediaType: mediaType,
		ResponseSchema:      schema(),
	}
}

func buildUserMessage(ctx enginestate.TraversalContext) string {
	var sb strings.Builder
	if ctx.PageName != "" {
		sb.WriteString(fmt.Sprintf("页面名: %s\n", ctx.PageName))
	}
	if ctx.Step > 0 {
		sb.WriteString(fmt.Sprintf("当前步数: %d\n", ctx.Step))
	}
	if ctx.XML != "" {
		xmlSnippet := ctx.XML
		const maxXMLLen = 2500
		if len(xmlSnippet) > maxXMLLen {
			xmlSnippet = xmlSnippet[:maxXMLLen] + "\n... (截断)"
		}
		sb.WriteString("可参考的页面结构(XML)：\n")
		sb.WriteString(xmlSnippet)
		sb.WriteString("\n")
	}
	sb.WriteString("请输出当前截图中的关键控件区域。")
	return sb.String()
}

func schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"controls": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"control_type": map[string]any{
							"type": "string",
							"enum": []string{"button", "input", "tab", "list_item", "icon", "text", "dialog_action", "close", "back", "unknown"},
						},
						"text":       map[string]any{"type": "string"},
						"hint":       map[string]any{"type": "string"},
						"clickable":  map[string]any{"type": "boolean"},
						"confidence": map[string]any{"type": "number"},
						"bounds": map[string]any{
							"oneOf": []map[string]any{
								{
									"type": "object",
									"properties": map[string]any{
										"left":   map[string]any{"type": "number"},
										"top":    map[string]any{"type": "number"},
										"right":  map[string]any{"type": "number"},
										"bottom": map[string]any{"type": "number"},
									},
									"required": []string{"left", "top", "right", "bottom"},
								},
								{
									"type": "array",
									"items": map[string]any{
										"type": "number",
									},
									"minItems": 4,
									"maxItems": 4,
								},
							},
						},
					},
					"required": []string{"bounds", "confidence"},
				},
			},
		},
		"required": []string{"controls"},
	}
}

func cloneBytes(src []byte) []byte {
	if len(src) == 0 {
		return nil
	}
	result := make([]byte, len(src))
	copy(result, src)
	return result
}
