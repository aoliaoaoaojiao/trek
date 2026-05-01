package monkey

import (
	"fmt"
	"hash/fnv"
	"strings"
	"trek/internal/engine/decision/shared/types"
	"trek/pkg/session"
)

// blockDetector 检测遍历过程中的卡死模式：滚动无变化、同页面无移动、两状态乒乓、高访问低收益。
//
// 签名分为两层：
//   - 骨架签名（pageFingerprintName）：仅基于 XML 结构，忽略动态文本/内容。
//     同一界面动态加载数据后骨架不变，用于判断"是否在同一个界面"。
//   - 内容签名（pageSignature）：基于页面名+XML 全文哈希，用于判断内容是否有实质变化。
type blockDetector struct {
	noChangeThreshold         int
	twoStateLoopThreshold     int
	highVisitThreshold        int
	lowRewardWindow           int
	consecutiveNoChangeScroll int
	consecutiveSamePageNoMove int
	consecutiveTwoStateLoops  int
	recentAfterSigs           []string // 骨架签名，用于两状态乒乓检测（A→B→A→B）
	recentObservedSigs        []string // 骨架签名，用于高访问低收益检测
	pageVisitCount            map[string]int
	lastReason                string
	imageSignatureFunc        func(screenshot []byte) string // 可选，XML 不可用时用图片签名替代
}

func newBlockDetector(noChangeThreshold int, twoStateLoopThreshold int, highVisitThreshold int, lowRewardWindow int) *blockDetector {
	if noChangeThreshold <= 0 {
		noChangeThreshold = defaultBlockNoChangeThreshold
	}
	if twoStateLoopThreshold <= 0 {
		twoStateLoopThreshold = defaultTwoStateLoopThreshold
	}
	if highVisitThreshold <= 0 {
		highVisitThreshold = defaultHighVisitThreshold
	}
	if lowRewardWindow <= 0 {
		lowRewardWindow = defaultLowRewardWindow
	}
	return &blockDetector{
		noChangeThreshold:  noChangeThreshold,
		twoStateLoopThreshold: twoStateLoopThreshold,
		highVisitThreshold: highVisitThreshold,
		lowRewardWindow:    lowRewardWindow,
		recentAfterSigs:    make([]string, 0, 8),
		recentObservedSigs: make([]string, 0, 16),
		pageVisitCount:     make(map[string]int),
	}
}

func (d *blockDetector) withImageSignatureFunc(f func([]byte) string) *blockDetector {
	d.imageSignatureFunc = f
	return d
}

// resolveSkeleton 解析骨架签名：优先 XML 结构指纹，XML 不可用时降级到图片签名。
func (d *blockDetector) resolveSkeleton(xml string, screenshot []byte) string {
	sig := pageFingerprintName(xml)
	if sig != "" && sig != "UnknownPage" {
		return sig
	}
	if d != nil && d.imageSignatureFunc != nil && len(screenshot) > 0 {
		if imgSig := d.imageSignatureFunc(screenshot); imgSig != "" {
			return imgSig
		}
	}
	return sig
}

func (d *blockDetector) Observe(cmd *types.ActionCommand, before session.PageSnapshot, after *session.PageSnapshot) bool {
	if d == nil || cmd == nil || after == nil {
		d.Reset()
		return false
	}

	triggerNoChangeScroll := false
	triggerSamePageNoMove := false
	triggerTwoStateLoop := false
	triggerHighVisitLowGain := false

	// 骨架签名：仅基于 XML 结构，同一界面动态加载数据后不变
	// XML 不可用时降级到图片签名
	beforeSkeleton := d.resolveSkeleton(before.XML, before.Screenshot)
	afterSkeleton := d.resolveSkeleton(after.XML, after.Screenshot)
	// 内容签名：基于页面名+XML 全文，用于检测内容是否有实质变化
	afterContent := pageSignature(after.PageName, after.XML)

	if !isBlockDetectorIgnoredAction(cmd.Act) && afterSkeleton != "" {
		d.pageVisitCount[afterSkeleton]++
		d.pushObservedSignature(afterSkeleton)
	}

	// 滚动无变化：骨架相同即为同一界面（内容可变，如动态加载）
	if cmd.IsScrollAction() && beforeSkeleton != "" && beforeSkeleton == afterSkeleton {
		d.consecutiveNoChangeScroll++
	} else {
		d.consecutiveNoChangeScroll = 0
	}
	if d.consecutiveNoChangeScroll >= d.noChangeThreshold {
		triggerNoChangeScroll = true
		d.lastReason = blockReasonScrollNoChange
	}

	// 同页面无移动：骨架和内容都相同才算"什么都没发生"
	if !isBlockDetectorIgnoredAction(cmd.Act) && !cmd.IsScrollAction() && beforeSkeleton != "" && beforeSkeleton == afterSkeleton {
		beforeContent := pageSignature(before.PageName, before.XML)
		if beforeContent == afterContent {
			d.consecutiveSamePageNoMove++
		} else {
			d.consecutiveSamePageNoMove = 0
		}
	} else {
		d.consecutiveSamePageNoMove = 0
	}
	if d.consecutiveSamePageNoMove >= d.noChangeThreshold {
		triggerSamePageNoMove = true
		d.lastReason = blockReasonSamePageNoChange
	}

	// 两状态乒乓：用骨架签名检测 A→B→A→B 模式
	if !isBlockDetectorIgnoredAction(cmd.Act) && beforeSkeleton != "" && afterSkeleton != "" && beforeSkeleton != afterSkeleton {
		d.pushAfterSignature(afterSkeleton)
		if d.isTailABAB() {
			d.consecutiveTwoStateLoops++
		} else {
			d.consecutiveTwoStateLoops = 0
		}
	} else {
		d.consecutiveTwoStateLoops = 0
	}
	if d.consecutiveTwoStateLoops >= d.twoStateLoopThreshold {
		triggerTwoStateLoop = true
		d.lastReason = blockReasonTwoStatePingPong
	}

	if d.isHighVisitLowReward(afterSkeleton) {
		triggerHighVisitLowGain = true
		d.lastReason = blockReasonHighVisitLowGain
	}

	if triggerTwoStateLoop {
		d.lastReason = blockReasonTwoStatePingPong
	} else if triggerNoChangeScroll {
		d.lastReason = blockReasonScrollNoChange
	} else if triggerSamePageNoMove {
		d.lastReason = blockReasonSamePageNoChange
	} else if triggerHighVisitLowGain {
		d.lastReason = blockReasonHighVisitLowGain
	}

	return triggerNoChangeScroll || triggerSamePageNoMove || triggerTwoStateLoop || triggerHighVisitLowGain
}

