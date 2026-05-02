package monkey

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"trek/internal/engine/decision/shared/types"
	"trek/logger"

	"github.com/beevik/etree"
)

var widgetXPathRegex = regexp.MustCompile(`(?:^|[,{ ])xpath:([^,}]+)`)
var widgetPathRegex = regexp.MustCompile(`(?:^|[,{ ])path:([^,}]+)`)

func (r *Runner) execute(cmd *types.ActionCommand) error {
	switch cmd.Act {
	case types.NOP:
		return nil
	case types.CLICK:
		pt, err := centerPoint(cmd.Pos)
		if err != nil {
			return err
		}
		if err = r.driver.Click(pt); err != nil {
			return err
		}
		return r.tryInputText(cmd)
	case types.LONG_CLICK:
		pt, err := centerPoint(cmd.Pos)
		if err != nil {
			return err
		}
		if err = r.driver.LongClick(pt, r.cfg.LongClickDuration.Milliseconds()); err != nil {
			return err
		}
		return r.tryInputText(cmd)
	case types.SCROLL_BOTTOM_UP, types.SCROLL_TOP_DOWN, types.SCROLL_LEFT_RIGHT, types.SCROLL_RIGHT_LEFT:
		return r.swipeByAction(cmd.Pos, cmd.Act)
	case types.SCROLL_BOTTOM_UP_N:
		repeat := r.cfg.ScrollRepeat
		if repeat <= 0 {
			repeat = defaultScrollRepeat
		}
		for i := 0; i < repeat; i++ {
			if err := r.swipeByAction(cmd.Pos, types.SCROLL_BOTTOM_UP); err != nil {
				return err
			}
		}
		return nil
	case types.BACK:
		return r.driver.Back()
	case types.START:
		return r.driver.StartApp(r.cfg.PackageName)
	case types.RESTART:
		return r.driver.RestartApp(r.cfg.PackageName, false)
	case types.CLEAN_RESTART:
		return r.driver.RestartApp(r.cfg.PackageName, true)
	case types.ACTIVATE:
		return r.driver.ActivateApp(r.cfg.PackageName)
	default:
		return fmt.Errorf("暂不支持动作: %s", cmd.Act.String())
	}
}

func (r *Runner) normalizePocoScrollCommand(step int, cmd *types.ActionCommand, xml string) {
	if cmd == nil {
		return
	}
	if !strings.EqualFold(strings.TrimSpace(r.cfg.PageSourceType), "poco") {
		return
	}
	switch cmd.Act {
	case types.SCROLL_TOP_DOWN, types.SCROLL_BOTTOM_UP, types.SCROLL_LEFT_RIGHT, types.SCROLL_RIGHT_LEFT, types.SCROLL_BOTTOM_UP_N:
		if cmd.Pos.IsEmpty() {
			if rect, ok := resolvePocoScrollRectFromWidgetPath(cmd.WidgetInfo, xml); ok {
				cmd.Pos = *rect
				logger.Warnf("monkey step=%d action=%s bounds empty under poco, fallback to ancestor rect=%s", step, cmd.Act.String(), cmd.Pos.String())
				return
			}
			cmd.Pos = *types.NewRect(0, 0, 1, 1)
			logger.Warnf("monkey step=%d action=%s bounds empty under poco, fallback to normalized full-screen rect", step, cmd.Act.String())
		}
	}
}

func (r *Runner) applyEffectiveTouchArea(step int, cmd *types.ActionCommand, xml string) {
	if cmd == nil || len(r.cfg.EffectiveTouchAreas) == 0 {
		return
	}
	orientation := r.resolveScreenOrientation()
	if orientation == "" {
		logger.Debugf("monkey step=%d skip effective_touch_area: 无法从设备层获取实际朝向", step)
		return
	}
	area := matchEffectiveTouchArea(r.cfg.EffectiveTouchAreas, r.cfg.DeviceSerial, r.cfg.PackageName, orientation)
	if area == nil {
		return
	}
	switch cmd.Act {
	case types.CLICK, types.LONG_CLICK, types.SCROLL_TOP_DOWN, types.SCROLL_BOTTOM_UP, types.SCROLL_LEFT_RIGHT, types.SCROLL_RIGHT_LEFT, types.SCROLL_BOTTOM_UP_N:
	default:
		return
	}
	if cmd.Pos.IsEmpty() {
		return
	}
	if !isNormalizedRect(cmd.Pos) {
		return
	}
	oldRect := cmd.Pos
	mapped, ok := mapRectToEffectiveRange(cmd.Pos, area.Range)
	if !ok {
		return
	}
	cmd.Pos = mapped
	logger.Debugf(
		"monkey step=%d apply effective_touch_area scope=%s::%s orientation=%s from=%s to=%s",
		step,
		strings.TrimSpace(area.Serial),
		strings.TrimSpace(area.PackageName),
		orientation,
		oldRect.String(),
		cmd.Pos.String(),
	)
}

