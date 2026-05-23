package uctbandit

import (
	"fmt"
	"math/rand"
	"strings"
	"trek/internal/config"
	"trek/internal/engine/core/types"
	"trek/logger"

	sharedgraph "trek/internal/engine/decision/shared/graph"
)

// AgentConfig 定义 Agent 的运行参数。
type AgentConfig struct {
	UCTWeight              float64 // UCT 分数权重，默认 0.6
	BanditWeight           float64 // Bandit 分数权重，默认 0.3
	HeuristicWeight        float64 // 启发式权重，默认 0.1
	BackPenalty            float64 // back 动作默认惩罚，默认 -1.0
	EscapeBonus            float64 // 逃逸加成，默认 3.0
	ExploreRatio           float64 // ε-贪心探索率，默认 0.15（15% 概率随机选择）
	ActionCooldownPenalty  float64 // 同状态同动作近期重复惩罚
	RecentActionWindow     int     // 近期动作窗口大小
	LoopEscapeExploreBoost float64 // 检测到回环时的探索增益
}

// DefaultAgentConfig 返回默认 Agent 配置。
func DefaultAgentConfig() AgentConfig {
	return AgentConfig{
		UCTWeight:              0.6,
		BanditWeight:           0.3,
		HeuristicWeight:        0.1,
		BackPenalty:            config.DefaultBackPenalty,
		EscapeBonus:            config.DefaultEscapeBonus,
		ExploreRatio:           config.DefaultExploreRatio,
		ActionCooldownPenalty:  config.DefaultActionCooldownPenalty,
		RecentActionWindow:     config.DefaultRecentActionWindow,
		LoopEscapeExploreBoost: config.DefaultLoopEscapeExploreBoost,
	}
}

// Agent 是 UCT+Bandit 算法的核心实现，
// 串联状态构造、候选动作生成、UCT+Bandit 打分、启发式加成/惩罚、动作选择、reward 回写。
// 实现 types.IAgent 接口。
type Agent struct {
	model      *sharedgraph.Model
	deviceType types.DeviceType

	planner  *Planner
	policy   *BanditPolicy
	rewarder *Rewarder
	stats    *StatsStore

	currentState  types.IState
	lastState     types.IState
	newState      types.IState
	lastAction    *types.StatefulAction
	currentAction *types.StatefulAction
	newAction     *types.StatefulAction

	lastStep             StepContext
	recentStates         []string       // 最近的 state ID，用于短环检测
	recentSelections     []string       // 最近选择的 "stateID|actionKey"
	edgeTransitionCounts map[string]int // "fromState->toState" 计数

	config       AgentConfig
	rewardConfig RewardConfig

	staticConfig  types.StaticConfigProvider
	algorithmType string
}

// 确保 Agent 实现了 IAgent 接口。
var _ types.IAgent = (*Agent)(nil)

// NewAgent 创建新的 UCT+Bandit Agent。
func NewAgent(model *sharedgraph.Model, deviceType types.DeviceType, staticCfg ...types.StaticConfigProvider) *Agent {
	rewardConfig := DefaultRewardConfig()
	plannerConfig := DefaultPlannerConfig()
	banditConfig := DefaultBanditConfig()
	agentConfig := DefaultAgentConfig()

	stats := NewStatsStore()

	var sc types.StaticConfigProvider
	if len(staticCfg) > 0 && staticCfg[0] != nil {
		sc = staticCfg[0]
	}

	return &Agent{
		model:                model,
		deviceType:           deviceType,
		planner:              NewPlanner(stats, plannerConfig),
		policy:               NewBanditPolicy(stats, banditConfig),
		rewarder:             NewRewarder(rewardConfig),
		stats:                stats,
		config:               agentConfig,
		rewardConfig:         rewardConfig,
		staticConfig:         sc,
		algorithmType:        "uctbandit",
		recentStates:         make([]string, 0, 10),
		recentSelections:     make([]string, 0, 10),
		edgeTransitionCounts: make(map[string]int),
	}
}

// CreateState 创建状态（复用 types.Create）。
func (a *Agent) CreateState(pageName string, element types.IElement) types.IState {
	state := types.Create(element, pageName)
	return state
}

// OnAddNode 图添加新节点时的回调。
func (a *Agent) OnAddNode(node types.IState) {
	a.newState = node
	a.stats.RecordStateVisit(node.GetId())
}