func (d *blockDetector) Reset() {
	if d == nil {
		return
	}
	d.consecutiveNoChangeScroll = 0
	d.consecutiveSamePageNoMove = 0
	d.consecutiveTwoStateLoops = 0
	d.recentAfterSigs = d.recentAfterSigs[:0]
	d.recentObservedSigs = d.recentObservedSigs[:0]
	clear(d.pageVisitCount)
	d.lastReason = ""
}

func (d *blockDetector) LastReason() string {
	if d == nil || strings.TrimSpace(d.lastReason) == "" {
		return "unknown"
	}
	return d.lastReason
}

func (d *blockDetector) pushAfterSignature(sig string) {
	if d == nil || strings.TrimSpace(sig) == "" {
		return
	}
	d.recentAfterSigs = append(d.recentAfterSigs, sig)
	if len(d.recentAfterSigs) > 8 {
		d.recentAfterSigs = d.recentAfterSigs[len(d.recentAfterSigs)-8:]
	}
}

func (d *blockDetector) pushObservedSignature(sig string) {
	if d == nil || strings.TrimSpace(sig) == "" {
		return
	}
	d.recentObservedSigs = append(d.recentObservedSigs, sig)
	if len(d.recentObservedSigs) > 16 {
		d.recentObservedSigs = d.recentObservedSigs[len(d.recentObservedSigs)-16:]
	}
}

func (d *blockDetector) isTailABAB() bool {
	if d == nil || len(d.recentAfterSigs) < 4 {
		return false
	}
	n := len(d.recentAfterSigs)
	a := d.recentAfterSigs[n-4]
	b := d.recentAfterSigs[n-3]
	c := d.recentAfterSigs[n-2]
	e := d.recentAfterSigs[n-1]
	return a == c && b == e && a != b
}

func (d *blockDetector) isHighVisitLowReward(afterSkeleton string) bool {
	if d == nil || strings.TrimSpace(afterSkeleton) == "" {
		return false
	}
	if d.pageVisitCount[afterSkeleton] < d.highVisitThreshold {
		return false
	}
	if d.lowRewardWindow <= 0 || len(d.recentObservedSigs) < d.lowRewardWindow {
		return false
	}
	start := len(d.recentObservedSigs) - d.lowRewardWindow
	tail := d.recentObservedSigs[start:]
	unique := make(map[string]struct{}, len(tail))
	for _, sig := range tail {
		if sig == "" {
			continue
		}
		unique[sig] = struct{}{}
	}
	return len(unique) <= 2
}

func isBlockDetectorIgnoredAction(act types.ActionType) bool {
	return act == types.NOP || act == types.START || act == types.RESTART || act == types.CLEAN_RESTART || act == types.ACTIVATE
}

func pageSignature(pageName string, xml string) string {
	name := strings.TrimSpace(pageName)
	content := strings.TrimSpace(xml)
	if name == "" && content == "" {
		return ""
	}
	h := fnv.New64a()
	_, _ = h.Write([]byte(name))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(content))
	return fmt.Sprintf("%x", h.Sum64())
}
