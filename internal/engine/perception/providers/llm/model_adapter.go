package llm

import (
	"math"
	"strings"
)

// ModelFamily 表示模型家族类型。
type ModelFamily string

const (
	ModelFamilyUnknown ModelFamily = "unknown"
	ModelFamilyClaude  ModelFamily = "claude"
	ModelFamilyGemini  ModelFamily = "gemini"
	ModelFamilyQwen    ModelFamily = "qwen"
	ModelFamilyGPT     ModelFamily = "gpt"
	ModelFamilyDoubao  ModelFamily = "doubao"
	ModelFamilyGLM     ModelFamily = "glm"
	ModelFamilyAutoGLM ModelFamily = "autoglm"
	ModelFamilyOther   ModelFamily = "other"
)

// ModelAdapter 根据模型家族适配 prompt 格式和坐标格式。
type ModelAdapter struct {
	Family ModelFamily
}

// NewModelAdapter 从模型名称创建适配器。
func NewModelAdapter(modelName string) *ModelAdapter {
	return &ModelAdapter{
		Family: detectModelFamily(modelName),
	}
}

// detectModelFamily 从模型名称推断模型家族。
func detectModelFamily(modelName string) ModelFamily {
	lower := strings.ToLower(strings.TrimSpace(modelName))
	if lower == "" {
		return ModelFamilyUnknown
	}

	if strings.Contains(lower, "claude") {
		return ModelFamilyClaude
	}
	if strings.Contains(lower, "gemini") {
		return ModelFamilyGemini
	}
	if strings.Contains(lower, "qwen") {
		return ModelFamilyQwen
	}
	if strings.Contains(lower, "gpt") || strings.Contains(lower, "gpt-4o") || strings.Contains(lower, "o1") || strings.Contains(lower, "o3") {
		return ModelFamilyGPT
	}
	if strings.Contains(lower, "doubao") || strings.Contains(lower, "skylark") {
		return ModelFamilyDoubao
	}
	if strings.Contains(lower, "glm") || strings.Contains(lower, "glm-4v") || strings.Contains(lower, "cogview") {
		return ModelFamilyGLM
	}
	if strings.Contains(lower, "autoglm") {
		return ModelFamilyAutoGLM
	}

	return ModelFamilyOther
}

// AdaptSystemPrompt 根据模型家族调整系统提示。
func (a *ModelAdapter) AdaptSystemPrompt(basePrompt string) string {
	if a == nil {
		return basePrompt
	}
	return basePrompt
}

// AdaptUserMessage 根据模型家族调整用户消息。
func (a *ModelAdapter) AdaptUserMessage(baseMessage string) string {
	if a == nil {
		return baseMessage
	}
	return baseMessage
}

// SupportsStructuredOutput 返回模型是否支持结构化输出。
func (a *ModelAdapter) SupportsStructuredOutput() bool {
	if a == nil {
		return false
	}
	switch a.Family {
	case ModelFamilyClaude, ModelFamilyGPT, ModelFamilyGemini, ModelFamilyQwen,
		ModelFamilyDoubao, ModelFamilyGLM:
		return true
	default:
		return false
	}
}

// GetPreferredOutputFormat 返回模型偏好的输出格式。
func (a *ModelAdapter) GetPreferredOutputFormat() string {
	return "json"
}

// AdaptBbox 按模型族将 VLM 输出的边界框转换为归一化 [0,1] 坐标。
// 输入: [left, top, right, bottom]，shotW/shotH 是发送给 VLM 的截图尺寸。
//
// 各模型族坐标格式:
//   - Claude/GPT:       [0,1000] 归一化
//   - Gemini:           [y1, x1, y2, x2]（轴交换）
//   - Qwen2.5-VL:       原始像素坐标
//   - Doubao:           灵活格式（自动检测）
//   - GLM/GLM-4V:       [0,1000] 归一化（同标准格式）
//   - AutoGLM:          [0,999] 点坐标
func (a *ModelAdapter) AdaptBbox(bbox [4]float64, shotW, shotH int) [4]float64 {
	if a == nil {
		return a.adaptDefault(bbox, shotW, shotH)
	}

	switch a.Family {
	case ModelFamilyGemini:
		return a.adaptGemini(bbox, shotW, shotH)
	case ModelFamilyQwen:
		return a.adaptQwen(bbox, shotW, shotH)
	case ModelFamilyDoubao:
		return a.adaptDoubao(bbox, shotW, shotH)
	case ModelFamilyAutoGLM:
		return a.adaptAutoGLM(bbox, shotW, shotH)
	case ModelFamilyGLM:
		// GLM-4V 使用 [0,1000] 归一化，同标准格式
		return a.adaptDefault(bbox, shotW, shotH)
	case ModelFamilyClaude, ModelFamilyGPT, ModelFamilyOther:
		return a.adaptDefault(bbox, shotW, shotH)
	default:
		return a.adaptDefault(bbox, shotW, shotH)
	}
}

