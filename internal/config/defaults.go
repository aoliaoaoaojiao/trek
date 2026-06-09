// Package config 定义 Trek 全局默认配置常量。
// 前端通过 /api/defaults 获取，后端各模块直接引用。
package config

// 页面识别相关默认值
const (
	// DefaultScrollInferThreshold 滚动推断阈值：可点击元素数量 >= 阈值时标记为可滚动。
	DefaultScrollInferThreshold = 5

	// DefaultImageSimilaritySSIMThreshold 图片相似度 SSIM 阈值，范围 0~1，越接近 1 越严格。
	DefaultImageSimilaritySSIMThreshold = 0.995

	// DefaultImageFingerprintHammingThreshold 图片指纹 Hamming 距离阈值。
	// 对于默认 2 region（512 bit）指纹，阈值 10 表示允许约 2% 的 bit 差异。
	// 状态栏时间/电量变化通常导致 10-20 bits 差异，阈值 10 可以过滤这些噪声。
	DefaultImageFingerprintHammingThreshold = 10

	// DefaultPageControlCacheTTLSeconds OCR/LLM 页面理解结果缓存有效期（秒）。
	// 动态 TTL 公式：effectiveTTL = baseTTL × (1 + ln(hitCount))，最大不超过 3 天。
	// 默认 4 小时，高频访问页面自动延长：10次≈13h，100次≈22h，1000次≈32h。
	DefaultPageControlCacheTTLSeconds = 14400 // 4小时

	// DefaultExploreOCRTimeoutMs OCR 请求超时（毫秒）。
	DefaultExploreOCRTimeoutMs = 10000

	// DefaultLLMTimeoutMs LLM 请求超时（毫秒）。
	DefaultLLMTimeoutMs = 15000
)

// UCT-Bandit reward 相关默认值
const (
	// DefaultNewStateReward 首次到达新状态的奖励。
	DefaultNewStateReward = 5.0

	// DefaultNewEdgeReward 首次发现新边的奖励。
	DefaultNewEdgeReward = 3.0

	// DefaultStructureChangeReward 页面名相同但结构变化的奖励。
	DefaultStructureChangeReward = 2.0

	// DefaultNoOpPenalty 页面无变化的惩罚。
	DefaultNoOpPenalty = -1.0

	// DefaultShortLoopPenalty 短环惩罚。
	DefaultShortLoopPenalty = -2.0

	// DefaultTwoStateLoopPenalty 双状态往返惩罚（A↔B）。
	DefaultTwoStateLoopPenalty = -3.0

	// DefaultEdgeRepeatPenalty 重复边惩罚。
	DefaultEdgeRepeatPenalty = -1.0

	// DefaultEdgeRepeatThreshold 重复边阈值，超过后开始惩罚。
	DefaultEdgeRepeatThreshold = 2

	// DefaultEmptyResultPenalty 空结果/连续无效的惩罚。
	DefaultEmptyResultPenalty = -3.0

	// DefaultShortLoopWindow 短环检测窗口大小。
	DefaultShortLoopWindow = 3

	// DefaultStagnationThreshold 停滞阈值。
	DefaultStagnationThreshold = 2
)

// UCT-Bandit agent 相关默认值
const (
	// DefaultBackPenalty Back 动作惩罚，减少返回旧页面倾向。
	DefaultBackPenalty = -1.0

	// DefaultEscapeBonus 逃逸加成。
	DefaultEscapeBonus = 3.0

	// DefaultExploreRatio ε-贪心探索率。
	DefaultExploreRatio = 0.15

	// DefaultActionCooldownPenalty 同状态同动作近期重复惩罚。
	DefaultActionCooldownPenalty = 0.8

	// DefaultRecentActionWindow 近期动作窗口大小。
	DefaultRecentActionWindow = 6

	// DefaultLoopEscapeExploreBoost 检测到回环时的探索增益。
	DefaultLoopEscapeExploreBoost = 0.25
)
