package touch

import (
	"encoding/json"
	"fmt"
	"net/http"
	"trek/internal/engine/core/primitives"
	"trek/pkg/driver/android/uia"
	"trek/pkg/driver/common"
)

var _ common.ITouch = (*UIATouch)(nil)

const (
	defaultActionPauseMS = int64(50)
	defaultSwipeMS       = int64(500)
)

// NewUIATouch 创建 UIATouch 实例。
func NewUIATouch(client *uia.UiaClient) *UIATouch {
	return &UIATouch{
		UiaClient:    client,
		pointerState: make(map[int64]pointerState),
	}
}

type UIATouch struct {
	*uia.UiaClient
	pointerState map[int64]pointerState
}

type pointerState struct {
	point  primitives.Point
	isDown bool
}

type actionRequest struct {
	Actions []actionSource `json:"actions"`
}

type actionSource struct {
	Type       string                   `json:"type"`
	ID         string                   `json:"id"`
	Parameters map[string]interface{}   `json:"parameters,omitempty"`
	Actions    []map[string]interface{} `json:"actions"`
}

func (u *UIATouch) Click(point primitives.Point) error {
	return u.performPointerSequence(pointerID(0), []map[string]interface{}{
		pointerMoveAction(point, 0),
		pointerDownAction(),
		pauseAction(defaultActionPauseMS),
		pointerUpAction(),
	}, true)
}

func (u *UIATouch) LongClick(point primitives.Point, duration int64) error {
	if duration < 0 {
		return fmt.Errorf("long click duration must be >= 0, current: %d", duration)
	}

	return u.performPointerSequence(pointerID(0), []map[string]interface{}{
		pointerMoveAction(point, 0),
		pointerDownAction(),
		pauseAction(duration),
		pointerUpAction(),
	}, true)
}

// Swipe 使用 W3C Actions 执行滑动。
func (u *UIATouch) Swipe(startPoint primitives.Point, endPoint primitives.Point, step int64, duration int64) error {
	if duration <= 0 {
		duration = defaultSwipeMS
		if step > 0 {
			duration = step * 5
		}
	}

	return u.performPointerSequence(pointerID(0), []map[string]interface{}{
		pointerMoveAction(startPoint, 0),
		pointerDownAction(),
		pauseAction(defaultActionPauseMS),
		pointerMoveAction(endPoint, duration),
		pointerUpAction(),
	}, true)
}

func (u *UIATouch) Pinch(centerPoint primitives.Point, startDistance float64, endDistance float64, duration int64) error {
	if u.UiaClient == nil {
		return common.NoUIAClientErr
	}
	if err := u.CheckSessionId(); err != nil {
		return err
	}
	if duration <= 0 {
		return fmt.Errorf("pinch duration must be > 0, current: %d", duration)
	}
	if startDistance == endDistance {
		return nil
	}

	startHalfDist := startDistance / 2
	endHalfDist := endDistance / 2

	finger0Start := primitives.Point{X: centerPoint.X - startHalfDist, Y: centerPoint.Y}
	finger0End := primitives.Point{X: centerPoint.X - endHalfDist, Y: centerPoint.Y}
	finger1Start := primitives.Point{X: centerPoint.X + startHalfDist, Y: centerPoint.Y}
	finger1End := primitives.Point{X: centerPoint.X + endHalfDist, Y: centerPoint.Y}

	return u.performActions([]actionSource{
		newPointerSource(pointerID(0), []map[string]interface{}{
			pointerMoveAction(finger0Start, 0),
			pointerDownAction(),
			pauseAction(defaultActionPauseMS),
			pointerMoveAction(finger0End, duration),
			pointerUpAction(),
		}),
		newPointerSource(pointerID(1), []map[string]interface{}{
			pointerMoveAction(finger1Start, 0),
			pointerDownAction(),
			pauseAction(defaultActionPauseMS),
			pointerMoveAction(finger1End, duration),
			pointerUpAction(),
		}),
	}, true)
}

// Drag 使用与 Swipe 相同的 W3C Actions 实现。
func (u *UIATouch) Drag(startPoint primitives.Point, endPoint primitives.Point, duration int, elementId, destElId string) error {
	return u.Swipe(startPoint, endPoint, 0, int64(duration))
}

// TouchEvent 按顺序执行触摸事件。
func (u *UIATouch) TouchEvent(touchList ...common.TouchEvent) error {
	if u.UiaClient == nil {
		return common.NoUIAClientErr
	}
	if err := u.CheckSessionId(); err != nil {
		return err
	}

	for _, touch := range touchList {
		if err := u.TouchAction(touch); err != nil {
			return err
		}
	}
	return nil
}

