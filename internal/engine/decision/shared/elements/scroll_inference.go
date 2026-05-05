package elements

import (
	"trek/internal/engine/core/types"
	"trek/logger"
)

// InferScrollableElements 为未被标记 scrollable 但包含足够多可点击后代的元素推断滚动能力。
// 使用后序遍历，优先标记最深层（最接近叶子）的合格容器，避免标记过于宽泛的祖先容器。
// 这解决了 Poco/Unity 游戏中 UI 节点不声明 ScrollRect 组件导致缺少 SCROLL 动作的问题。
func InferScrollableElements(root types.IElement, threshold int) {
	if threshold <= 0 || root == nil {
		return
	}
	inferScrollable(root, threshold)
}

// inferScrollable 递归推断元素是否应标记为可滚动。
// 返回值表示当前元素是否被推断为可滚动（用于父级决策：避免重复标记宽泛的祖先容器）。
func inferScrollable(elem types.IElement, threshold int) bool {
	if elem == nil {
		return false
	}

	children := elem.GetChildren()

	// 后序遍历：先处理子节点，确保优先标记最深层容器
	childInferred := false
	for _, child := range children {
		if inferScrollable(child, threshold) {
			childInferred = true
		}
	}

	// 已标记为可滚动（来自 Poco/XML 属性），跳过
	if elem.GetScrollType() != types.NONE {
		return false
	}

	// 叶子节点不可标记为滚动容器
	if len(children) == 0 {
		return false
	}

	// 如果子节点已被推断为可滚动，不再标记父级。
	// 策略：优先选择最深层（最接近叶子）的容器作为滚动区域，
	// 避免对同一个可点击区域产生多层冗余滚动动作。
	if childInferred {
		return false
	}

	// 统计可点击后代数量（递归）
	clickCount := countClickableDescendants(elem)
	if clickCount >= threshold {
		elem.SetScrollType("ALL")
		logger.Infof("scroll inference: element path=%s clickable_descendants=%d >= threshold=%d, marked as ALL scrollable",
			elem.GetPath(), clickCount, threshold)
		return true
	}

	return false
}

// countClickableDescendants 递归统计元素的所有可点击后代数量。
func countClickableDescendants(elem types.IElement) int {
	count := 0
	for _, child := range elem.GetChildren() {
		if child.GetClickable() {
			count++
		}
		count += countClickableDescendants(child)
	}
	return count
}
