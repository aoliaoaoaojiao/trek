package monkey

import (
	"testing"
	session "trek/pkg/coordinator"
	"trek/internal/engine/core/types"
	"trek/internal/testutil"
)

func TestBlockDetectorScrollNoChangeRespectsImageSSIM(t *testing.T) {
	original := testutil.ReadRootFixture(t, testutil.FixtureGameNavigation)
	mutated := mustMutateContentAreaFixture(t, original)

	detector := newBlockDetector(3, 0, 0, 0).withImageSimilarity(0.9999, []ImageFingerprintRegion{
		{Left: 0.25, Top: 0.25, Right: 0.75, Bottom: 0.6},
	})
	cmd := &types.ActionCommand{Act: types.SCROLL_BOTTOM_UP, Pos: *types.NewRect(0, 0, 1, 1)}
	before := session.PageSnapshot{
		PageName:   "IMGPage:test",
		XML:        "",
		Screenshot: original,
	}
	after := &session.PageSnapshot{
		PageName:   "IMGPage:test",
		XML:        "",
		Screenshot: mutated,
	}

	triggered := false
	for i := 0; i < 3; i++ {
		triggered = detector.Observe(cmd, before, after)
	}
	if triggered {
		t.Fatal("截图内容已明显变化时，不应误判为 scroll_no_change")
	}
}

func TestBlockDetectorSamePageNoChangeRespectsImageSSIM(t *testing.T) {
	original := testutil.ReadRootFixture(t, testutil.FixtureGameNavigation)
	mutated := mustMutateContentAreaFixture(t, original)

	detector := newBlockDetector(3, 0, 0, 0).withImageSimilarity(0.9999, []ImageFingerprintRegion{
		{Left: 0.25, Top: 0.25, Right: 0.75, Bottom: 0.6},
	})
	cmd := &types.ActionCommand{Act: types.CLICK, Pos: *types.NewRect(0.1, 0.1, 0.2, 0.2)}
	before := session.PageSnapshot{
		PageName:   "IMGPage:test",
		XML:        "",
		Screenshot: original,
	}
	after := &session.PageSnapshot{
		PageName:   "IMGPage:test",
		XML:        "",
		Screenshot: mutated,
	}

	triggered := false
	for i := 0; i < 3; i++ {
		triggered = detector.Observe(cmd, before, after)
	}
	if triggered {
		t.Fatal("截图内容已明显变化时，不应误判为 same_page_no_change")
	}
}
