package llm

import (
	"testing"
)

func TestDetectModelFamily(t *testing.T) {
	tests := []struct {
		name     string
		model    string
		expected ModelFamily
	}{
		{"empty", "", ModelFamilyUnknown},
		{"claude-3", "claude-3-opus-20240229", ModelFamilyClaude},
		{"claude-sonnet", "claude-sonnet-4-20250514", ModelFamilyClaude},
		{"gemini-pro", "gemini-pro", ModelFamilyGemini},
		{"gemini-1.5", "gemini-1.5-flash", ModelFamilyGemini},
		{"qwen-7b", "qwen-7b-chat", ModelFamilyQwen},
		{"qwen-max", "qwen-max", ModelFamilyQwen},
		{"gpt-4", "gpt-4", ModelFamilyGPT},
		{"gpt-3.5", "gpt-3.5-turbo", ModelFamilyGPT},
		{"other", "llama-2-70b", ModelFamilyOther},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := NewModelAdapter(tt.model)
			if adapter.Family != tt.expected {
				t.Errorf("detectModelFamily(%q) = %v, want %v", tt.model, adapter.Family, tt.expected)
			}
		})
	}
}

func TestModelAdapter_AdaptSystemPrompt(t *testing.T) {
	basePrompt := "You are a helpful assistant."

	tests := []struct {
		name     string
		model    string
		expected string
	}{
		{"nil adapter", "", basePrompt},
		{"claude", "claude-3-opus", basePrompt},
		{"gemini", "gemini-pro", basePrompt},
		{"qwen", "qwen-7b", basePrompt},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var adapter *ModelAdapter
			if tt.model != "" {
				adapter = NewModelAdapter(tt.model)
			}
			result := adapter.AdaptSystemPrompt(basePrompt)
			if result != tt.expected {
				t.Errorf("AdaptSystemPrompt(%q) = %q, want %q", tt.model, result, tt.expected)
			}
		})
	}
}

func TestModelAdapter_SupportsStructuredOutput(t *testing.T) {
	tests := []struct {
		name     string
		model    string
		expected bool
	}{
		{"nil", "", false},
		{"claude", "claude-3-opus", true},
		{"gemini", "gemini-pro", true},
		{"qwen", "qwen-7b", true},
		{"gpt", "gpt-4", true},
		{"other", "llama-2", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var adapter *ModelAdapter
			if tt.model != "" {
				adapter = NewModelAdapter(tt.model)
			}
			result := adapter.SupportsStructuredOutput()
			if result != tt.expected {
				t.Errorf("SupportsStructuredOutput() = %v, want %v", result, tt.expected)
			}
		})
	}
}