// ResolveNewAction 解析新动作：构建候选 → 打分 → 选择。
func (a *Agent) ResolveNewAction() types.IAction {
	action := a.selectAction()
	if action == nil {
		logger.Warnf("UCT-Bandit: selectAction returned nil, using fallback")
		return a.fallbackAction()
	}

	statefulAction, ok := action.(*types.StatefulAction)
	if !ok || statefulAction == nil {
		logger.Warnf("UCT-Bandit: action is not StatefulAction, using fallback")
		return a.fallbackAction()
	}

	a.newAction = statefulAction
	return action
}

// SelectNewAction 选择新动作（复用 ResolveNewAction 逻辑）。
func (a *Agent) SelectNewAction() types.IAction {
	return a.ResolveNewAction()
}

// UpdateStrategy 更新策略（reward 回写在 MoveForward 中处理）。
func (a *Agent) UpdateStrategy() {
	// UCT+Bandit 的 reward 在 MoveForward 中统一回写
	// 这里预留额外的策略更新空间
}

// MoveForward 前进到下一状态，完成 reward 回写。
func (a *Agent) MoveForward(nextState types.IState) {
	a.lastState = a.currentState
	a.currentState = a.newState
	a.newState = nextState

	a.lastAction = a.currentAction
	a.currentAction = a.newAction
	a.newAction = nil

	// reward 回写
	if a.currentAction != nil && a.currentState != nil {
		a.settleReward()
	}
}

// GetAlgorithmType 返回算法类型。
func (a *Agent) GetAlgorithmType() string {
	return a.algorithmType
}

// Stop 停止 Agent（预留清理逻辑）。
func (a *Agent) Stop() {
	// 第一阶段无持久化，预留
}

// GetModel 返回模型引用。
func (a *Agent) GetModel() *sharedgraph.Model {
	return a.model
}

// SetModel 设置模型引用。
func (a *Agent) SetModel(model *sharedgraph.Model) {
	a.model = model
}

// GetCurrentStateBlockTimes 返回当前状态的阻塞次数（实现 StateBlockAwareAgent）。
// 只有当停滞次数达到 StagnationThreshold 时才返回非零值，
// 避免 1 次停滞就触发 RESTART。
func (a *Agent) GetCurrentStateBlockTimes() int {
	if a.currentState == nil {
		return 0
	}
	stateID := a.currentState.GetId()
	stats, ok := a.stats.GetStateStats(stateID)
	if !ok {
		return 0
	}
	if stats.StagnationCount < a.rewardConfig.StagnationThreshold {
		return 0
	}
	return stats.StagnationCount
}

// selectAction 核心动作选择逻辑。
// 使用 ε-贪心策略：以 ExploreRatio 概率随机选择候选动作（探索），
// 其余概率选择最高分候选（利用）。
func (a *Agent) selectAction() types.IAction {
	if a.newState == nil {
		return nil
	}
	a.applyStaticConfigOverrides()

	candidates := a.buildCandidates(a.newState)
	candidates = a.filterCandidates(candidates)

	if len(candidates) == 0 {
		return a.fallbackAction()
	}

	// 对每个候选动作打分
	for i := range candidates {
		c := &candidates[i]
		stateID := a.newState.GetId()
		actionKey := c.ActionKey
		armKey := c.ArmKey

		c.UCTScore = a.planner.ComputeUCTScore(stateID, actionKey)
		c.BanditScore = a.policy.ComputeBanditScore(armKey)
		c.BonusScore = a.heuristicBonus(c)
		c.PenaltyScore = a.heuristicPenalty(c)

		c.FinalScore = a.config.UCTWeight*c.UCTScore +
			a.config.BanditWeight*c.BanditScore +
			a.config.HeuristicWeight*c.BonusScore +
			c.PenaltyScore
	}

	// ε-贪心选择：以 ExploreRatio 概率随机选择（探索），其余选择最高分（利用）。
	// 当检测到明显回环时，提高探索概率，尽快跳出 A↔B 局部最优。
	exploreRatio := a.config.ExploreRatio
	if a.isLikelyLooping() {
		exploreRatio += a.config.LoopEscapeExploreBoost
		if exploreRatio > 0.9 {
			exploreRatio = 0.9
		}
	}

	var selected Candidate
	if rand.Float64() < exploreRatio && len(candidates) > 1 {
		// 探索：随机选择一个候选动作
		selected = candidates[rand.Intn(len(candidates))]
		logger.Debugf("UCT-Bandit: exploring, random pick from %d candidates ratio=%.2f", len(candidates), exploreRatio)
	} else {
		// 利用：选择最高分候选
		bestIdx := 0
		bestScore := candidates[0].FinalScore
		for i := 1; i < len(candidates); i++ {
			if candidates[i].FinalScore > bestScore {
				bestIdx = i
				bestScore = candidates[i].FinalScore
			}
		}
		selected = candidates[bestIdx]
	}

	logger.Infof("UCT-Bandit: selected action=%s key=%s arm=%s uct=%.4f bandit=%.4f bonus=%.4f penalty=%.4f final=%.4f",
		selected.Action.GetId(), selected.ActionKey, selected.ArmKey,
		selected.UCTScore, selected.BanditScore, selected.BonusScore, selected.PenaltyScore, selected.FinalScore)

	// 记录选择
	a.recordSelection(selected, len(candidates))

	return selected.Action
}