// TouchAction 使用单指 W3C Actions 执行 down/move/up。
func (u *UIATouch) TouchAction(event common.TouchEvent) error {
	if u.UiaClient == nil {
		return common.NoUIAClientErr
	}
	if err := u.CheckSessionId(); err != nil {
		return err
	}

	id := pointerID(event.FingerID)
	state := u.pointerState[event.FingerID]
	actions := make([]map[string]interface{}, 0, 3)

	switch event.Type {
	case common.DOWN_TOUCH_EVENT:
		actions = append(actions, pointerMoveAction(event.Point, 0), pointerDownAction())
		state.point = event.Point
		state.isDown = true
	case common.MOVE_TOUCH_EVENT:
		if !state.isDown {
			actions = append(actions, pointerMoveAction(state.point, 0), pointerDownAction())
			state.isDown = true
		}
		actions = append(actions, pointerMoveAction(event.Point, waitDuration(event.WaitTime)))
		state.point = event.Point
	case common.UP_TOUCH_EVENT:
		if !state.isDown {
			return nil
		}
		actions = append(actions, pointerUpAction())
		state.isDown = false
	default:
		return fmt.Errorf("methodType error: %s", event.Type)
	}

	if event.WaitTime > 0 && event.Type != common.MOVE_TOUCH_EVENT {
		actions = append(actions, pauseAction(waitDuration(event.WaitTime)))
	}

	releaseActions := event.Type == common.UP_TOUCH_EVENT
	if err := u.performPointerSequence(id, actions, releaseActions); err != nil {
		return err
	}

	if state.isDown {
		u.pointerState[event.FingerID] = state
	} else {
		delete(u.pointerState, event.FingerID)
	}
	return nil
}

func (u *UIATouch) performPointerSequence(id string, actions []map[string]interface{}, release bool) error {
	if u.UiaClient == nil {
		return common.NoUIAClientErr
	}
	if err := u.CheckSessionId(); err != nil {
		return err
	}

	return u.performActions([]actionSource{newPointerSource(id, actions)}, release)
}

func (u *UIATouch) performActions(actions []actionSource, release bool) error {
	payload := actionRequest{Actions: actions}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	_, err = u.Request(http.MethodPost, u.SessionURL("/actions"), body)
	if err != nil {
		return err
	}

	if !release {
		return nil
	}

	_, err = u.Request(http.MethodDelete, u.SessionURL("/actions"), nil)
	if err != nil {
		// 某些直接启动的 UIA server 仅支持执行 W3C Actions，
		// 但没有暴露 DELETE /actions 路由。此时主动作已成功完成，
		// 不应因为释放路由缺失而让调用整体失败。
		if isUnsupportedReleaseActionsErr(err) {
			return nil
		}
		return err
	}
	return nil
}

func newPointerSource(id string, actions []map[string]interface{}) actionSource {
	return actionSource{
		Type: "pointer",
		ID:   id,
		Parameters: map[string]interface{}{
			"pointerType": "touch",
		},
		Actions: actions,
	}
}

func pointerID(fingerID int64) string {
	return fmt.Sprintf("finger-%d", fingerID)
}

func pointerMoveAction(point primitives.Point, duration int64) map[string]interface{} {
	return map[string]interface{}{
		"type":     "pointerMove",
		"duration": duration,
		"x":        point.X,
		"y":        point.Y,
	}
}

func pointerDownAction() map[string]interface{} {
	return map[string]interface{}{
		"type":   "pointerDown",
		"button": 0,
	}
}

func pointerUpAction() map[string]interface{} {
	return map[string]interface{}{
		"type":   "pointerUp",
		"button": 0,
	}
}

func pauseAction(duration int64) map[string]interface{} {
	return map[string]interface{}{
		"type":     "pause",
		"duration": duration,
	}
}

func waitDuration(waitTime int64) int64 {
	if waitTime <= 0 {
		return 0
	}
	return waitTime
}

func isUnsupportedReleaseActionsErr(err error) bool {
	if err == nil {
		return false
	}
	errText := err.Error()
	return errText == "uia request failed: unknown command: The requested resource could not be found, or a request was received using an HTTP method that is not supported by the mapped resource"
}

func (u *UIATouch) Close() error {
	u.UiaClient = nil
	u.pointerState = nil
	return nil
}
