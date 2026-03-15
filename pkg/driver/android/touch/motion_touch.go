package touch

import (
	"bytes"
	_ "embed"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
	"trek/pkg/driver/android/gadb"
	"trek/pkg/driver/common"

	types2 "trek/internal/core/types"
)

// source: https://github.com/aoliaoaoaojiao/AndroidTouch, build output to debug apk
//
//go:embed bin/touch.apk
var touchBytes []byte

var _ common.ITouch = (*MotionTouch)(nil)

const touchToolPath = "/data/local/tmp/atouch.jar"

// MotionTouch 基于AndroidTouch工具的触摸实现，通过shell循环连接发送触摸事件
type MotionTouch struct {
	lock          sync.Mutex
	shellLoopConn net.Conn
}

// NewMotionTouch 初始化触摸工具，推送apk并建立shell循环连接
func NewMotionTouch(device *gadb.Device) *MotionTouch {
	var err error
	// 推送触摸工具到设备
	err = device.Push(bytes.NewReader(touchBytes), touchToolPath, time.Now())
	if err != nil {
		panic(fmt.Sprintf("push touch apk failed: %v", err))
	}
	// 启动AndroidTouch并建立socket连接
	conn, err := device.RunShellLoopCommandSock(fmt.Sprintf(
		"CLASSPATH=%s app_process / com.aoliaoaojiao.AndroidTouch.Run v2.2",
		touchToolPath))
	if err != nil {
		panic(fmt.Sprintf("start touch tool failed: %v", err))
	}

	var initWg sync.WaitGroup

	initWg.Add(1)
	// 验证工具启动成功
	go func() {
		defer initWg.Done()
		byteDatas := make([]byte, 1024)
		n, err := conn.Read(byteDatas)
		if err != nil {
			fmt.Printf("touch tool read init data failed: %v\n", err)
			return
		}
		if !strings.Contains(string(byteDatas[:n]), "设备") {
			fmt.Printf("touch tool init failed, response: %s\n", string(byteDatas[:n]))
			return
		}
	}()
	initWg.Wait()

	return &MotionTouch{shellLoopConn: conn}
}

// Close 关闭shell循环连接
func (m *MotionTouch) Close() error {
	if m.shellLoopConn != nil {
		return m.shellLoopConn.Close()
	}
	return nil
}

// Click 模拟单指点击：DOWN→UP
func (m *MotionTouch) Click(point types2.Point) error {
	// 绑定手指ID为0（单指默认）
	clickEventDown := common.TouchEvent{
		Point:    point,
		Type:     common.DOWN_TOUCH_EVENT,
		FingerID: 0,
	}
	clickEventUp := common.TouchEvent{
		Point:    point,
		Type:     common.UP_TOUCH_EVENT,
		FingerID: 0,
	}
	return m.TouchEvent(clickEventDown, clickEventUp)
}

// LongClick 模拟单指长按：DOWN→等待duration→UP
func (m *MotionTouch) LongClick(point types2.Point, duration int64) error {
	longClickEventDown := common.TouchEvent{
		Point:    point,
		Type:     common.DOWN_TOUCH_EVENT,
		WaitTime: duration,
		FingerID: 0,
	}
	longClickEventUp := common.TouchEvent{
		Point:    point,
		Type:     common.UP_TOUCH_EVENT,
		FingerID: 0,
	}
	return m.TouchEvent(longClickEventDown, longClickEventUp)
}

