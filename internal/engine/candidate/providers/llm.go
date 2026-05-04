package providers

import candidatellm "trek/internal/engine/candidate/providers/llm"

// LLMHTTPProviderConfig 是通用 HTTP LLM provider 的对外配置入口。
type LLMHTTPProviderConfig = candidatellm.LLMHTTPProviderConfig

// OpenAIChatProviderConfig 是 OpenAI Chat Completions provider 的对外配置入口。
type OpenAIChatProviderConfig = candidatellm.OpenAIChatProviderConfig

// OpenAIResponsesProviderConfig 是 OpenAI Responses provider 的对外配置入口。
type OpenAIResponsesProviderConfig = candidatellm.OpenAIResponsesProviderConfig

// AnthropicMessagesProviderConfig 是 Anthropic Messages provider 的对外配置入口。
type AnthropicMessagesProviderConfig = candidatellm.AnthropicMessagesProviderConfig

// LLMHTTPProvider 是通用 HTTP LLM provider 的对外别名。
type LLMHTTPProvider = candidatellm.LLMHTTPProvider

// OpenAIChatProvider 是 OpenAI Chat Completions provider 的对外别名。
type OpenAIChatProvider = candidatellm.OpenAIChatProvider

// OpenAIResponsesProvider 是 OpenAI Responses provider 的对外别名。
type OpenAIResponsesProvider = candidatellm.OpenAIResponsesProvider

// AnthropicMessagesProvider 是 Anthropic Messages provider 的对外别名。
type AnthropicMessagesProvider = candidatellm.AnthropicMessagesProvider

// NewLLMHTTPProvider 通过 providers 门面创建通用 HTTP LLM provider。
func NewLLMHTTPProvider(cfg LLMHTTPProviderConfig) (*LLMHTTPProvider, error) {
	return candidatellm.NewLLMHTTPProvider(cfg)
}

// NewOpenAIChatProvider 通过 providers 门面创建 OpenAI Chat provider。
func NewOpenAIChatProvider(cfg OpenAIChatProviderConfig) (*OpenAIChatProvider, error) {
	return candidatellm.NewOpenAIChatProvider(cfg)
}

// NewOpenAIResponsesProvider 通过 providers 门面创建 OpenAI Responses provider。
func NewOpenAIResponsesProvider(cfg OpenAIResponsesProviderConfig) (*OpenAIResponsesProvider, error) {
	return candidatellm.NewOpenAIResponsesProvider(cfg)
}

// NewAnthropicMessagesProvider 通过 providers 门面创建 Anthropic Messages provider。
func NewAnthropicMessagesProvider(cfg AnthropicMessagesProviderConfig) (*AnthropicMessagesProvider, error) {
	return candidatellm.NewAnthropicMessagesProvider(cfg)
}