// buildCandidates 从当前状态构建候选动作列表。
func (a *Agent) buildCandidates(state types.IState) []Candidate {
	actions := state.GetActions()
	if len(actions) == 0 {
		return nil
	}

	candidates := make([]Candidate, 0, len(actions))

	// 提取页面特征用于聚类
	pageFeatures := a.extractPageFeatures(state)
	pageCluster := BuildPageCluster(
		pageFeatures.PageName,
		pageFeatures.ClickableCount,
		pageFeatures.HasList,
		pageFeatures.HasInput,
		pageFeatures.Title,
	)

	for _, action := range actions {
		actionKey := a.buildActionKeyFromStatefulAction(action, pageCluster)
		armKey := a.buildArmKeyFromStatefulAction(action, pageCluster)

		candidates = append(candidates, Candidate{
			Action:    action,
			ActionKey: actionKey,
			ArmKey:    armKey,
		})
	}

	return candidates
}

// filterCandidates 过滤明显无效的候选动作。
// 应用控制类动作（RESTART/START/CLEAN_RESTART）不应由算法层产生，
// 它们由 model.go 的 block state 机制统一管理。
func (a *Agent) filterCandidates(candidates []Candidate) []Candidate {
	filtered := make([]Candidate, 0, len(candidates))
	for _, c := range candidates {
		if c.Action == nil {
			continue
		}
		if !c.Action.GetEnabled() {
			continue
		}
		if !c.Action.IsValid() {
			continue
		}
		// 过滤掉应用控制类动作，这些由 model.go 的 block state 机制管理
		actType := c.Action.GetActionType()
		if actType == types.RESTART || actType == types.START || actType == types.CLEAN_RESTART {
			continue
		}
		filtered = append(filtered, c)
	}

	// 如果过滤后为空，至少保留 back 动作作为兜底
	if len(filtered) == 0 {
		for _, c := range candidates {
			if c.Action != nil && c.Action.IsBack() {
				filtered = append(filtered, c)
				break
			}
		}
	}

	return filtered
}

// fallbackAction 当候选动作为空时，选择回退动作。
// 不返回 RESTART — 应用级重启由 model.go 的 block state 机制统一管理。
func (a *Agent) fallbackAction() types.IAction {
	if a.newState != nil {
		for _, action := range a.newState.GetActions() {
			if action != nil && action.IsBack() {
				return action
			}
		}
	}
	// 无候选也无 back 动作，返回 nil 让 model.go 处理
	logger.Warnf("UCT-Bandit: no candidates and no back action, returning nil")
	return nil
}

// heuristicBonus 计算启发式加成。
func (a *Agent) heuristicBonus(c *Candidate) float64 {
	if c.Action == nil {
		return 0
	}

	var bonus float64

	// 未访问动作加成：未尝试的按钮应有更强的探索动力
	if !c.Action.IsVisited() {
		bonus += 1.0
	}

	// 点击动作倾向
	if c.Action.IsClick() {
		bonus += 0.5
	}

	// 停滞逃逸检测
	stateID := ""
	if a.newState != nil {
		stateID = a.newState.GetId()
	}
	if a.rewarder.NeedEscape(a.getCurrentStagnationCount(stateID)) {
		// 停滞状态下提升 back 优先级
		if c.Action.IsBack() {
			bonus += a.config.EscapeBonus
		}
	}

	return bonus
}