func (r *Runner) resolveScreenOrientation() ScreenOrientation {
	if orientation, err, updated := r.snapshotScreenOrientation(); updated {
		if err == nil {
			return orientation
		}
		logger.Debugf("monkey 获取缓存朝向失败: %v", err)
	}
	if r.driver != nil {
		rotation, err := r.driver.GetScreenRotation()
		if err == nil {
			return screenOrientationFromRotation(rotation)
		}
		logger.Debugf("monkey 获取设备实际朝向失败: %v", err)
	}
	return ""
}

func screenOrientationFromRotation(rotation int) ScreenOrientation {
	switch rotation {
	case 0:
		return ScreenOrientationPortrait
	case 1:
		return ScreenOrientationLandscapeLeft
	case 2:
		return ScreenOrientationPortraitReverse
	case 3:
		return ScreenOrientationLandscapeRight
	default:
		return ""
	}
}

func resolvePocoScrollRectFromWidgetPath(widgetInfo string, xml string) (*types.Rect, bool) {
	targetPath, ok := extractWidgetLocatorPath(widgetInfo)
	if !ok {
		return nil, false
	}

	doc := etree.NewDocument()
	if err := doc.ReadFromString(xml); err != nil {
		return nil, false
	}
	root := doc.Root()
	if root == nil {
		return nil, false
	}

	current := findElementByCompatiblePath(doc, root, targetPath)
	for current != nil {
		if rect, ok := parseRectFromBoundsValue(current.SelectAttrValue("bounds", "")); ok && !rect.IsEmpty() {
			return rect, true
		}
		current = current.Parent()
	}
	return nil, false
}

func extractWidgetLocatorPath(widgetInfo string) (string, bool) {
	if match := widgetXPathRegex.FindStringSubmatch(widgetInfo); len(match) >= 2 {
		xpath := strings.TrimSpace(match[1])
		if xpath != "" {
			return xpath, true
		}
	}
	if match := widgetPathRegex.FindStringSubmatch(widgetInfo); len(match) >= 2 {
		path := strings.TrimSpace(match[1])
		if path != "" {
			return path, true
		}
	}
	return "", false
}

func findElementByCompatiblePath(doc *etree.Document, root *etree.Element, targetPath string) *etree.Element {
	if doc == nil || root == nil {
		return nil
	}
	path := strings.TrimSpace(targetPath)
	if path == "" {
		return nil
	}
	if matched := doc.FindElement(path); matched != nil {
		return matched
	}
	if matched := root.FindElement(path); matched != nil {
		return matched
	}

	normalizedPath := strings.TrimPrefix(path, "/")
	if matched := root.FindElement(normalizedPath); matched != nil {
		return matched
	}

	rootPrefix := "/" + root.Tag
	if strings.HasPrefix(path, rootPrefix) {
		trimmed := strings.TrimPrefix(path, rootPrefix)
		if matched := root.FindElement(trimmed); matched != nil {
			return matched
		}
		trimmed = strings.TrimPrefix(trimmed, "[1]")
		if matched := root.FindElement(trimmed); matched != nil {
			return matched
		}
	}
	return nil
}

func parseRectFromBoundsValue(bounds string) (*types.Rect, bool) {
	text := strings.TrimSpace(bounds)
	if text == "" {
		return nil, false
	}
	parts := strings.Split(text, "][")
	if len(parts) != 2 {
		return nil, false
	}
	leftTop := strings.Trim(parts[0], "[]")
	rightBottom := strings.Trim(parts[1], "[]")
	lt := strings.Split(leftTop, ",")
	rb := strings.Split(rightBottom, ",")
	if len(lt) != 2 || len(rb) != 2 {
		return nil, false
	}
	left, err1 := strconv.ParseFloat(strings.TrimSpace(lt[0]), 64)
	top, err2 := strconv.ParseFloat(strings.TrimSpace(lt[1]), 64)
	right, err3 := strconv.ParseFloat(strings.TrimSpace(rb[0]), 64)
	bottom, err4 := strconv.ParseFloat(strings.TrimSpace(rb[1]), 64)
	if err1 != nil || err2 != nil || err3 != nil || err4 != nil {
		return nil, false
	}
	return types.NewRect(left, top, right, bottom), true
}

