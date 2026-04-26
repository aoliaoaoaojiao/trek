package candidate

import (
	"sort"
	"strings"
)

const (
	knownFailedRiskPenalty              = 2.0
	memoryWeightTagKey                  = "memory_weight_tag"
	memoryWeightTagCandidateEnhancement = "candidate_enhancement"
	memoryWeightTagBoost                = 0.25
)

// FusionOptions 定义候选融合的可选参数。
type FusionOptions struct {
	KnownFailedActions   map[string]bool
	RiskDropThreshold    float64
	EnableMinScoreFilter bool
	MinScoreThreshold    float64
	KeepTopOnFiltered    bool
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
			item.RiskScore = maxFloat(item.RiskScore, knownFailedRiskPenalty)
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
	return applyFusionFilters(result, options)
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
	score := item.Confidence + item.EscapeScore + item.NoveltyScore*0.5 - item.RiskScore
	if hasCandidateEnhancementTag(item) && item.RiskScore < knownFailedRiskPenalty {
		score += memoryWeightTagBoost
	}
	return score
}

func hasCandidateEnhancementTag(item Candidate) bool {
	if item.Metadata == nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(item.Metadata[memoryWeightTagKey]), memoryWeightTagCandidateEnhancement)
}

func maxFloat(left float64, right float64) float64 {
	if right > left {
		return right
	}
	return left
}

func applyFusionFilters(items []Candidate, options FusionOptions) []Candidate {
	if len(items) == 0 {
		return nil
	}
	filtered := make([]Candidate, 0, len(items))
	for _, item := range items {
		if options.RiskDropThreshold > 0 && item.RiskScore >= options.RiskDropThreshold {
			continue
		}
		if options.EnableMinScoreFilter && fusionScore(item) < options.MinScoreThreshold {
			continue
		}
		filtered = append(filtered, item)
	}
	if len(filtered) == 0 && options.KeepTopOnFiltered {
		return []Candidate{items[0]}
	}
	return filtered
}
