package runtime

import (
	engineplugin "trek/internal/engine/plugin"
)

func OnStepResult(input StepResultInput) error {
	return defaultRuntime.OnStepResult(input)
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