// Swipe 模拟单指分步滑动：DOWN→多步MOVE→UP
// startPoint: 滑动起点 | endPoint: 滑动终点 | step: 滑动步数 | duration: 总滑动时长(毫秒)
func (m *MotionTouch) Swipe(startPoint types2.Point, endPoint types2.Point, step int64, duration int64) error {
	// 1. 参数合法性校验
	if step <= 0 {
		return fmt.Errorf("swipe step must be > 0, current: %d", step)
	}
	if duration <= 0 {
		return fmt.Errorf("swipe duration must be > 0, current: %d", duration)
	}
	// 起点终点一致，无需滑动
	if startPoint.X == endPoint.X && startPoint.Y == endPoint.Y {
		return nil
	}

	// 2. 初始化滑动参数：单指默认FingerID=0
	fingerID := int64(0)
	startX, startY := startPoint.X, startPoint.Y
	endX, endY := endPoint.X, endPoint.Y
	// 总偏移量
	totalDX := endX - startX
	totalDY := endY - startY
	// 单步偏移量（平均拆分）
	stepDX := totalDX / float64(step)
	stepDY := totalDY / float64(step)
	// 单步等待时间（总时长平均拆分，毫秒）
	stepWait := duration / step

	// 3. 构造触摸事件序列：DOWN → 多步MOVE → UP
	var touchEvents []common.TouchEvent
	// 第一步：按下起点
	touchEvents = append(touchEvents, common.TouchEvent{
		Point:    types2.Point{X: startX, Y: startY},
		Type:     common.DOWN_TOUCH_EVENT,
		FingerID: fingerID,
	})

	// 第二步：分步移动（最后一步直接到终点，避免浮点累加精度误差）
	currentX, currentY := startX, startY
	for i := int64(0); i < step; i++ {
		// 最后一步强制到终点，保证滑动精准
		if i == step-1 {
			currentX, currentY = endX, endY
		} else {
			currentX += stepDX
			currentY += stepDY
		}
		// 构造移动事件，最后一步无需等待（后续直接UP）
		moveEvent := common.TouchEvent{
			Point:    types2.Point{X: currentX, Y: currentY},
			Type:     common.MOVE_TOUCH_EVENT,
			FingerID: fingerID,
		}
		if i != step-1 {
			moveEvent.WaitTime = stepWait
		}
		touchEvents = append(touchEvents, moveEvent)
	}

	// 第三步：松开手指
	touchEvents = append(touchEvents, common.TouchEvent{
		Type:     common.UP_TOUCH_EVENT,
		FingerID: fingerID,
	})

	// 4. 执行触摸事件序列
	return m.TouchEvent(touchEvents...)
}

// Pinch 模拟双指捏合/缩放：双指DOWN→多步同步MOVE→双指UP
// centerPoint: 缩放中心点 | startDistance: 双指起始距离(像素) | endDistance: 双指结束距离(像素) | duration: 总时长(毫秒)
func (m *MotionTouch) Pinch(centerPoint types2.Point, startDistance float64, endDistance float64, duration int64) error {
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
	// 分步数（内部定10步，保证缩放流畅，可后续扩展为入参）
	step := int64(10)
	// 单步等待时间
	stepWait := duration / step

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

		// 非最后一步，添加等待时间（双指移动后等待，保证速度均匀）
		if i != step-1 {
			// 给最后一个MOVE事件加等待，避免无意义的空事件
			touchEvent1[len(touchEvent1)-1].WaitTime = stepWait
			touchEvent2[len(touchEvent2)-1].WaitTime = stepWait
		}
	}

	// 第三步：双指同时松开（连续添加UP事件，实现同步松开）
	touchEvent1 = append(touchEvent1, common.TouchEvent{
		Type:     common.UP_TOUCH_EVENT,
		FingerID: finger0ID,
		WaitTime: stepWait,
	})
	touchEvent2 = append(touchEvent2, common.TouchEvent{
		Type:     common.UP_TOUCH_EVENT,
		FingerID: finger1ID,
		WaitTime: stepWait,
	})

	//waitTouch := sync.WaitGroup{}
	//waitTouch.Add(2)
	//
	//var err1, err2 error
	//
	//go func() {
	//	defer waitTouch.Done()
	//	// 6. 同步执行双指事件：交替发送，保证双指同步
	//	for i := range touchEvent1 {
	//		if err1 = m.TouchEvent(touchEvent1[i]); err1 != nil {
	//			break
	//		}
	//	}
	//}()
	//
	//go func() {
	//	defer waitTouch.Done()
	//	// 6. 同步执行双指事件：交替发送，保证双指同步
	//	for i := range touchEvent2 {
	//		if err2 = m.TouchEvent(touchEvent2[i]); err2 != nil {
	//			break
	//		}
	//	}
	//}()
	//
	//waitTouch.Wait()
	//
	//if err1 != nil {
	//	return err1
	//}
	//
	//if err2 != nil {
	//	return err2
	//}

	// 6. 同步执行双指事件：交替发送，保证双指同步
	for i := range touchEvent1 {
		if err := m.TouchEvent(touchEvent1[i]); err != nil {
			return err
		}
		if err := m.TouchEvent(touchEvent2[i]); err != nil {
			return err
		}
	}
	return nil
}

