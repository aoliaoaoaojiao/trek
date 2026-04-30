package runtime

import (
	engineplugin "trek/internal/engine/plugin"
)

func OnStepResult(input StepResultInput) error {
	mu.RLock()
	p := scriptPlugin
	mu.RUnlock()
	if p == nil {
		return nil
	}
	before := pageSnapshotFromInput(input.Before)
	ctx := engineplugin.StepResultContext{
		PluginContext: engineplugin.PluginContext{
			Page: before,
			Runtime: engineplugin.RuntimeContext{
				PackageName: packageName(),
			},
		},
		Result: engineplugin.StepResult{
			Step:       input.Step,
			Action:     engineplugin.FromActionCommand(input.Action),
			Success:    input.Success,
			Error:      input.Error,
			DurationMs: input.DurationMs,
			Crash:      input.Crash,
			ANR:        input.ANR,
			Before:     before,
			After:      pageSnapshotPtrFromInput(input.After),
		},
	}
	return p.OnStepResult(ctx)
}

func pageSnapshotFromInput(input PageSnapshotInput) engineplugin.PageSnapshot {
	page := engineplugin.PageSnapshot{
		Name: input.PageName,
		XML:  input.XML,
	}
	if len(input.Screenshot) > 0 {
		page.Screenshot = &engineplugin.Screenshot{
			Bytes: append([]byte(nil), input.Screenshot...),
			MIME:  "image/png",
		}
	}
	return page
}

func pageSnapshotPtrFromInput(input *PageSnapshotInput) *engineplugin.PageSnapshot {
	if input == nil {
		return nil
	}
	page := pageSnapshotFromInput(*input)
	return &page
}
