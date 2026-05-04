package pagecontrol

import (
	_ "embed"
	"encoding/base64"
	"fmt"
	"strings"

	enginestate "trek/internal/engine/state"
)

//go:embed prompt_system.md
var promptSystemContent string

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
	userMessage := buildUserMessage(ctx)
	mediaType := "image/png"
	if len(ctx.Screenshot) == 0 {
		mediaType = ""
	}
	return Prompt{
		SystemContent:       strings.TrimSpace(promptSystemContent),
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
						"action_type": map[string]any{
							"type": "string",
							"enum": []string{"click", "drag", "swipe_up", "swipe_down", "swipe_left", "swipe_right", "input"},
						},
						"control_type": map[string]any{
							"type": "string",
							"enum": []string{"button", "input", "tab", "list_item", "icon", "text", "dialog_action", "close", "back", "unknown"},
						},
						"text":       map[string]any{"type": "string"},
						"hint":       map[string]any{"type": "string"},
						"clickable":  map[string]any{"type": "boolean"},
						"confidence": map[string]any{"type": "number"},
						"drag_target": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"x": map[string]any{"type": "number"},
								"y": map[string]any{"type": "number"},
							},
							"required": []string{"x", "y"},
						},
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
					"required": []string{"action_type", "bounds", "confidence"},
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