// TouchEvent 执行触摸事件序列，自动判断相对/绝对坐标，按顺序执行
func (m *MotionTouch) TouchEvent(touchList ...common.TouchEvent) error {
	for _, touch := range touchList {
		var err error
		// 修复bug：正确判断Airtest相对坐标（0≤X≤1 且 0≤Y≤1）
		if touch.X >= 0 && touch.X <= 1 && touch.Y >= 0 && touch.Y <= 1 {
			err = m.touchAirtestPoint(touch) // 相对坐标走airtest命令
		} else {
			err = m.touchNormalPoint(touch) // 绝对坐标走normal命令
		}
		if err != nil {
			return fmt.Errorf("execute touch event failed: %v, event: %+v", err, touch)
		}
		// 事件等待：非0则休眠对应毫秒
		if touch.WaitTime > 0 {
			time.Sleep(time.Duration(touch.WaitTime) * time.Millisecond)
		}
	}
	return nil
}

// touchNormalPoint 发送普通绝对坐标触摸事件
func (m *MotionTouch) touchNormalPoint(touchEvent common.TouchEvent) error {
	var cmd string
	switch touchEvent.Type {
	case common.DOWN_TOUCH_EVENT, common.MOVE_TOUCH_EVENT:
		// DOWN/MOVE：需要坐标+手指ID
		cmd = fmt.Sprintf("touch %s %d %d %d\n",
			touchEvent.Type, int64(touchEvent.X), int64(touchEvent.Y), touchEvent.FingerID)
	default:
		// UP：仅需要手指ID
		cmd = fmt.Sprintf("touch %s %d\n", common.UP_TOUCH_EVENT, touchEvent.FingerID)
	}
	m.lock.Lock()
	_, err := m.shellLoopConn.Write([]byte(cmd))
	m.lock.Unlock()
	if err != nil {
		return fmt.Errorf("write normal touch cmd failed: %v, cmd: %s", err, cmd)
	}
	return nil
}

// touchAirtestPoint 发送Airtest相对坐标触摸事件（0-1）
func (m *MotionTouch) touchAirtestPoint(touchEvent common.TouchEvent) error {
	var cmd string
	switch touchEvent.Type {
	case common.DOWN_TOUCH_EVENT, common.MOVE_TOUCH_EVENT:
		// DOWN/MOVE：需要坐标+手指ID
		cmd = fmt.Sprintf("airtest %s %f %f %d\n",
			touchEvent.Type, touchEvent.X, touchEvent.Y, touchEvent.FingerID)
	default:
		// UP：仅需要手指ID
		cmd = fmt.Sprintf("airtest %s %d\n", common.UP_TOUCH_EVENT, touchEvent.FingerID)
	}
	m.lock.Lock()
	_, err := m.shellLoopConn.Write([]byte(cmd))
	m.lock.Unlock()
	if err != nil {
		return fmt.Errorf("write airtest touch cmd failed: %v, cmd: %s", err, cmd)
	}
	return nil
}
