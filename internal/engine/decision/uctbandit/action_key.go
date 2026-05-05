package uctbandit

import (
	"fmt"
	"strings"
	"unicode"

	"trek/internal/engine/core/types"
)

// BuildActionKey 为动作生成稳定的归一化 key。
// actionType + widgetType + resourceID_or_empty + textPattern + positionBucket
func BuildActionKey(actionType types.ActionType, widgetType, resourceID, text string, normX, normY float64) string {
	textPattern := classifyTextPattern(text)
	positionBucket := classifyPosition(normX, normY)
	resID := resourceID
	if resID == "" {
		resID = "no_res_id"
	}

	return fmt.Sprintf("%s|%s|%s|%s|%s",
		actionType.String(),
		widgetType,
		resID,
		textPattern,
		positionBucket,
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
