//go:build integration

package providers

import (
	"testing"
	"trek/internal/engine/state"
	"trek/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOCRIntegration_BuildCandidatesFromRealDevice(t *testing.T) {
	driver := testutil.RequireDevice(t)
	endpoint, apiKey := testutil.RequireOCREnv(t)

	xml, screenshot := testutil.CapturePageSnapshot(t, driver)

	provider, err := NewOCRHTTPProvider(OCRHTTPProviderConfig{
		Endpoint: endpoint,
		APIKey:   apiKey,
	})
	require.NoError(t, err, "创建 OCR provider 失败")

	ctx := state.BuildTraversalContext(state.BuildInput{
		Step:       1,
		Mode:       state.ModeExplore,
		XML:        xml,
		Screenshot: screenshot,
	})

	candidates, err := provider.BuildCandidates(ctx)
	require.NoError(t, err, "OCR BuildCandidates 调用失败")
	t.Logf("OCR 返回 %d 个候选", len(candidates))
	assert.NotEmpty(t, candidates, "OCR 应返回至少一个候选")
}

func TestOCRIntegration_CandidatesHaveValidBounds(t *testing.T) {
	driver := testutil.RequireDevice(t)
	endpoint, apiKey := testutil.RequireOCREnv(t)

	xml, screenshot := testutil.CapturePageSnapshot(t, driver)

	provider, err := NewOCRHTTPProvider(OCRHTTPProviderConfig{
		Endpoint: endpoint,
		APIKey:   apiKey,
	})
	require.NoError(t, err)

	ctx := state.BuildTraversalContext(state.BuildInput{
		Step:       1,
		Mode:       state.ModeExplore,
		XML:        xml,
		Screenshot: screenshot,
	})

	candidates, err := provider.BuildCandidates(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, candidates)

	for i, c := range candidates {
		require.NotNil(t, c.Command, "候选 %d 的 Command 不应为 nil", i)
		assert.NotZero(t, c.Command.Pos, "候选 %d 的 Pos 不应为零值", i)
		t.Logf("候选 %d: bounds=[%v,%v,%v,%v] text=%q", i,
			c.Command.Pos.Left, c.Command.Pos.Top,
			c.Command.Pos.Right, c.Command.Pos.Bottom,
			c.Command.Text)
	}
}

func TestOCRIntegration_CandidatesHaveMetadata(t *testing.T) {
	driver := testutil.RequireDevice(t)
	endpoint, apiKey := testutil.RequireOCREnv(t)

	xml, screenshot := testutil.CapturePageSnapshot(t, driver)

	provider, err := NewOCRHTTPProvider(OCRHTTPProviderConfig{
		Endpoint: endpoint,
		APIKey:   apiKey,
	})
	require.NoError(t, err)

	ctx := state.BuildTraversalContext(state.BuildInput{
		Step:       1,
		Mode:       state.ModeExplore,
		XML:        xml,
		Screenshot: screenshot,
	})

	candidates, err := provider.BuildCandidates(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, candidates)

	for i, c := range candidates {
		assert.NotEmpty(t, c.Source, "候选 %d 的 Source 不应为空", i)
		assert.NotEmpty(t, c.Intent, "候选 %d 的 Intent 不应为空", i)
		t.Logf("候选 %d: source=%s intent=%s confidence=%.2f", i, c.Source, c.Intent, c.Confidence)
	}
}