// AdaptBboxToRect 转换并返回归一化后的四元组。
func (a *ModelAdapter) AdaptBboxToRect(bbox [4]float64, shotW, shotH int) (left, top, right, bottom float64) {
	adapted := a.AdaptBbox(bbox, shotW, shotH)
	return adapted[0], adapted[1], adapted[2], adapted[3]
}

// adaptDefault 处理标准格式: [0,1000] 归一化坐标 → [0,1]。
func (a *ModelAdapter) adaptDefault(bbox [4]float64, shotW, shotH int) [4]float64 {
	left, top, right, bottom := bbox[0], bbox[1], bbox[2], bbox[3]

	if left <= 1 && top <= 1 && right <= 1 && bottom <= 1 {
		return bbox
	}
	if left <= 1000 && top <= 1000 && right <= 1000 && bottom <= 1000 {
		return [4]float64{left / 1000, top / 1000, right / 1000, bottom / 1000}
	}
	if shotW > 0 && shotH > 0 {
		return [4]float64{
			math.Max(0, math.Min(1, left/float64(shotW))),
			math.Max(0, math.Min(1, top/float64(shotH))),
			math.Max(0, math.Min(1, right/float64(shotW))),
			math.Max(0, math.Min(1, bottom/float64(shotH))),
		}
	}
	return bbox
}

// adaptGemini 处理 Gemini 格式: [y1, x1, y2, x2] → [x1, y1, x2, y2]。
func (a *ModelAdapter) adaptGemini(bbox [4]float64, shotW, shotH int) [4]float64 {
	swapped := [4]float64{bbox[1], bbox[0], bbox[3], bbox[2]}
	return a.adaptDefault(swapped, shotW, shotH)
}

// adaptQwen 处理 Qwen2.5-VL 格式: 原始像素坐标。
func (a *ModelAdapter) adaptQwen(bbox [4]float64, shotW, shotH int) [4]float64 {
	left, top, right, bottom := bbox[0], bbox[1], bbox[2], bbox[3]

	if left <= 1 && top <= 1 && right <= 1 && bottom <= 1 {
		return bbox
	}
	if shotW > 0 && shotH > 0 {
		return [4]float64{
			math.Max(0, math.Min(1, left/float64(shotW))),
			math.Max(0, math.Min(1, top/float64(shotH))),
			math.Max(0, math.Min(1, right/float64(shotW))),
			math.Max(0, math.Min(1, bottom/float64(shotH))),
		}
	}
	return bbox
}

// adaptDoubao 处理 Doubao-Vision 灵活格式。
// Doubao 可能返回: [x1,y1,x2,y2] 数组或 {left,top,right,bottom} 对象，
// 值可能是 [0,1000] 归一化或像素坐标，自动检测。
func (a *ModelAdapter) adaptDoubao(bbox [4]float64, shotW, shotH int) [4]float64 {
	return a.adaptDefault(bbox, shotW, shotH)
}

// adaptAutoGLM 处理 AutoGLM 格式: [0,999] 点坐标。
// AutoGLM 返回的是点击点而非边界框，扩展为一个微小矩形。
func (a *ModelAdapter) adaptAutoGLM(bbox [4]float64, shotW, shotH int) [4]float64 {
	// AutoGLM 可能返回 [x, y] 点坐标
	px := bbox[0]
	py := bbox[1]

	// [0,999] → [0,1]
	nx := math.Max(0, math.Min(1, px/999.0))
	ny := math.Max(0, math.Min(1, py/999.0))

	// 扩展为微小矩形（半径 5px）
	halfSize := 5.0
	var left, top, right, bottom float64
	if shotW > 0 && shotH > 0 {
		halfNormX := halfSize / float64(shotW)
		halfNormY := halfSize / float64(shotH)
		left = math.Max(0, nx-halfNormX)
		top = math.Max(0, ny-halfNormY)
		right = math.Min(1, nx+halfNormX)
		bottom = math.Min(1, ny+halfNormY)
	} else {
		left = math.Max(0, nx-0.01)
		top = math.Max(0, ny-0.01)
		right = math.Min(1, nx+0.01)
		bottom = math.Min(1, ny+0.01)
	}
	return [4]float64{left, top, right, bottom}
}

// ModelFamilyFromName 从模型名称推断 ModelFamily。
func ModelFamilyFromName(modelName string) ModelFamily {
	return detectModelFamily(modelName)
}