// heuristicPenalty 计算启发式惩罚。
func (a *Agent) heuristicPenalty(c *Candidate) float64 {
	if c.Action == nil {
		return 0
	}

	var penalty float64

	// back 动作默认惩罚
	if c.Action.IsBack() {
		penalty += a.config.BackPenalty
	}

	// NOP/RESTART 惩罚
	if c.Action.IsNop() || c.Action.GetActionType() == types.RESTART {
		penalty += 0.5
	}

	// 同状态同动作近期重复惩罚，降低在同一页反复点击同一控件的概率。
	stateID := a.stateIDEffective()
	repeatHits := a.countRecentSelectionHits(stateID, c.ActionKey)
	if repeatHits > 0 {
		penalty += float64(repeatHits) * a.config.ActionCooldownPenalty
	}

	return penalty
}

// recordSelection 记录动作选择到统计存储。
func (a *Agent) recordSelection(c Candidate, candidateCount int) {
	actionKey := c.ActionKey
	armKey := c.ArmKey

	// 更新 step context
	a.lastStep = StepContext{
		LastActionKey:  actionKey,
		LastArmKey:     armKey,
		CandidateCount: candidateCount,
	}

	if a.currentState != nil {
		a.lastStep.PrevStateID = a.currentState.GetId()
		a.lastStep.PrevPageName = a.currentState.GetPageNameString()
	}

	// 记录动作选择（reward 默认 0，后续在 settleReward 中回写）
	a.stats.RecordActionSelection(
		a.stateIDEffective(),
		actionKey,
		0,
	)

	// 记录 arm 拉取（初始 reward 为 0）
	a.stats.RecordArmPull(armKey, 0)
	a.recordRecentSelection(a.stateIDEffective(), actionKey)
}

// stateIDEffective 返回当前有效状态 ID。
func (a *Agent) stateIDEffective() string {
	if a.newState != nil {
		return a.newState.GetId()
	}
	if a.currentState != nil {
		return a.currentState.GetId()
	}
	return ""
}

// settleReward 在 MoveForward 中结算 reward。
func (a *Agent) settleReward() {
	if a.currentAction == nil || a.currentState == nil {
		return
	}
	a.applyStaticConfigOverrides()

	prevStateID := ""
	if a.lastState != nil {
		prevStateID = a.lastState.GetId()
	}

	nextStateID := ""
	if a.newState != nil {
		nextStateID = a.newState.GetId()
	}

	// 检测各种信号
	// 注意：MoveForward 中 currentState 和 newState 来自同一次状态创建，
	// 它们始终相同（OnAddNode 设置 newState，MoveForward 的 nextState 也是同一个 state），
	// 因此必须用 lastState（动作前的状态）和 newState（当前状态）比较来判断是否发生了状态转移。
	foundNewState := a.lastState != nil && a.newState != nil &&
		!a.lastState.Equals(a.newState)

	// 检测新边：动作产生了状态转移且需要目标
	foundNewEdge := foundNewState && a.currentAction.RequireTarget()

	// 页面无变化：lastState 和 newState 相同（有前驱状态可比较时才判定）
	isNoOp := a.lastState != nil && !foundNewState

	// 结构变化检测：页面名相同但 hash 不同
	structureChanged := false
	if a.lastState != nil && a.newState != nil &&
		a.lastState.GetPageNameString() == a.newState.GetPageNameString() &&
		a.lastState.Hash() != a.newState.Hash() {
		structureChanged = true
	}

	// 计算 reward
	edgeRepeatCount := a.increaseEdgeTransitionCount(prevStateID, nextStateID)
	rewardResult := a.rewarder.ComputeReward(RewardInput{
		FoundNewState:    foundNewState,
		FoundNewEdge:     foundNewEdge,
		StructureChanged: structureChanged,
		IsNoOp:           isNoOp,
		IsShortLoop:      false, // 短环检测在 ComputeReward 中完成
		IsEmptyResult:    a.newState == nil,
		PrevStateID:      prevStateID,
		NextStateID:      nextStateID,
		EdgeRepeatCount:  edgeRepeatCount,
		RecentStateIDs:   a.recentStates,
	})

	// 回写 reward 到统计
	stateID := a.currentState.GetId()
	actionKey := a.lastStep.LastActionKey
	if actionKey == "" && a.currentAction != nil {
		// 如果 lastStep 没有记录，从前一个 currentAction 重建 key
		actionKey = fmt.Sprintf("%s|fallback|no_res_id|empty|no_pos",
			a.currentAction.GetActionType().String())
	}

	a.stats.RecordActionSelection(stateID, actionKey, rewardResult.Reward)
	if a.lastStep.LastArmKey != "" {
		a.stats.RecordArmPull(a.lastStep.LastArmKey, rewardResult.Reward)
	}

	// 更新停滞计数
	if isNoOp {
		a.stats.IncrementStagnation(stateID)
	} else {
		a.stats.ResetStagnation(stateID)
	}

	// 更新近期状态历史
	a.updateRecentStates(nextStateID)

	// 设定逃逸信号
	rewardResult.NeedEscape = a.rewarder.NeedEscape(
		a.getCurrentStagnationCount(stateID))

	logger.Debugf("UCT-Bandit reward: state=%s action=%s reward=%.2f reason=%s newState=%v shortLoop=%v twoStateLoop=%v edgeRepeat=%d escape=%v",
		stateID, actionKey, rewardResult.Reward, rewardResult.Reason,
		rewardResult.FoundNewState, rewardResult.IsShortLoop, rewardResult.IsTwoStateLoop, edgeRepeatCount, rewardResult.NeedEscape)
}

