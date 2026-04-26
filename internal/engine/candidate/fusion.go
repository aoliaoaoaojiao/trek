package candidate

import (
	"sort"
	"strings"
)

// FusionOptions 定义候选融合的可选参数。
type FusionOptions struct {
	KnownFailedActions map[string]bool
}

// FuseCandidates 对多来源候选做去重与基础打分排序。
func FuseCandidates(items []Candidate, options FusionOptions) []Candidate {
	if len(items) == 0 {
		return nil
	}
	merged := make(map[string]Candidate, len(items))
	order := make([]string, 0, len(items))

	for _, item := range items {
		if item.Command == nil {
			continue
		}
		key := commandKey(item)
		if key == "" {
			continue
		}
		if options.KnownFailedActions != nil && options.KnownFailedActions[key] {
			item.RiskScore = maxFloat(item.RiskScore, 1)
		}
		existing, ok := merged[key]
		if !ok {
			merged[key] = normalizeCandidate(item)
			order = append(order, key)
			continue
		}
		merged[key] = mergeCandidate(existing, item)
	}

	result := make([]Candidate, 0, len(order))
	for _, key := range order {
		if item, ok := merged[key]; ok {
			result = append(result, item)
		}
	}

	sort.SliceStable(result, func(i, j int) bool {
		return fusionScore(result[i]) > fusionScore(result[j])
	})
	return result
}

func mergeCandidate(base Candidate, incoming Candidate) Candidate {
	result := base
	result.Confidence = maxFloat(base.Confidence, incoming.Confidence)
	result.NoveltyScore = maxFloat(base.NoveltyScore, incoming.NoveltyScore)
	result.EscapeScore = maxFloat(base.EscapeScore, incoming.EscapeScore)
	result.RiskScore = maxFloat(base.RiskScore, incoming.RiskScore)
	if strings.TrimSpace(result.Intent) == "" {
		result.Intent = incoming.Intent
	}
	result.Source = mergeSources(base.Source, incoming.Source)
	result.Metadata = mergeMetadata(base.Metadata, incoming.Metadata)
	return result
}

func normalizeCandidate(item Candidate) Candidate {
	result := item
	result.Source = strings.TrimSpace(result.Source)
	result.Metadata = cloneMetadata(result.Metadata)
	return result
}

func mergeSources(left string, right string) string {
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	if left == "" {
		return right
	}
	if right == "" || strings.EqualFold(left, right) {
		return left
	}
	return left + "|" + right
}

func mergeMetadata(left map[string]string, right map[string]string) map[string]string {
	result := cloneMetadata(left)
	for key, value := range right {
		if _, exists := result[key]; !exists {
			result[key] = value
		}
	}
	return result
}

func commandKey(item Candidate) string {
	if item.Command == nil {
		return ""
	}
	return item.Command.ToJSON()
}

func fusionScore(item Candidate) float64 {
	return item.Confidence + item.EscapeScore + item.NoveltyScore*0.5 - item.RiskScore
}

func maxFloat(left float64, right float64) float64 {
	if right > left {
		return right
	}
	return left
}
