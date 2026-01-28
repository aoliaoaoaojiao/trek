package driver

import "trek/internal/core/types"

type TouchEventType string

const (
	DOWN_TOUCH_EVENT TouchEventType = "down"
	UP_TOUCH_EVENT   TouchEventType = "up"
	MOVE_TOUCH_EVENT TouchEventType = "move"
)

type TouchEvent struct {
	Type     TouchEventType
	WaitTime int64
}

type IDriver interface {
	Name() string
	Click(point types.Point) error
	LongClick(point types.Point, duration int64) error
	Swipe(startPoint types.Point, endPoint types.Point, step int64, duration int64) error
	TouchEvent(touchList []TouchEvent) error
}
