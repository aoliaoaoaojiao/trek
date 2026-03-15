package touch

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
	types2 "trek/internal/core/types"
	"trek/pkg/driver/android/uia"
	"trek/pkg/driver/common"
)

var _ common.ITouch = (*UIATouch)(nil)

// NewUIATouch 创建UIATouch实例
func NewUIATouch(client *uia.UiaClient) *UIATouch {
	return &UIATouch{
		UiaClient: client,
	}
}

type UIATouch struct {
	*uia.UiaClient
}

func (u *UIATouch) Click(point types2.Point) error {
	if u.UiaClient == nil {
		return common.NoUIAClientErr
	}
	if err := u.CheckSessionId(); err != nil {
		return err
	}

	data := map[string]interface{}{
		"x": point.X,
		"y": point.Y,
	}
	body, err := json.Marshal(data)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/session/%s/appium/tap", u.RemoteUrl, u.SessionId)
	_, err = u.Request(http.MethodPost, url, body)
	return err
}

func (u *UIATouch) LongClick(point types2.Point, duration int64) error {
	if u.UiaClient == nil {
		return common.NoUIAClientErr
	}
	if err := u.CheckSessionId(); err != nil {
		return err
	}

	data := map[string]interface{}{
		"params": map[string]interface{}{
			"x":        point.X,
			"y":        point.Y,
			"duration": duration,
		},
	}
	body, err := json.Marshal(data)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/session/%s/touch/longclick", u.RemoteUrl, u.SessionId)
	_, err = u.Request(http.MethodPost, url, body)
	return err
}

// Swipe 使用uia滑动，如果设置了持续时间，则step不起作用，请注意，step 100 步，大约0.5s
func (u *UIATouch) Swipe(startPoint types2.Point, endPoint types2.Point, step int64, duration int64) error {
	if u.UiaClient == nil {
		return common.NoUIAClientErr
	}
	if err := u.CheckSessionId(); err != nil {
		return err
	}

	// example：100步时，滑动大约需要半秒，如果持续时间设置了，先以持续时间为优先级最高
	if duration > 0 {
		step = duration / 5
	}

	data := map[string]interface{}{
		"startX": startPoint.X,
		"startY": startPoint.Y,
		"endX":   endPoint.X,
		"endY":   endPoint.Y,
		"steps":  step,
	}
	body, err := json.Marshal(data)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/session/%s/touch/perform", u.RemoteUrl, u.SessionId)
	_, err = u.Request(http.MethodPost, url, body)
	return err
}

