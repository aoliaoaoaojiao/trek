//go:build integration

package llm

import (
	"testing"
	"trek/internal/engine/state"
	"trek/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenAIIntegration_BuildCandidates(t *testing.T) {
	driver := testutil.RequireDevice(t)
	baseURL, apiKey, model := testutil.RequireOpenAIEnv(t)

	xml, screenshot := testutil.CapturePageSnapshot(t, driver)

	provider, err := NewOpenAIResponsesProvider(OpenAIResponsesProviderConfig{
		BaseURL: baseURL,
		APIKey:  apiKey,
		Model:   model,
	})
	require.NoError(t, err, "创建 OpenAI provider 失败")

	ctx := state.BuildTraversalContext(state.BuildInput{
		Step:       1,
		Mode:       state.ModeRecover,
		XML:        xml,
		Screenshot: screenshot,
	})

	candidates, err := provider.BuildCandidates(ctx)
	require.NoError(t, err, "OpenAI BuildCandidates 调用失败")
	t.Logf("OpenAI 返回 %d 个恢复候选", len(candidates))
	assert.NotEmpty(t, candidates, "OpenAI 应返回至少一个恢复候选")

	for i, c := range candidates {
		require.NotNil(t, c.Command, "候选 %d 的 Command 不应为 nil", i)
		assert.True(t, c.Command.IsValid(), "候选 %d 的 ActionCommand 应有效", i)
		t.Logf("候选 %d: act=%s pos=[%v,%v,%v,%v] intent=%s confidence=%.2f",
			i, c.Command.Act,
			c.Command.Pos.Left, c.Command.Pos.Top,
			c.Command.Pos.Right, c.Command.Pos.Bottom,
			c.Intent, c.Confidence)
	}
}

func TestOpenAIIntegration_DetectPageControls(t *testing.T) {
	driver := testutil.RequireDevice(t)
	baseURL, apiKey, model := testutil.RequireOpenAIEnv(t)

	xml, screenshot := testutil.CapturePageSnapshot(t, driver)

	provider, err := NewOpenAIResponsesProvider(OpenAIResponsesProviderConfig{
		BaseURL: baseURL,
		APIKey:  apiKey,
		Model:   model,
	})
	require.NoError(t, err)

	ctx := state.BuildTraversalContext(state.BuildInput{
		Step:       1,
		Mode:       state.ModeExplore,
		XML:        xml,
		Screenshot: screenshot,
	})

	candidates, err := provider.DetectPageControls(ctx)
	require.NoError(t, err, "OpenAI DetectPageControls 调用失败")
	t.Logf("OpenAI 控件检测返回 %d 个候选", len(candidates))
	assert.NotEmpty(t, candidates, "OpenAI 控件检测应返回至少一个候选")

	for i, c := range candidates {
		require.NotNil(t, c.Command, "候选 %d 的 Command 不应为 nil", i)
		t.Logf("控件 %d: act=%s bounds=[%v,%v,%v,%v] intent=%s confidence=%.2f",
			i, c.Command.Act,
			c.Command.Pos.Left, c.Command.Pos.Top,
			c.Command.Pos.Right, c.Command.Pos.Bottom,
			c.Intent, c.Confidence)
	}
}