// updateRecentStates 更新近期状态历史。
func (a *Agent) updateRecentStates(stateID string) {
	a.recentStates = append(a.recentStates, stateID)
	// 保留最近 shortLoopWindow + 1 个状态
	maxHistory := a.rewardConfig.ShortLoopWindow + 2
	if len(a.recentStates) > maxHistory {
		a.recentStates = a.recentStates[len(a.recentStates)-maxHistory:]
	}
}

func (a *Agent) recordRecentSelection(stateID, actionKey string) {
	if strings.TrimSpace(stateID) == "" || strings.TrimSpace(actionKey) == "" {
		return
	}
	key := stateID + "|" + actionKey
	a.recentSelections = append(a.recentSelections, key)
	maxSize := a.config.RecentActionWindow
	if maxSize <= 0 {
		maxSize = 6
	}
	if len(a.recentSelections) > maxSize {
		a.recentSelections = a.recentSelections[len(a.recentSelections)-maxSize:]
	}
}

func (a *Agent) countRecentSelectionHits(stateID, actionKey string) int {
	if strings.TrimSpace(stateID) == "" || strings.TrimSpace(actionKey) == "" {
		return 0
	}
	key := stateID + "|" + actionKey
	hits := 0
	for _, candidate := range a.recentSelections {
		if candidate == key {
			hits++
		}
	}
	return hits
}

func (a *Agent) isLikelyLooping() bool {
	n := len(a.recentStates)
	if n < 4 {
		return false
	}

	// 检测 A-B-A-B 交替循环
	a1 := a.recentStates[n-4]
	b1 := a.recentStates[n-3]
	a2 := a.recentStates[n-2]
	b2 := a.recentStates[n-1]
	if a1 == a2 && b1 == b2 && a1 != b1 {
		return true
	}

	// 检测 A-A-A-A 同状态卡死：最近 3 个状态全部相同
	if n >= 3 {
		s1 := a.recentStates[n-3]
		s2 := a.recentStates[n-2]
		s3 := a.recentStates[n-1]
		if s1 == s2 && s2 == s3 {
			return true
		}
	}

	return false
}

func (a *Agent) increaseEdgeTransitionCount(prevStateID, nextStateID string) int {
	if strings.TrimSpace(prevStateID) == "" || strings.TrimSpace(nextStateID) == "" {
		return 0
	}
	key := prevStateID + "->" + nextStateID
	a.edgeTransitionCounts[key]++
	return a.edgeTransitionCounts[key]
}

func (a *Agent) applyStaticConfigOverrides() {
	if a.staticConfig == nil {
		return
	}
	uctCfg := a.staticConfig.GetUCTBanditConfig()

	if uctCfg.TwoStateLoopPenalty.IsSet() {
		a.rewardConfig.TwoStateLoopPenalty = uctCfg.TwoStateLoopPenalty.Get()
	}
	if uctCfg.EdgeRepeatPenalty.IsSet() {
		a.rewardConfig.EdgeRepeatPenalty = uctCfg.EdgeRepeatPenalty.Get()
	}
	if uctCfg.EdgeRepeatThreshold.IsSet() && uctCfg.EdgeRepeatThreshold.Get() > 0 {
		a.rewardConfig.EdgeRepeatThreshold = uctCfg.EdgeRepeatThreshold.Get()
	}
	if uctCfg.ActionCooldownPenalty.IsSet() {
		a.config.ActionCooldownPenalty = uctCfg.ActionCooldownPenalty.Get()
	}
	if uctCfg.RecentActionWindow.IsSet() && uctCfg.RecentActionWindow.Get() > 0 {
		a.config.RecentActionWindow = uctCfg.RecentActionWindow.Get()
	}
	if uctCfg.LoopEscapeExploreBoost.IsSet() {
		a.config.LoopEscapeExploreBoost = uctCfg.LoopEscapeExploreBoost.Get()
	}
	a.rewarder.UpdateConfig(a.rewardConfig)
}