func (u *UIATouch) Pinch(centerPoint types2.Point, startDistance float64, endDistance float64, duration int64) error {
	if u.UiaClient == nil {
		return common.NoUIAClientErr
	}
	if err := u.CheckSessionId(); err != nil {
		return err
	}

	// 1. 参数合法性校验
	if duration <= 0 {
		return fmt.Errorf("pinch duration must be > 0, current: %d", duration)
	}
	// 起止距离一致，无需缩放
	if startDistance == endDistance {
		return nil
	}

	// 2. 初始化缩放参数：双指用FingerID=0和1（通用双指标识）
	finger0ID, finger1ID := int64(0), int64(1)
	centerX, centerY := centerPoint.X, centerPoint.Y

	// example：100步时，滑动大约需要半秒，如果持续时间设置了，先以持续时间为优先级最高
	step := duration / 5

	// 单步等待时间
	//stepWait := duration / step

	// 3. 计算双指起始/结束坐标（水平对称布局，最直观，可改为斜向）
	// 手指0：中心点左侧 | 手指1：中心点右侧 | Y轴与中心点一致，保证水平缩放
	startHalfDist := startDistance / 2
	endHalfDist := endDistance / 2
	// 起始坐标
	f0StartX, f0StartY := centerX-startHalfDist, centerY
	f1StartX, f1StartY := centerX+startHalfDist, centerY
	// 结束坐标
	f0EndX, f0EndY := centerX-endHalfDist, centerY
	f1EndX, f1EndY := centerX+endHalfDist, centerY

	// 4. 计算双指单步偏移量
	f0DX := (f0EndX - f0StartX) / float64(step)
	f0DY := (f0EndY - f0StartY) / float64(step)
	f1DX := (f1EndX - f1StartX) / float64(step)
	f1DY := (f1EndY - f1StartY) / float64(step)

	// 5. 构造双指触摸事件序列：双指DOWN → 多步同步MOVE → 双指UP
	var touchEvent1 []common.TouchEvent
	var touchEvent2 []common.TouchEvent
	// 第一步：双指同时按下起始位置（连续添加DOWN事件，实现同步按下）
	touchEvent1 = append(touchEvent1, common.TouchEvent{
		Point:    types2.Point{X: f0StartX, Y: f0StartY},
		Type:     common.DOWN_TOUCH_EVENT,
		FingerID: finger0ID,
	})
	touchEvent2 = append(touchEvent2, common.TouchEvent{
		Point:    types2.Point{X: f1StartX, Y: f1StartY},
		Type:     common.DOWN_TOUCH_EVENT,
		FingerID: finger1ID,
	})

	// 第二步：双指同步分步移动（最后一步直接到终点，保证缩放精准）
	f0CurrentX, f0CurrentY := f0StartX, f0StartY
	f1CurrentX, f1CurrentY := f1StartX, f1StartY
	for i := int64(0); i < step; i++ {
		// 最后一步强制到结束坐标
		if i == step-1 {
			f0CurrentX, f0CurrentY = f0EndX, f0EndY
			f1CurrentX, f1CurrentY = f1EndX, f1EndY
		} else {
			f0CurrentX += f0DX
			f0CurrentY += f0DY
			f1CurrentX += f1DX
			f1CurrentY += f1DY
		}

		// 构造双指同步移动事件（连续添加，底层连续发送实现同步）
		touchEvent1 = append(touchEvent1, common.TouchEvent{
			Point:    types2.Point{X: f0CurrentX, Y: f0CurrentY},
			Type:     common.MOVE_TOUCH_EVENT,
			FingerID: finger0ID,
		})

		touchEvent2 = append(touchEvent2, common.TouchEvent{
			Point:    types2.Point{X: f1CurrentX, Y: f1CurrentY},
			Type:     common.MOVE_TOUCH_EVENT,
			FingerID: finger1ID,
		})

	}

	// 第三步：双指同时松开（连续添加UP事件，实现同步松开）
	touchEvent1 = append(touchEvent1, common.TouchEvent{
		Type:     common.UP_TOUCH_EVENT,
		FingerID: finger0ID,
	})
	touchEvent2 = append(touchEvent2, common.TouchEvent{
		Type:     common.UP_TOUCH_EVENT,
		FingerID: finger1ID,
	})

	for i := range touchEvent1 {
		if err := u.TouchEvent(touchEvent1[i]); err != nil {
			return err
		}
		if err := u.TouchEvent(touchEvent2[i]); err != nil {
			return err
		}
	}

	return nil
}

// Drag 拖拽操作
func (u *UIATouch) Drag(startPoint types2.Point, endPoint types2.Point, duration int, elementId, destElId string) error {
	if u.UiaClient == nil {
		return common.NoUIAClientErr
	}
	if err := u.CheckSessionId(); err != nil {
		return err
	}

	steps := 100
	if duration > 0 {
		steps = duration / 5
	}

	data := map[string]interface{}{
		"startX":    startPoint.X,
		"startY":    startPoint.Y,
		"endX":      endPoint.X,
		"endY":      endPoint.Y,
		"steps":     steps,
		"elementId": elementId,
		"destElId":  destElId,
	}
	body, err := json.Marshal(data)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/session/%s/touch/drag", u.RemoteUrl, u.SessionId)
	_, err = u.Request(http.MethodPost, url, body)
	return err
}

func (u *UIATouch) TouchEvent(touchList ...common.TouchEvent) error {
	for _, touch := range touchList {
		err := u.TouchAction(touch)
		if err != nil {
			return err
		}
	}
	return nil
}

// TouchAction 触摸操作（down/up/move）
func (u *UIATouch) TouchAction(event common.TouchEvent) error {
	if u.UiaClient == nil {
		return common.NoUIAClientErr
	}
	if err := u.CheckSessionId(); err != nil {
		return err
	}

	var path string
	switch event.Type {
	case common.DOWN_TOUCH_EVENT:
		path = "/touch/down"
	case common.UP_TOUCH_EVENT:
		path = "/touch/up"
	case common.MOVE_TOUCH_EVENT:
		path = "/touch/move"
	default:
		return fmt.Errorf("methodType error: %s", event.Type)
	}

	data := map[string]interface{}{
		"params": map[string]interface{}{
			"x": event.X,
			"y": event.Y,
		},
	}
	body, err := json.Marshal(data)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/session/%s%s", u.RemoteUrl, u.SessionId, path)
	_, err = u.Request(http.MethodPost, url, body)
	if event.WaitTime > 0 {
		time.Sleep(time.Duration(event.WaitTime) * time.Second)
	}
	return err
}

func (u *UIATouch) Close() error {
	u.UiaClient = nil
	return nil
}
