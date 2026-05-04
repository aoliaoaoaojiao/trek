//go:build integration

package llm

import (
	"testing"
	"trek/internal/engine/state"
	"trek/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLLMIntegration_BuildCandidatesFromRealDevice(t *testing.T) {
	driver := testutil.RequireDevice(t)
	endpoint, apiKey, model := testutil.RequireLLMEnv(t)

	xml, screenshot := testutil.CapturePageSnapshot(t, driver)

	provider, err := NewLLMHTTPProvider(LLMHTTPProviderConfig{
		Endpoint: endpoint,
		APIKey:   apiKey,
		Model:    model,
	})
	require.NoError(t, err, "创建 LLM provider 失败")

	ctx := state.BuildTraversalContext(state.BuildInput{
		Step:       1,
		Mode:       state.ModeRecover,
		XML:        xml,
		Screenshot: screenshot,
	})

	candidates, err := provider.BuildCandidates(ctx)
	require.NoError(t, err, "LLM BuildCandidates 调用失败")
	t.Logf("LLM 返回 %d 个恢复候选", len(candidates))
	assert.NotEmpty(t, candidates, "LLM 应返回至少一个恢复候选")
}

func TestLLMIntegration_DetectPageControls(t *testing.T) {
	driver := testutil.RequireDevice(t)
	endpoint, apiKey, model := testutil.RequireLLMEnv(t)

	xml, screenshot := testutil.CapturePageSnapshot(t, driver)

	provider, err := NewLLMHTTPProvider(LLMHTTPProviderConfig{
		Endpoint: endpoint,
		APIKey:   apiKey,
		Model:    model,
	})
	require.NoError(t, err)

	ctx := state.BuildTraversalContext(state.BuildInput{
		Step:       1,
		Mode:       state.ModeExplore,
		XML:        xml,
		Screenshot: screenshot,
	})

	candidates, err := provider.DetectPageControls(ctx)
	require.NoError(t, err, "LLM DetectPageControls 调用失败")
	t.Logf("LLM 控件检测返回 %d 个候选", len(candidates))
	assert.NotEmpty(t, candidates, "LLM 控件检测应返回至少一个候选")
}

func TestLLMIntegration_CandidatesHaveValidActions(t *testing.T) {
	driver := testutil.RequireDevice(t)
	endpoint, apiKey, model := testutil.RequireLLMEnv(t)

	xml, screenshot := testutil.CapturePageSnapshot(t, driver)

	provider, err := NewLLMHTTPProvider(LLMHTTPProviderConfig{
		Endpoint: endpoint,
		APIKey:   apiKey,
		Model:    model,
	})
	require.NoError(t, err)

	ctx := state.BuildTraversalContext(state.BuildInput{
		Step:       1,
		Mode:       state.ModeRecover,
		XML:        xml,
		Screenshot: screenshot,
	})

	candidates, err := provider.BuildCandidates(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, candidates)

	for i, c := range candidates {
		require.NotNil(t, c.Command, "候选 %d 的 Command 不应为 nil", i)
		assert.True(t, c.Command.IsValid(), "候选 %d 的 ActionCommand 应有效", i)
		assert.NotEmpty(t, c.Source, "候选 %d 的 Source 不应为空", i)
		t.Logf("候选 %d: act=%s pos=[%v,%v,%v,%v] source=%s intent=%s",
			i, c.Command.Act,
			c.Command.Pos.Left, c.Command.Pos.Top,
			c.Command.Pos.Right, c.Command.Pos.Bottom,
			c.Source, c.Intent)
	}
}
