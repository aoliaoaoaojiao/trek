package uctbandit

import (
	"fmt"
	"strings"
)

// BuildPageCluster 构造页面聚类标识。
// 页面名|可点击数量分桶|是否有列表|是否有输入框|标题模式
func BuildPageCluster(pageName string, clickableCount int, hasList, hasInput bool, title string) string {
	clickableBucket := classifyClickableCount(clickableCount)
	listFlag := "no_list"
	if hasList {
		listFlag = "list"
	}
	inputFlag := "no_input"
	if hasInput {
		inputFlag = "has_input"
	}
	titlePattern := classifyTitle(title)

	return fmt.Sprintf("%s|%s|%s|%s|%s",
		pageName,
		clickableBucket,
		listFlag,
		inputFlag,
		titlePattern,
	)
}

// classifyClickableCount 将可点击元素数量分桶。
func classifyClickableCount(count int) string {
	if count <= 5 {
		return "few_clickables"
	}
	return "many_clickables"
}

// classifyTitle 将页面标题归类。
func classifyTitle(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return "title_empty"
	}
	runeCount := len([]rune(title))
	if runeCount <= 6 {
		return "title_short"
	}
	return "title_long"
}

// PageFeatures 从 IState 中提取页面聚类所需的特征。
// 参数由 Agent 在构建候选动作时准备并传入。
type PageFeatures struct {
	PageName       string
	ClickableCount int
	HasList        bool
	HasInput       bool
	Title          string
}