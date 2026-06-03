package pagecontrol

import (
	"bytes"
	_ "embed"
	"encoding/base64"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png"
	"strings"

	enginestate "trek/internal/engine/state"
)

//go:embed prompt_system.md
var promptSystemContent string

const llmScreenshotMaxWidth = 1280

// Prompt 是页面控件检测专用提示，供视觉模型输出控件区域。
type Prompt struct {
	SystemContent       string
	UserContent         string
	Screenshot          []byte
	ScreenshotMediaType string
	ResponseSchema      map[string]any
	OrigWidth           int
	OrigHeight          int
	ShotWidth           int
	ShotHeight          int
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
	screenshot := cloneBytes(ctx.Screenshot)
	origW, origH, shotW, shotH := 0, 0, 0, 0
	resized := false
	if len(screenshot) > 0 {
		screenshot, origW, origH, shotW, shotH = resizeForLLM(screenshot)
		resized = origW != shotW || origH != shotH
	}
	userMessage := buildUserMessage(ctx, origW, origH)
	mediaType := "image/png"
	if len(screenshot) == 0 {
		mediaType = ""
	} else if resized {
		mediaType = "image/jpeg"
	}
	return Prompt{
		SystemContent:       strings.TrimSpace(promptSystemContent),
		UserContent:         userMessage,
		Screenshot:          screenshot,
		ScreenshotMediaType: mediaType,
		ResponseSchema:      schema(),
		OrigWidth:           origW,
		OrigHeight:          origH,
		ShotWidth:           shotW,
		ShotHeight:          shotH,
	}
}

// resizeForLLM 缩放截图以减少 token 消耗，返回缩放后的字节和原始/缩放后尺寸。
func resizeForLLM(data []byte, maxWidths ...int) ([]byte, int, int, int, int) {
	maxWidth := llmScreenshotMaxWidth
	if len(maxWidths) > 0 && maxWidths[0] > 0 {
		maxWidth = maxWidths[0]
	}
	if len(data) == 0 {
		return data, 0, 0, 0, 0
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return data, 0, 0, 0, 0
	}
	bounds := img.Bounds()
	origW := bounds.Dx()
	origH := bounds.Dy()
	if origW <= maxWidth {
		return data, origW, origH, origW, origH
	}
	scale := float64(maxWidth) / float64(origW)
	newW := maxWidth
	newH := int(float64(origH)*scale + 0.5)
	if newH <= 0 {
		newH = 1
	}
	resized := image.NewRGBA(image.Rect(0, 0, newW, newH))
	for y := 0; y < newH; y++ {
		srcY := bounds.Min.Y + int(float64(y)/scale+0.5)
		if srcY >= bounds.Max.Y {
			srcY = bounds.Max.Y - 1
		}
		for x := 0; x < newW; x++ {
			srcX := bounds.Min.X + int(float64(x)/scale+0.5)
			if srcX >= bounds.Max.X {
				srcX = bounds.Max.X - 1
			}
			resized.Set(x, y, img.At(srcX, srcY))
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, resized, &jpeg.Options{Quality: 85}); err != nil {
		return data, origW, origH, origW, origH
	}
	return buf.Bytes(), origW, origH, newW, newH
}

func buildUserMessage(ctx enginestate.TraversalContext, origW, origH int) string {
	var sb strings.Builder
	if ctx.PageName != "" {
		sb.WriteString(fmt.Sprintf("页面名: %s\n", ctx.PageName))
	}
	if ctx.Step > 0 {
		sb.WriteString(fmt.Sprintf("当前步数: %d\n", ctx.Step))
	}
	if origW > 0 && origH > 0 {
		sb.WriteString(fmt.Sprintf("设备屏幕分辨率: %dx%d\n", origW, origH))
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
							"enum": []string{"button", "input", "tab", "list_item", "icon", "text", "dialog_action", "close", "back", "drag_handle", "draggable", "unknown"},
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
