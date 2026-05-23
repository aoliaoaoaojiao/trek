package uctbandit

import (
	"fmt"
	"strings"
	"unicode"

	"trek/internal/engine/core/types"
)

// BuildActionKey 为动作生成稳定的归一化 key。
// Click: 同一 resourceID 的控件 = 同一动作，不区分点击位置
// Scroll: 按方向区分（上/下/左/右），不区分起始位置
func BuildActionKey(actionType types.ActionType, widgetType, resourceID, text string, normX, normY float64) string {
	resID := resourceID
	if resID == "" {
		resID = "no_res_id"
	}

	// Scroll 动作按方向区分
	if isScrollAction(actionType) {
		direction := classifyScrollDirection(actionType)
		return fmt.Sprintf("%s|%s|%s",
			actionType.String(),
			resID,
			direction,
		)
	}

	// Click 等动作：同一控件 = 同一动作
	textPattern := classifyTextPattern(text)
	return fmt.Sprintf("%s|%s|%s|%s",
		actionType.String(),
		widgetType,
		resID,
		textPattern,
	)
}

// BuildArmKey 为 Bandit arm 生成泛化 key。
// pageCluster + widgetType + actionType + positionBucket
func BuildArmKey(pageCluster, widgetType string, actionType types.ActionType, positionBucket string) string {
	return fmt.Sprintf("%s|%s|%s|%s",
		pageCluster,
		widgetType,
		actionType.String(),
		positionBucket,
	)
}

// classifyTextPattern 将控件文本归一化为稳定的模式。
func classifyTextPattern(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return "empty"
	}

	// 纯数字
	if isNumberLike(text) {
		return "number_like"
	}

	// 关键词检测优先（短文本也可能包含关键关键词）
	lower := strings.ToLower(text)
	if strings.Contains(lower, "更多") || strings.Contains(lower, "more") {
		return "more_like"
	}
	if strings.Contains(lower, "详情") || strings.Contains(lower, "detail") {
		return "detail_like"
	}

	// 长文本
	runeCount := len([]rune(text))
	if runeCount > 15 {
		return "long_text"
	}

	return "short_text"
}

// classifyPosition 将相对坐标分桶。
func classifyPosition(normX, normY float64) string {
	// 分为 9 宫格
	xZone := "left"
	if normX > 0.66 {
		xZone = "right"
	} else if normX > 0.33 {
		xZone = "center"
	}

	yZone := "top"
	if normY > 0.66 {
		yZone = "bottom"
	} else if normY > 0.33 {
		yZone = "middle"
	}

	return yZone + "_" + xZone
}

// isNumberLike 检测文本是否类似数字。
func isNumberLike(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	digitCount := 0
	for _, r := range text {
		if unicode.IsDigit(r) {
			digitCount++
		} else if r == '.' || r == ',' || r == '%' || r == ' ' {
			// 允许数字中的常见符号
		} else {
			return false
		}
	}
	return digitCount > 0
}

// isScrollAction 判断是否为滑动类动作。
func isScrollAction(actionType types.ActionType) bool {
	switch actionType {
	case types.SCROLL_BOTTOM_UP, types.SCROLL_TOP_DOWN,
		types.SCROLL_LEFT_RIGHT, types.SCROLL_RIGHT_LEFT:
		return true
	}
	return false
}

// classifyScrollDirection 将滑动动作归类为方向。
func classifyScrollDirection(actionType types.ActionType) string {
	switch actionType {
	case types.SCROLL_BOTTOM_UP:
		return "scroll_up"
	case types.SCROLL_TOP_DOWN:
		return "scroll_down"
	case types.SCROLL_LEFT_RIGHT:
		return "scroll_right"
	case types.SCROLL_RIGHT_LEFT:
		return "scroll_left"
	default:
		return "scroll_unknown"
	}
}
