package uctbandit

import "sync"

// StateStats 记录单个状态的访问统计。
type StateStats struct {
	Visits          int
	LastVisitStep   int
	StagnationCount int
}

// ActionStats 记录单个状态-动作对的统计。
type ActionStats struct {
	Visits       int
	TotalReward  float64
	MeanReward   float64
	SuccessCount int
	NoOpCount    int
	LoopCount    int
}

// BanditArmStats 记录 Bandit arm 的统计。
type BanditArmStats struct {
	Pulls       int
	TotalReward float64
	MeanReward  float64
}

// StatsStore 维护 UCT+Bandit 算法所需的全部统计信息。
type StatsStore struct {
	mu            sync.RWMutex
	stateStats    map[string]*StateStats     // stateID → StateStats
	actionStats   map[string]*ActionStats    // "stateID|actionKey" → ActionStats
	armStats      map[string]*BanditArmStats // armKey → BanditArmStats
	totalVisits   int
	totalArmPulls int
}

// NewStatsStore 创建新的统计存储。
func NewStatsStore() *StatsStore {
	return &StatsStore{
		stateStats:  make(map[string]*StateStats),
		actionStats: make(map[string]*ActionStats),
		armStats:    make(map[string]*BanditArmStats),
	}
}

// actionStatsKey 生成 actionStats 的复合键。
func actionStatsKey(stateID, actionKey string) string {
	return stateID + "|" + actionKey
}

// RecordStateVisit 记录状态访问。
func (s *StatsStore) RecordStateVisit(stateID string) *StateStats {
	s.mu.Lock()
	defer s.mu.Unlock()

	stats, ok := s.stateStats[stateID]
	if !ok {
		stats = &StateStats{}
		s.stateStats[stateID] = stats
	}
	stats.Visits++
	s.totalVisits++
	return stats
}

// GetStateStats 获取状态统计。
func (s *StatsStore) GetStateStats(stateID string) (*StateStats, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats, ok := s.stateStats[stateID]
	if !ok {
		return nil, false
	}
	// 返回副本避免外部修改
	copy := *stats
	return &copy, true
}

// RecordActionSelection 记录动作选择及其 reward。
func (s *StatsStore) RecordActionSelection(stateID, actionKey string, reward float64) *ActionStats {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := actionStatsKey(stateID, actionKey)
	stats, ok := s.actionStats[key]
	if !ok {
		stats = &ActionStats{}
		s.actionStats[key] = stats
	}
	stats.Visits++
	stats.TotalReward += reward
	stats.MeanReward = stats.TotalReward / float64(stats.Visits)
	return stats
}

// GetActionStats 获取动作统计。
func (s *StatsStore) GetActionStats(stateID, actionKey string) (*ActionStats, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := actionStatsKey(stateID, actionKey)
	stats, ok := s.actionStats[key]
	if !ok {
		return nil, false
	}
	copy := *stats
	return &copy, true
}

// RecordArmPull 记录 Bandit arm 拉取。
func (s *StatsStore) RecordArmPull(armKey string, reward float64) *BanditArmStats {
	s.mu.Lock()
	defer s.mu.Unlock()

	stats, ok := s.armStats[armKey]
	if !ok {
		stats = &BanditArmStats{}
		s.armStats[armKey] = stats
	}
	stats.Pulls++
	stats.TotalReward += reward
	stats.MeanReward = stats.TotalReward / float64(stats.Pulls)
	s.totalArmPulls++
	return stats
}

// GetArmStats 获取 Bandit arm 统计。
func (s *StatsStore) GetArmStats(armKey string) (*BanditArmStats, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats, ok := s.armStats[armKey]
	if !ok {
		return nil, false
	}
	copy := *stats
	return &copy, true
}

// TotalStateVisits 返回总状态访问次数。
func (s *StatsStore) TotalStateVisits() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.totalVisits
}

// TotalArmPulls 返回总 arm 拉取次数。
func (s *StatsStore) TotalArmPulls() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.totalArmPulls
}

// IncrementStagnation 增加状态停滞计数。
func (s *StatsStore) IncrementStagnation(stateID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	stats, ok := s.stateStats[stateID]
	if !ok {
		stats = &StateStats{}
		s.stateStats[stateID] = stats
	}
	stats.StagnationCount++
}

// ResetStagnation 重置状态停滞计数。
func (s *StatsStore) ResetStagnation(stateID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	stats, ok := s.stateStats[stateID]
	if !ok {
		return
	}
	stats.StagnationCount = 0
}

// StateVisits 返回指定状态的访问次数。
func (s *StatsStore) StateVisits(stateID string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats, ok := s.stateStats[stateID]
	if !ok {
		return 0
	}
	return stats.Visits
}

// ActionVisits 返回指定状态-动作的访问次数。
func (s *StatsStore) ActionVisits(stateID, actionKey string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := actionStatsKey(stateID, actionKey)
	stats, ok := s.actionStats[key]
	if !ok {
		return 0
	}
	return stats.Visits
}

// ActionMeanReward 返回指定状态-动作的平均 reward。
func (s *StatsStore) ActionMeanReward(stateID, actionKey string) float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := actionStatsKey(stateID, actionKey)
	stats, ok := s.actionStats[key]
	if !ok {
		return 0
	}
	return stats.MeanReward
}

// ArmPulls 返回指定 arm 被拉取的次数。
func (s *StatsStore) ArmPulls(armKey string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats, ok := s.armStats[armKey]
	if !ok {
		return 0
	}
	return stats.Pulls
}

// ArmMeanReward 返回指定 arm 的平均 reward。
func (s *StatsStore) ArmMeanReward(armKey string) float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats, ok := s.armStats[armKey]
	if !ok {
		return 0
	}
	return stats.MeanReward
}