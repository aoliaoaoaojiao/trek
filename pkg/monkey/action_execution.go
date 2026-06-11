package monkey

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"math"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"trek/internal/engine/config"
	"trek/internal/engine/core/types"
	"trek/internal/vision/coord"
	"trek/logger"
	"trek/pkg/coordinator"

	"github.com/beevik/etree"
)

var widgetXPathRegex = regexp.MustCompile(`(?:^|[,{ ])xpath:([^,}]+)`)
var widgetPathRegex = regexp.MustCompile(`(?:^|[,{ ])path:([^,}]+)`)

func (r *Runner) execute(cmd *types.ActionCommand, pageName string) error {
	// 检查动作是否命中黑名单区域（excluded_touch_areas），无论任何模式都不允许触控
	if r.isInBlackRect(cmd, pageName) {
		logger.Warnf("monkey: action %s blocked by excluded_touch_areas at %s", cmd.Act.String(), cmd.Pos.String())
		return fmt.Errorf("blocked by excluded_touch_areas")
	}

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
		if cmd.Text != "" {
			return r.tryInputText(cmd)
		}
		return nil
	case types.LONG_CLICK:
		pt, err := centerPoint(cmd.Pos)
		if err != nil {
			return err
		}
		if err = r.driver.LongClick(pt, r.cfg.LongClickDuration.Milliseconds()); err != nil {
			return err
		}
		if cmd.Text != "" {
			return r.tryInputText(cmd)
		}
		return nil
	case types.INPUT:
		pt, err := centerPoint(cmd.Pos)
		if err != nil {
			return err
		}
		if err = r.driver.Click(pt); err != nil {
			return err
		}
		cmd.Clear = true // INPUT 动作始终先清空再输入
		return r.tryInputText(cmd)
	case types.SCROLL_BOTTOM_UP, types.SCROLL_TOP_DOWN, types.SCROLL_LEFT_RIGHT, types.SCROLL_RIGHT_LEFT:
		if cmd.DragTo != nil {
			return r.dragByTarget(cmd)
		}
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

// isInBlackRect 检查动作是否命中黑名单区域。
// 对于点击类动作，检查中心点；对于滑动类动作，检查起始点。
func (r *Runner) isInBlackRect(cmd *types.ActionCommand, pageName string) bool {
	if cmd == nil || cmd.Pos.IsEmpty() {
		return false
	}

	cfgMgr := config.GetInstance()
	if cfgMgr == nil {
		return false
	}

	// 对于点击类动作，检查中心点是否在黑名单内
	switch cmd.Act {
	case types.CLICK, types.LONG_CLICK, types.INPUT:
		pt, err := centerPoint(cmd.Pos)
		if err != nil {
			return false
		}
		return cfgMgr.CheckPointIsInBlackRects(pageName, int(pt.X), int(pt.Y))
	case types.SCROLL_BOTTOM_UP, types.SCROLL_TOP_DOWN, types.SCROLL_LEFT_RIGHT, types.SCROLL_RIGHT_LEFT, types.SCROLL_BOTTOM_UP_N:
		// 对于滑动类动作，检查起始点是否在黑名单内
		// 取矩形的中心点作为滑动起始点
		pt, err := centerPoint(cmd.Pos)
		if err != nil {
			return false
		}
		return cfgMgr.CheckPointIsInBlackRects(pageName, int(pt.X), int(pt.Y))
	}

	return false
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

// cropScreenshotForEffectiveTouchArea 在截图进入决策管线前按 effective_touch_area 裁剪。
// 这样 goja/LLM 看到的是子区域截图，返回的归一化坐标经 applyEffectiveTouchArea 映射后位置正确。
func (r *Runner) cropScreenshotForEffectiveTouchArea(screenshot []byte) []byte {
	if len(screenshot) == 0 || len(r.cfg.EffectiveTouchAreas) == 0 {
		return screenshot
	}
	orientation := r.resolveScreenOrientation()
	if orientation == "" {
		return screenshot
	}
	area := matchEffectiveTouchArea(r.cfg.EffectiveTouchAreas, r.cfg.DeviceSerial, r.cfg.PackageName, orientation)
	if area == nil {
		return screenshot
	}
	return cropScreenshotByNormalizedRange(screenshot, area.Range)
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

func (r *Runner) dragByTarget(cmd *types.ActionCommand) error {
	if cmd == nil || cmd.DragTo == nil {
		return fmt.Errorf("拖拽终点为空")
	}
	start, err := centerPoint(cmd.Pos)
	if err != nil {
		return err
	}
	return r.driver.Swipe(start, *cmd.DragTo, r.cfg.ScrollSteps, r.cfg.ScrollDuration.Milliseconds())
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
	text := strings.TrimSpace(cmd.Text)
	if text == "" {
		text = r.randomText(6)
	}
	return r.driver.InputText(text, cmd.Clear)
}

var defaultInputCharset = "测试输入文本搜索登录注册设置确定取消返回删除编辑保存分享收藏关注点赞评论消息通知帮助反馈关于版本更新加载中请稍候成功失败重试跳过同意拒绝"

func (r *Runner) randomText(n int) string {
	if r.rng == nil {
		return "测试"
	}
	charset := r.cfg.InputCharset
	if charset == "" {
		charset = defaultInputCharset
	}
	runes := []rune(charset)
	b := make([]rune, n)
	for i := range b {
		b[i] = runes[r.rng.Intn(len(runes))]
	}
	return string(b)
}

// resolveScreenSize 从截图解码屏幕尺寸并缓存。
func (r *Runner) resolveScreenSize(screenshot []byte) (int, int) {
	if r.cachedScreenW > 0 && r.cachedScreenH > 0 {
		return r.cachedScreenW, r.cachedScreenH
	}
	if len(screenshot) == 0 {
		return 0, 0
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(screenshot))
	if err != nil {
		return 0, 0
	}
	r.cachedScreenW = cfg.Width
	r.cachedScreenH = cfg.Height
	// 设置屏幕分辨率到配置管理器，用于归一化坐标转换（如 excluded_touch_areas）
	if cfgMgr := config.GetInstance(); cfgMgr != nil {
		cfgMgr.SetScreenSize(cfg.Width, cfg.Height)
	}
	return r.cachedScreenW, r.cachedScreenH
}

// toAbsoluteCoordinates 将归一化 [0,1] 坐标转换为绝对像素坐标。
// 仅当 cmd.Pos 处于归一化空间时才执行转换；已经是绝对坐标的跳过。
func (r *Runner) toAbsoluteCoordinates(cmd *types.ActionCommand, screenshot []byte) {
	if cmd == nil || !isNormalizedRect(cmd.Pos) {
		return
	}
	w, h := r.resolveScreenSize(screenshot)
	if w <= 0 || h <= 0 {
		return
	}

	// DPR-aware coordinate conversion when device dimensions are configured
	if r.cfg.DeviceWidth > 0 && r.cfg.DeviceHeight > 0 {
		dprInfo := coord.DPRInfo{
			ScreenshotWidth:  w,
			ScreenshotHeight: h,
			DeviceWidth:      r.cfg.DeviceWidth,
			DeviceHeight:     r.cfg.DeviceHeight,
		}
		if rect := coord.ScreenshotToDevice(&types.Rect{
			Left: cmd.Pos.Left, Top: cmd.Pos.Top,
			Right: cmd.Pos.Right, Bottom: cmd.Pos.Bottom,
		}, dprInfo); rect != nil {
			cmd.Pos = *rect
		}
		if cmd.DragTo != nil {
			cmd.DragTo.X = cmd.DragTo.X * float64(r.cfg.DeviceWidth) / float64(w)
			cmd.DragTo.Y = cmd.DragTo.Y * float64(r.cfg.DeviceHeight) / float64(h)
		}
		return
	}

	// Fallback: direct screenshot dimension conversion
	fw, fh := float64(w), float64(h)
	cmd.Pos = types.Rect{
		Left:   cmd.Pos.Left * fw,
		Top:    cmd.Pos.Top * fh,
		Right:  cmd.Pos.Right * fw,
		Bottom: cmd.Pos.Bottom * fh,
	}
	if cmd.DragTo != nil {
		cmd.DragTo.X *= fw
		cmd.DragTo.Y *= fh
	}
}

func (r *Runner) markFailed(report *Report, record StepRecord, stepStart time.Time, before *coordinator.PageSnapshot, after *coordinator.PageSnapshot) {
	report.StepsTotal++
	report.StepsFailed++
	report.ConsecutiveFailures++
	r.appendRecord(report, record, stepStart, before, after)
}

func (r *Runner) appendRecord(report *Report, record StepRecord, stepStart time.Time, before *coordinator.PageSnapshot, after *coordinator.PageSnapshot) {
	if !r.cfg.KeepStepRecords {
		return
	}
	record.DurationMs = time.Since(stepStart).Milliseconds()
	if before != nil {
		record.BeforePageName = strings.TrimSpace(before.PageName)
		record.BeforeXML = before.XML
		record.BeforeElement = before.Element
		record.BeforeScreenshot = append([]byte(nil), before.Screenshot...)
	}
	if after != nil {
		record.AfterPageName = strings.TrimSpace(after.PageName)
		record.AfterXML = after.XML
		record.AfterElement = after.Element
		record.AfterScreenshot = append([]byte(nil), after.Screenshot...)
	}

	// 页面时序编号作为目录名（P1、P2…）
	beforePageDir := pageDirName(r.getOrAssignPageNum(record.BeforePageName))
	afterPageDir := pageDirName(r.getOrAssignPageNum(record.AfterPageName))

	// 实时写盘：截图 + XML 立即落盘，释放截图内存（XML 保留至报告生成阶段）
	if r.cfg.ArtifactDir != "" {
		if ref, err := writeStepSnapshotArtifacts(r.cfg.ArtifactDir, record, "before",
			beforePageDir, record.BeforeXML, record.BeforeScreenshot); err == nil && ref != nil {
			record.BeforeArtifactRef = ref
			// 保存原始截图 + 标注截图
			pageDirPath := filepath.Join(r.cfg.ArtifactDir, ref.PageDir)
			saveOriginalIfNew(pageDirPath, record.BeforeScreenshot)
			prefix := buildArtifactFilePrefix(record, "before")
			annotateAndSaveMarked(pageDirPath, prefix, record.BeforeScreenshot, record.Action, record.ActionTargetBounds, record.SwipeStart, record.SwipeEnd)
			writePageHashMarker(pageDirPath, record.BeforePageName)
		} else if err != nil {
			logger.Warnf("monkey step=%d 写入 before 产物失败: %v", record.Step, err)
		}
		if ref, err := writeStepSnapshotArtifacts(r.cfg.ArtifactDir, record, "after",
			afterPageDir, record.AfterXML, record.AfterScreenshot); err == nil && ref != nil {
			record.AfterArtifactRef = ref
			if afterPageDir != "" && afterPageDir != beforePageDir {
				afterDirPath := filepath.Join(r.cfg.ArtifactDir, ref.PageDir)
				writePageHashMarker(afterDirPath, record.AfterPageName)
			}
		} else if err != nil {
			logger.Warnf("monkey step=%d 写入 after 产物失败: %v", record.Step, err)
		}
		record.BeforeScreenshot = nil
		record.AfterScreenshot = nil
	}

	report.Records = append(report.Records, record)
}

// getOrAssignPageNum 返回页面名的时序编号（P1 对应 1，P2 对应 2…）。
// 首次遇到时自动分配下一个编号，页面名为空时返回 0。
func (r *Runner) getOrAssignPageNum(pageName string) int {
	pageName = strings.TrimSpace(pageName)
	if pageName == "" {
		return 0
	}
	if num, ok := r.pageNumCache[pageName]; ok {
		return num
	}
	r.pageNumSeq++
	r.pageNumCache[pageName] = r.pageNumSeq
	return r.pageNumSeq
}

// pageDirName 将编号转为目录名，0 返回空串。
func pageDirName(num int) string {
	if num <= 0 {
		return ""
	}
	return fmt.Sprintf("P%d", num)
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

type subImager interface {
	SubImage(r image.Rectangle) image.Image
}

// cropScreenshotByNormalizedRange 按归一化坐标范围裁剪截图。
// area 值域 [0,1]，全屏范围不做裁剪直接返回原图。
func cropScreenshotByNormalizedRange(screenshot []byte, area EffectiveTouchRange) []byte {
	if len(screenshot) == 0 {
		return screenshot
	}
	if area.Left <= 0 && area.Top <= 0 && area.Right >= 1 && area.Bottom >= 1 {
		return screenshot
	}
	img, format, err := image.Decode(bytes.NewReader(screenshot))
	if err != nil || format != "png" {
		return screenshot
	}
	si, ok := img.(subImager)
	if !ok {
		return screenshot
	}
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w <= 0 || h <= 0 {
		return screenshot
	}
	rect := image.Rect(
		int(area.Left*float64(w))+bounds.Min.X,
		int(area.Top*float64(h))+bounds.Min.Y,
		int(area.Right*float64(w))+bounds.Min.X,
		int(area.Bottom*float64(h))+bounds.Min.Y,
	)
	rect = rect.Intersect(bounds)
	if rect.Empty() {
		return screenshot
	}
	cropped := si.SubImage(rect)
	var buf bytes.Buffer
	if err := png.Encode(&buf, cropped); err != nil {
		return screenshot
	}
	return buf.Bytes()
}
