package monkey

import (
	"fmt"
	"hash/fnv"
	"strconv"
	"strings"
	"trek/internal/engine/core/types"
	"trek/pkg/coordinator"
)

// blockDetector 基于 (pageName, actionKey, escaped) 检测遍历卡死模式。
//
// 判断条件（任一触发即阻塞）：
//   - 条件 1：连续 N 步 (pageName, actionKey) 完全相同，且 escaped=false（同按钮反复点没反应）
//   - 条件 2：连续 N 步 pageName 完全相同，且 escaped=false（同页面任何操作都没反应）
type blockDetector struct {
	threshold          int
	recentTraces       []traceRecord
	lastReason         string
	ignoredActionTypes map[types.ActionType]bool
}

type traceRecord struct {
	pageName  string
	actionKey string
	escaped   bool
}

func newBlockDetector(threshold int) *blockDetector {
	if threshold <= 0 {
		threshold = defaultBlockNoChangeThreshold
	}
	return &blockDetector{
		threshold:    threshold,
		recentTraces: make([]traceRecord, 0, threshold+1),
		ignoredActionTypes: map[types.ActionType]bool{
			types.NOP:           true,
			types.START:         true,
			types.RESTART:       true,
			types.CLEAN_RESTART: true,
			types.ACTIVATE:      true,
			types.BACK:          true,
		},
	}
}

// Observe 记录一步的结果并检测阻塞。
// pageName: 当前页面名（来自 page_name_strategy）
// actionKey: 动作类型+操作区域，格式 "CLICK@[0.100,0.200,0.300,0.400]"
// escaped: 动作后页面是否发生了变化
func (d *blockDetector) Observe(pageName, actionKey string, act types.ActionType, escaped bool) bool {
	if d == nil {
		return false
	}
	pageName = strings.TrimSpace(pageName)
	actionKey = strings.TrimSpace(actionKey)

	// 忽略系统级动作（BACK/RESTART 等），不纳入阻塞判断
	if d.ignoredActionTypes[act] || pageName == "" {
		d.recentTraces = d.recentTraces[:0]
		return false
	}

	d.recentTraces = append(d.recentTraces, traceRecord{
		pageName:  pageName,
		actionKey: actionKey,
		escaped:   escaped,
	})
	// 保留最近 threshold+1 条记录即可判断
	maxLen := d.threshold + 1
	if len(d.recentTraces) > maxLen {
		d.recentTraces = d.recentTraces[len(d.recentTraces)-maxLen:]
	}

	if len(d.recentTraces) < d.threshold {
		return false
	}

	tail := d.recentTraces[len(d.recentTraces)-d.threshold:]

	// 条件 1：连续 N 步 (pageName, actionKey) 完全相同，且都没跳出
	sameAction := true
	samePage := true
	allStuck := true
	for _, t := range tail {
		if t.escaped {
			allStuck = false
			break
		}
		if t.pageName != tail[0].pageName || t.actionKey != tail[0].actionKey {
			sameAction = false
		}
		if t.pageName != tail[0].pageName {
			samePage = false
		}
	}

	if !allStuck {
		return false
	}

	if sameAction {
		d.lastReason = blockReasonSameActionNoChange
		return true
	}
	if samePage {
		d.lastReason = blockReasonSamePageNoChange
		return true
	}
	return false
}

func (d *blockDetector) Reset() {
	if d == nil {
		return
	}
	d.recentTraces = d.recentTraces[:0]
	d.lastReason = ""
}

func (d *blockDetector) LastReason() string {
	if d == nil || strings.TrimSpace(d.lastReason) == "" {
		return "unknown"
	}
	return d.lastReason
}

// buildActionKey 生成包含动作类型和操作区域的 key。
func buildActionKey(cmd *types.ActionCommand) string {
	if cmd == nil {
		return ""
	}
	pos := cmd.Pos
	return fmt.Sprintf("%s@[%.3f,%.3f,%.3f,%.3f]", cmd.Act.String(), pos.Left, pos.Top, pos.Right, pos.Bottom)
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
	return strconv.FormatUint(h.Sum64(), 16)
}

// cachedSignature 优先使用 PageSnapshot 缓存的签名，未缓存时现场计算。
func cachedSignature(page coordinator.PageSnapshot) string {
	if page.Signature != "" {
		return page.Signature
	}
	return pageSignature(page.PageName, page.XML)
}

// cachedSignaturePtr 对 *PageSnapshot 同理。
func cachedSignaturePtr(page *coordinator.PageSnapshot) string {
	if page == nil {
		return ""
	}
	if page.Signature != "" {
		return page.Signature
	}
	return pageSignature(page.PageName, page.XML)
}
