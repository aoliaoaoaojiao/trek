package monkey

import (
	"fmt"
	"hash/fnv"
	"strings"
	"trek/internal/engine/decision/shared/types"
	"trek/pkg/session"
)

// blockDetector 检测遍历过程中的卡死模式：滚动无变化、同页面无移动、两状态乒乓、高访问低收益。
type blockDetector struct {
	noChangeThreshold         int
	twoStateLoopThreshold     int
	highVisitThreshold        int
	lowRewardWindow           int
	consecutiveNoChangeScroll int
	consecutiveSamePageNoMove int
	consecutiveTwoStateLoops  int
	recentAfterSignatures     []string
	recentObservedSignatures  []string
	pageVisitCount            map[string]int
	lastReason                string
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
		noChangeThreshold:        noChangeThreshold,
		twoStateLoopThreshold:    twoStateLoopThreshold,
		highVisitThreshold:       highVisitThreshold,
		lowRewardWindow:          lowRewardWindow,
		recentAfterSignatures:    make([]string, 0, 8),
		recentObservedSignatures: make([]string, 0, 16),
		pageVisitCount:           make(map[string]int),
	}
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

	beforeSig := pageSignature(before.PageName, before.XML)
	afterSig := pageSignature(after.PageName, after.XML)
	if !isBlockDetectorIgnoredAction(cmd.Act) && afterSig != "" {
		d.pageVisitCount[afterSig]++
		d.pushObservedSignature(afterSig)
	}

	if cmd.IsScrollAction() && beforeSig != "" && beforeSig == afterSig {
		d.consecutiveNoChangeScroll++
	} else {
		d.consecutiveNoChangeScroll = 0
	}
	if d.consecutiveNoChangeScroll >= d.noChangeThreshold {
		triggerNoChangeScroll = true
		d.lastReason = blockReasonScrollNoChange
	}

	if !isBlockDetectorIgnoredAction(cmd.Act) && !cmd.IsScrollAction() && beforeSig != "" && beforeSig == afterSig {
		d.consecutiveSamePageNoMove++
	} else {
		d.consecutiveSamePageNoMove = 0
	}
	if d.consecutiveSamePageNoMove >= d.noChangeThreshold {
		triggerSamePageNoMove = true
		d.lastReason = blockReasonSamePageNoChange
	}

	if !isBlockDetectorIgnoredAction(cmd.Act) && beforeSig != "" && afterSig != "" && beforeSig != afterSig {
		d.pushAfterSignature(afterSig)
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

	if d.isHighVisitLowReward(afterSig) {
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
	d.recentAfterSignatures = d.recentAfterSignatures[:0]
	d.recentObservedSignatures = d.recentObservedSignatures[:0]
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
	d.recentAfterSignatures = append(d.recentAfterSignatures, sig)
	if len(d.recentAfterSignatures) > 8 {
		d.recentAfterSignatures = d.recentAfterSignatures[len(d.recentAfterSignatures)-8:]
	}
}

func (d *blockDetector) pushObservedSignature(sig string) {
	if d == nil || strings.TrimSpace(sig) == "" {
		return
	}
	d.recentObservedSignatures = append(d.recentObservedSignatures, sig)
	if len(d.recentObservedSignatures) > 16 {
		d.recentObservedSignatures = d.recentObservedSignatures[len(d.recentObservedSignatures)-16:]
	}
}

func (d *blockDetector) isTailABAB() bool {
	if d == nil || len(d.recentAfterSignatures) < 4 {
		return false
	}
	n := len(d.recentAfterSignatures)
	a := d.recentAfterSignatures[n-4]
	b := d.recentAfterSignatures[n-3]
	c := d.recentAfterSignatures[n-2]
	e := d.recentAfterSignatures[n-1]
	return a == c && b == e && a != b
}

func (d *blockDetector) isHighVisitLowReward(afterSig string) bool {
	if d == nil || strings.TrimSpace(afterSig) == "" {
		return false
	}
	if d.pageVisitCount[afterSig] < d.highVisitThreshold {
		return false
	}
	if d.lowRewardWindow <= 0 || len(d.recentObservedSignatures) < d.lowRewardWindow {
		return false
	}
	start := len(d.recentObservedSignatures) - d.lowRewardWindow
	tail := d.recentObservedSignatures[start:]
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
