package llm

import (
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
	ModelFamilyOther   ModelFamily = "other"
)

// ModelAdapter 根据模型家族适配 prompt 格式。
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

	// Claude 系列
	if strings.Contains(lower, "claude") {
		return ModelFamilyClaude
	}

	// Gemini 系列
	if strings.Contains(lower, "gemini") {
		return ModelFamilyGemini
	}

	// Qwen 系列
	if strings.Contains(lower, "qwen") {
		return ModelFamilyQwen
	}

	// GPT 系列
	if strings.Contains(lower, "gpt") {
		return ModelFamilyGPT
	}

	return ModelFamilyOther
}

// AdaptSystemPrompt 根据模型家族调整系统提示。
func (a *ModelAdapter) AdaptSystemPrompt(basePrompt string) string {
	if a == nil {
		return basePrompt
	}

	switch a.Family {
	case ModelFamilyClaude:
		// Claude 擅长处理结构化指令，保持原样
		return basePrompt
	case ModelFamilyGemini:
		// Gemini 可能需要更简洁的指令
		return a.simplifyForGemini(basePrompt)
	case ModelFamilyQwen:
		// Qwen 中文优化，可以使用中文指令
		return a.adaptForQwen(basePrompt)
	case ModelFamilyGPT:
		// GPT 系列保持原样
		return basePrompt
	default:
		return basePrompt
	}
}

// AdaptUserMessage 根据模型家族调整用户消息。
func (a *ModelAdapter) AdaptUserMessage(baseMessage string) string {
	if a == nil {
		return baseMessage
	}

	switch a.Family {
	case ModelFamilyClaude:
		return baseMessage
	case ModelFamilyGemini:
		return a.simplifyForGemini(baseMessage)
	case ModelFamilyQwen:
		return a.adaptForQwen(baseMessage)
	case ModelFamilyGPT:
		return baseMessage
	default:
		return baseMessage
	}
}

// simplifyForGemini 为 Gemini 模型简化提示。
func (a *ModelAdapter) simplifyForGemini(prompt string) string {
	// Gemini 对长提示可能效果不佳，保持原样但可以后续优化
	return prompt
}

// adaptForQwen 为 Qwen 模型适配提示。
func (a *ModelAdapter) adaptForQwen(prompt string) string {
	// Qwen 中文优化，保持原样
	return prompt
}

// SupportsStructuredOutput 返回模型是否支持结构化输出。
func (a *ModelAdapter) SupportsStructuredOutput() bool {
	if a == nil {
		return false
	}

	switch a.Family {
	case ModelFamilyClaude, ModelFamilyGPT:
		return true
	case ModelFamilyGemini:
		return true
	case ModelFamilyQwen:
		return true
	default:
		return false
	}
}

// GetPreferredOutputFormat 返回模型偏好的输出格式。
func (a *ModelAdapter) GetPreferredOutputFormat() string {
	if a == nil {
		return "json"
	}

	switch a.Family {
	case ModelFamilyClaude:
		return "json"
	case ModelFamilyGemini:
		return "json"
	case ModelFamilyQwen:
		return "json"
	case ModelFamilyGPT:
		return "json"
	default:
		return "json"
	}
}
