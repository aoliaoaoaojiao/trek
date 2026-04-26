package memory

import (
	"sort"
	"strings"
	enginestate "trek/internal/engine/state"
)

func findMatches(records []RecoveryMemoryRecord, ctx enginestate.TraversalContext) []RecoveryMemoryRecord {
	trace := buildTraceSignature(ctx)

	exact := make([]RecoveryMemoryRecord, 0, 8)
	for _, item := range records {
		if equalFold(item.Mode, ctx.Mode) &&
			equalFold(item.PageSignature, ctx.PageSignature) &&
			equalFold(item.BlockReason, ctx.BlockReason) &&
			equalFold(item.TraceSignature, trace) {
			exact = append(exact, cloneRecord(item))
		}
	}
	if len(exact) > 0 {
		sortBySuccessRate(exact)
		return exact
	}

	cluster := make([]RecoveryMemoryRecord, 0, 8)
	for _, item := range records {
		if equalFold(item.Mode, ctx.Mode) &&
			equalFold(item.ClusterSignature, ctx.ClusterSignature) &&
			equalFold(item.BlockReason, ctx.BlockReason) {
			cluster = append(cluster, cloneRecord(item))
		}
	}
	if len(cluster) > 0 {
		sortBySuccessRate(cluster)
		return cluster
	}

	page := make([]RecoveryMemoryRecord, 0, 8)
	for _, item := range records {
		if equalFold(item.Mode, ctx.Mode) &&
			equalFold(item.PageSignature, ctx.PageSignature) {
			page = append(page, cloneRecord(item))
		}
	}
	sortBySuccessRate(page)
	return page
}

func buildTraceSignature(ctx enginestate.TraversalContext) string {
	if len(ctx.RecentTrace) == 0 {
		return ""
	}
	parts := make([]string, 0, len(ctx.RecentTrace))
	for _, item := range ctx.RecentTrace {
		key := strings.TrimSpace(item.ActionKey)
		if key == "" {
			continue
		}
		parts = append(parts, key)
	}
	return strings.Join(parts, ">")
}

func sortBySuccessRate(items []RecoveryMemoryRecord) {
	sort.SliceStable(items, func(i, j int) bool {
		return successRate(items[i]) > successRate(items[j])
	})
}

func successRate(item RecoveryMemoryRecord) float64 {
	total := item.SuccessCount + item.FailCount
	if total <= 0 {
		return 0
	}
	return float64(item.SuccessCount) / float64(total)
}

func equalFold(left string, right string) bool {
	return strings.EqualFold(strings.TrimSpace(left), strings.TrimSpace(right))
}