func formatTapPointLog(cmd *types.ActionCommand) string {
	if cmd == nil {
		return ""
	}
	if cmd.Act != types.CLICK && cmd.Act != types.LONG_CLICK {
		return ""
	}
	pt, err := centerPoint(cmd.Pos)
	if err != nil {
		return ""
	}
	return fmt.Sprintf(" tap_point=[%.3f,%.3f]", pt.X, pt.Y)
}

func formatSwipePointLog(cmd *types.ActionCommand) string {
	if cmd == nil {
		return ""
	}
	start, end, err := resolveSwipePoints(cmd.Pos, cmd.Act)
	if err != nil {
		return ""
	}
	return fmt.Sprintf(" swipe_start=[%.3f,%.3f] swipe_end=[%.3f,%.3f]", start.X, start.Y, end.X, end.Y)
}

func (r *Runner) swipeByAction(rect types.Rect, act types.ActionType) error {
	start, end, err := resolveSwipePoints(rect, act)
	if err != nil {
		return err
	}
	return r.driver.Swipe(start, end, r.cfg.ScrollSteps, r.cfg.ScrollDuration.Milliseconds())
}

func resolveSwipePoints(rect types.Rect, act types.ActionType) (types.Point, types.Point, error) {
	if rect.IsEmpty() {
		return types.Point{}, types.Point{}, fmt.Errorf("滑动区域为空")
	}
	switch act {
	case types.SCROLL_BOTTOM_UP:
		return pointByRatio(rect, 0.5, 0.82), pointByRatio(rect, 0.5, 0.22), nil
	case types.SCROLL_TOP_DOWN:
		return pointByRatio(rect, 0.5, 0.22), pointByRatio(rect, 0.5, 0.82), nil
	case types.SCROLL_LEFT_RIGHT:
		return pointByRatio(rect, 0.22, 0.5), pointByRatio(rect, 0.82, 0.5), nil
	case types.SCROLL_RIGHT_LEFT:
		return pointByRatio(rect, 0.82, 0.5), pointByRatio(rect, 0.22, 0.5), nil
	default:
		return types.Point{}, types.Point{}, fmt.Errorf("不支持的滑动动作: %s", act.String())
	}
}

func (r *Runner) tryInputText(cmd *types.ActionCommand) error {
	if strings.TrimSpace(cmd.Text) == "" {
		return nil
	}
	return r.driver.InputText(cmd.Text, cmd.Clear)
}

func (r *Runner) markFailed(report *Report, record StepRecord, stepStart time.Time) {
	report.StepsTotal++
	report.StepsFailed++
	report.ConsecutiveFailures++
	r.appendRecord(report, record, stepStart)
}

func (r *Runner) appendRecord(report *Report, record StepRecord, stepStart time.Time) {
	if !r.cfg.KeepStepRecords {
		return
	}
	record.DurationMs = time.Since(stepStart).Milliseconds()
	report.Records = append(report.Records, record)
}

func (r *Runner) sleepStep(ctx context.Context, d time.Duration) {
	if d <= 0 {
		return
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}

func (r *Runner) resolveStepDelay(cmd *types.ActionCommand) time.Duration {
	delay := r.cfg.StepInterval
	if cmd == nil || !isActionThrottleEnabled(r.cfg) {
		return delay
	}

	if cmd.WaitTime > 0 {
		waitDelay := time.Duration(cmd.WaitTime) * time.Millisecond
		if waitDelay > delay {
			delay = waitDelay
		}
	}
	if cmd.Throttle > 0 {
		throttleMs := int64(math.Ceil(float64(cmd.Throttle)))
		throttleDelay := time.Duration(throttleMs) * time.Millisecond
		if throttleDelay > delay {
			delay = throttleDelay
		}
	}

	if r.cfg.RandomizeThrottle && delay > 1*time.Millisecond && r.rng != nil {
		n := r.rng.Int63n(delay.Milliseconds()) + 1
		delay = time.Duration(n) * time.Millisecond
	}
	return delay
}