// getCurrentStagnationCount 获取当前状态停滞计数。
func (a *Agent) getCurrentStagnationCount(stateID string) int {
	stats, ok := a.stats.GetStateStats(stateID)
	if !ok {
		return 0
	}
	return stats.StagnationCount
}

// extractPageFeatures 从 IState 中提取页面特征。
func (a *Agent) extractPageFeatures(state types.IState) PageFeatures {
	features := PageFeatures{
		PageName: state.GetPageNameString(),
	}

	actions := state.GetActions()
	clickableCount := 0
	hasList := false
	hasInput := false

	for _, action := range actions {
		if action.IsClick() {
			clickableCount++
		}
		target := action.GetTarget()
		if target != nil {
			if target.IsEditable() {
				hasInput = true
			}
			// 如果可滚动则认为存在列表
			if action.GetActionType() == types.SCROLL_BOTTOM_UP ||
				action.GetActionType() == types.SCROLL_TOP_DOWN ||
				action.GetActionType() == types.SCROLL_LEFT_RIGHT ||
				action.GetActionType() == types.SCROLL_RIGHT_LEFT {
				hasList = true
			}
		}
	}

	features.ClickableCount = clickableCount
	features.HasList = hasList
	features.HasInput = hasInput

	return features
}

// buildActionKeyFromStatefulAction 从 StatefulAction 构建 ActionKey。
func (a *Agent) buildActionKeyFromStatefulAction(sa *types.StatefulAction, pageCluster string) string {
	widgetType := ""
	resourceID := ""
	text := ""
	normX := 0.5
	normY := 0.5

	target := sa.GetTarget()
	if target != nil {
		widgetType = fmt.Sprintf("%T", target)
		resourceID = target.GetPath()
		text = target.GetText()
		// 尝试从边界框计算归一化位置
		bounds := target.GetBounds()
		if bounds != nil && !bounds.IsEmpty() && a.newState != nil {
			state, ok := a.newState.(*types.State)
			if ok {
				rootBounds := state.RootBounds
				if rootBounds != nil && !rootBounds.IsEmpty() {
					rootW := rootBounds.Right - rootBounds.Left
					rootH := rootBounds.Bottom - rootBounds.Top
					if rootW > 0 && rootH > 0 {
						centerX := (bounds.Left + bounds.Right) / 2
						centerY := (bounds.Top + bounds.Bottom) / 2
						normX = (centerX - rootBounds.Left) / rootW
						normY = (centerY - rootBounds.Top) / rootH
						// 限制在 [0, 1] 范围内
						if normX < 0 {
							normX = 0
						} else if normX > 1 {
							normX = 1
						}
						if normY < 0 {
							normY = 0
						} else if normY > 1 {
							normY = 1
						}
					}
				}
			}
		}
	}

	return BuildActionKey(sa.GetActionType(), widgetType, resourceID, text, normX, normY)
}

// buildArmKeyFromStatefulAction 从 StatefulAction 构建 ArmKey。
func (a *Agent) buildArmKeyFromStatefulAction(sa *types.StatefulAction, pageCluster string) string {
	widgetType := ""
	positionBucket := "middle_center"

	target := sa.GetTarget()
	if target != nil {
		widgetType = fmt.Sprintf("%T", target)
		bounds := target.GetBounds()
		if bounds != nil && !bounds.IsEmpty() && a.newState != nil {
			state, ok := a.newState.(*types.State)
			if ok {
				rootBounds := state.RootBounds
				if rootBounds != nil && !rootBounds.IsEmpty() {
					rootW := rootBounds.Right - rootBounds.Left
					rootH := rootBounds.Bottom - rootBounds.Top
					if rootW > 0 && rootH > 0 {
						centerX := (bounds.Left + bounds.Right) / 2
						centerY := (bounds.Top + bounds.Bottom) / 2
						normX := (centerX - rootBounds.Left) / rootW
						normY := (centerY - rootBounds.Top) / rootH
						positionBucket = classifyPosition(normX, normY)
					}
				}
			}
		}
	}

	return BuildArmKey(pageCluster, widgetType, sa.GetActionType(), positionBucket)
}
