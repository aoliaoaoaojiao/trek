package pagecontrol

import (
	"bytes"
	_ "embed"
	"encoding/base64"
	"fmt"
	"strings"

	enginestate "trek/internal/engine/state"
	"trek/internal/vision/imageproc"
	"image"

	trekann "trek/internal/vision/annotation"
	"trek/logger"
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
// 当 ctx.AnnotationEnabled 时自动转为带标注的 BuildAnnotatedPrompt。
func BuildPrompt(ctx enginestate.TraversalContext) Prompt {
	// 上下文标记了标注启用时，自动走标注路径
	if ctx.AnnotationEnabled {
		return BuildAnnotatedPrompt(ctx, nil, AnnotationConfig{
			Enabled:   true,
			FontScale: ctx.AnnotationFontScale,
		})
	}
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
// 使用双线性插值替代原来的 nearest-neighbor，JPEG Q85 → Q90。
func resizeForLLM(data []byte, maxWidths ...int) ([]byte, int, int, int, int) {
	maxWidth := llmScreenshotMaxWidth
	if len(maxWidths) > 0 && maxWidths[0] > 0 {
		maxWidth = maxWidths[0]
	}
	if len(data) == 0 {
		return data, 0, 0, 0, 0
	}

	cfg := imageproc.DefaultVLMConfig()
	cfg.MaxWidth = maxWidth
	cfg.Quality = 90

	optimized, origW, origH, newW, newH, err := imageproc.OptimizeForVLM(data, cfg)
	if err != nil {
		return data, origW, origH, origW, origH
	}
	return optimized, origW, origH, newW, newH
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

// AnnotationConfig 配置 VLM 截图编号标注。
type AnnotationConfig struct {
	Enabled   bool
	FontScale int
}

// BuildAnnotatedPrompt 构建带编号标注的页面控件检测提示。
// 在发送给 VLM 前在截图上绘制带编号的边界框，使 VLM 可引用元素编号而非仅靠坐标猜测。
// rects 可选：传 nil 时自动从 ctx.XML 提取元素边界框（需先解码截图获取尺寸）。
func BuildAnnotatedPrompt(ctx enginestate.TraversalContext, rects []image.Rectangle, annCfg AnnotationConfig) Prompt {
	screenshot := cloneBytes(ctx.Screenshot)

	// 自动从 XML 提取 rects
	if len(rects) == 0 && annCfg.Enabled && ctx.XML != "" && len(screenshot) > 0 {
		if img, _, err := image.Decode(bytes.NewReader(screenshot)); err == nil {
			if w, h := img.Bounds().Dx(), img.Bounds().Dy(); w > 0 && h > 0 {
				rects = ExtractRectsFromXML(ctx.XML, w, h)
				if len(rects) > 0 {
					logger.Debugf("[annotation] 从 XML 提取 %d 个标注框 (shot=%dx%d)", len(rects), w, h)
				}
			}
		}
	}

	// 执行编号标注
	if annCfg.Enabled && len(screenshot) > 0 && len(rects) > 0 {
		annotated, err := trekann.DrawLabeledBoxesFromBytes(screenshot, rects)
		if err == nil && len(annotated) > 0 {
			screenshot = annotated
		}
	}

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

// parseBoundsInt 解析 bounds="[left,top][right,bottom]" 字符串。
func parseBoundsInt(bStr string) (int, int, int, int) {
	parts := strings.Split(bStr, "][")
	if len(parts) != 2 {
		return -1, -1, -1, -1
	}
	lt := strings.Split(strings.Trim(parts[0], "[]"), ",")
	rb := strings.Split(strings.Trim(parts[1], "[]"), ",")
	if len(lt) != 2 || len(rb) != 2 {
		return -1, -1, -1, -1
	}
	return atoi(lt[0]), atoi(lt[1]), atoi(rb[0]), atoi(rb[1])
}

// atoi 快速字符串转整数。
func atoi(s string) int {
	v := 0
	neg := false
	for i := 0; i < len(s); i++ {
		if s[i] == '-' && i == 0 {
			neg = true
			continue
		}
		if s[i] >= '0' && s[i] <= '9' {
			v = v*10 + int(s[i]-'0')
		}
	}
	if neg {
		return -v
	}
	return v
}

// ExtractRectsFromXML 从 UI Automator XML 中提取元素边界框，返回 [0,10000] 范围的 image.Rectangle。
// 过滤过小（< 10px）和全屏元素。
func ExtractRectsFromXML(xml string, shotW, shotH int) []image.Rectangle {
	if xml == "" || shotW <= 0 || shotH <= 0 {
		return nil
	}
	var rects []image.Rectangle
	searchFrom := 0
	for {
		marker := `bounds="`
		start := strings.Index(xml[searchFrom:], marker)
		if start < 0 {
			break
		}
		valStart := searchFrom + start + len(marker)
		end := strings.IndexByte(xml[valStart:], '"')
		if end < 0 {
			break
		}
		bStr := xml[valStart : valStart+end]
		searchFrom = valStart + end + 1

		l, t, r, b := parseBoundsInt(bStr)
		if l < 0 || t < 0 || r < 0 || b < 0 {
			continue
		}
		w := r - l
		h := b - t
		if w < 10 || h < 10 || w >= shotW-10 || h >= shotH-10 {
			continue
		}
		rects = append(rects, image.Rect(l, t, r, b))
		if len(rects) >= 50 {
			break
		}
	}
	return rects
}
