package monkey

import (
	"context"
	"testing"
	"trek/internal/engine/core/types"
	"trek/internal/engine/perception"
	enginestate "trek/internal/engine/state"
	session "trek/pkg/coordinator"
)

func TestRecoveryStateMachineEscalatesFromExploreToRecover(t *testing.T) {
	sm := newRecoveryStateMachine()

	if sm.Mode() != TraversalModeExplore {
		t.Fatalf("初始模式错误: %s", sm.Mode())
	}

	sm.OnBlockDetected("scroll_no_change")
	if sm.Mode() != TraversalModeSuspectBlocked {
		t.Fatalf("首次阻塞后应进入 SuspectBlocked，实际: %s", sm.Mode())
	}
	if sm.BlockReason() != "scroll_no_change" {
		t.Fatalf("阻塞原因未保留，实际: %s", sm.BlockReason())
	}

	sm.OnBlockDetected("scroll_no_change")
	if sm.Mode() != TraversalModeRecover {
		t.Fatalf("二次阻塞后应进入 Recover，实际: %s", sm.Mode())
	}
}

func TestRecoveryStateMachineRecoverSelfLoop(t *testing.T) {
	sm := newRecoveryStateMachine()

	// Explore -> SuspectBlocked -> Recover
	sm.OnBlockDetected("scroll_no_change")
	sm.OnBlockDetected("scroll_no_change")
	if sm.Mode() != TraversalModeRecover {
		t.Fatalf("应进入 Recover，实际: %s", sm.Mode())
	}

	// Recover -> Recover（自循环：恢复失败但预算未耗尽）
	sm.OnBlockDetected("two_state_ping_pong")
	if sm.Mode() != TraversalModeRecover {
		t.Fatalf("Recover 状态下再遇阻塞应保持 Recover（自循环），实际: %s", sm.Mode())
	}
	if sm.BlockReason() != "two_state_ping_pong" {
		t.Fatalf("阻塞原因应更新为最新原因，实际: %s", sm.BlockReason())
	}

	// 再次自循环
	sm.OnBlockDetected("same_page_no_change")
	if sm.Mode() != TraversalModeRecover {
		t.Fatalf("Recover 自循环应可重复，实际: %s", sm.Mode())
	}
}

func TestRecoveryStateMachineProgressEntersCooldownThenBackToExplore(t *testing.T) {
	sm := newRecoveryStateMachineWithCooldown(2)

	sm.OnBlockDetected("two_state_ping_pong")
	if sm.Mode() != TraversalModeSuspectBlocked {
		t.Fatalf("首次阻塞后应进入 SuspectBlocked，实际: %s", sm.Mode())
	}
	sm.OnBlockDetected("two_state_ping_pong")
	if sm.Mode() != TraversalModeRecover {
		t.Fatalf("二次阻塞后应进入 Recover，实际: %s", sm.Mode())
	}

	sm.OnProgress(true)
	if sm.Mode() != TraversalModeCooldown {
		t.Fatalf("恢复进展后应进入 Cooldown，实际: %s", sm.Mode())
	}
	if sm.BlockReason() != "" {
		t.Fatalf("恢复进展后应清空阻塞原因，实际: %s", sm.BlockReason())
	}

	sm.OnStepAdvance()
	if sm.Mode() != TraversalModeCooldown {
		t.Fatalf("冷却未结束前应保持 Cooldown，实际: %s", sm.Mode())
	}
	sm.OnStepAdvance()
	if sm.Mode() != TraversalModeExplore {
		t.Fatalf("冷却结束后应回到 Explore，实际: %s", sm.Mode())
	}
}

func TestRunnerHandleBlockDetectedRequiresEscalationBeforePendingRecovery(t *testing.T) {
	runner, err := NewRunner(&fakeDecider{}, &fakeDriver{}, Config{
		StepInterval: 0,
		StopOnCrash:  true,
		StopOnANR:    true,
	})
	if err != nil {
		t.Fatalf("创建 runner 失败: %v", err)
	}

	runner.handleBlockDetected("scroll_no_change")
	if runner.pendingBlockRecovery {
		t.Fatalf("首次阻塞不应立即进入待恢复状态")
	}
	if runner.recoveryState.Mode() != TraversalModeSuspectBlocked {
		t.Fatalf("首次阻塞后 runner 应进入 SuspectBlocked，实际: %s", runner.recoveryState.Mode())
	}

	runner.handleBlockDetected("scroll_no_change")
	if !runner.pendingBlockRecovery {
		t.Fatalf("二次阻塞后应进入待恢复状态")
	}
	if runner.recoveryState.Mode() != TraversalModeRecover {
		t.Fatalf("二次阻塞后 runner 应进入 Recover，实际: %s", runner.recoveryState.Mode())
	}
}

func TestRunnerBuildTraversalContextIncludesRecoveryStateAndTrace(t *testing.T) {
	runner, err := NewRunner(&fakeDecider{}, &fakeDriver{}, Config{
		StepInterval: 0,
		StopOnCrash:  true,
		StopOnANR:    true,
	})
	if err != nil {
		t.Fatalf("创建 runner 失败: %v", err)
	}

	runner.recoveryState.OnBlockDetected("scroll_no_change")
	runner.recoveryState.OnBlockDetected("scroll_no_change")
	runner.recentTrace = []enginestate.ActionTrace{
		{PageSignature: "page-a", ActionKey: "CLICK#1"},
		{PageSignature: "page-b", ActionKey: "BACK#1"},
	}

	ctx := runner.buildTraversalContext(7, session.PageSnapshot{
		PageName:   "MainActivity",
		XML:        "<hierarchy/>",
		Screenshot: []byte{1, 2, 3},
	}, map[string]int{"MainActivity": 4}, map[string]int{"CLICK": 3})

	if ctx.Step != 7 {
		t.Fatalf("step 不符合预期: %d", ctx.Step)
	}
	if ctx.Mode != enginestate.ModeRecover {
		t.Fatalf("mode 不符合预期: %s", ctx.Mode)
	}
	if ctx.BlockReason != "scroll_no_change" {
		t.Fatalf("block reason 不符合预期: %s", ctx.BlockReason)
	}
	if ctx.PageSignature == "" {
		t.Fatalf("page signature 不应为空")
	}
	if len(ctx.RecentTrace) != 2 {
		t.Fatalf("recent trace 长度错误: %d", len(ctx.RecentTrace))
	}
	if ctx.VisitStats.PageVisitCount["MainActivity"] != 4 {
		t.Fatalf("page visit count 不符合预期")
	}
	if ctx.VisitStats.ActionCount["CLICK"] != 3 {
		t.Fatalf("action count 不符合预期")
	}
}

func TestRunnerRecordActionTraceKeepsRecentWindow(t *testing.T) {
	runner, err := NewRunner(&fakeDecider{}, &fakeDriver{}, Config{
		StepInterval: 0,
		StopOnCrash:  true,
		StopOnANR:    true,
	})
	if err != nil {
		t.Fatalf("创建 runner 失败: %v", err)
	}

	for i := 0; i < 10; i++ {
		runner.recordActionTrace(session.PageSnapshot{
			PageName: "Page",
			XML:      "<hierarchy/>",
		}, &types.ActionCommand{Act: types.CLICK})
	}

	if len(runner.recentTrace) != maxRecentTraceEntries {
		t.Fatalf("recent trace 应限制长度为 %d，实际: %d", maxRecentTraceEntries, len(runner.recentTrace))
	}
	for _, item := range runner.recentTrace {
		if item.PageSignature == "" {
			t.Fatalf("recent trace page signature 不应为空")
		}
		if item.ActionKey == "" {
			t.Fatalf("recent trace action key 不应为空")
		}
	}
}

func TestRunnerPrefersContextAwareBlockRecoveryDecider(t *testing.T) {
	decider := &contextAwareRecoveryDecider{
		recoveryAwareDecider: recoveryAwareDecider{
			fakeDecider: fakeDecider{
				commands: []*types.ActionCommand{
					{Act: types.SCROLL_BOTTOM_UP, Pos: *types.NewRect(0, 0, 1, 1)},
				},
			},
			recoveryAction: &types.ActionCommand{Act: types.CLICK, Pos: *types.NewRect(0.1, 0.1, 0.2, 0.2)},
		},
	}
	driver := &fakeDriver{pageSource: &fakePageSource{xml: `<node class="MainActivity"/>`}}

	runner, err := NewRunner(decider, driver, Config{
		MaxSteps:               10,
		StepInterval:           0,
		KeepStepRecords:        true,
		StopOnCrash:            true,
		StopOnANR:              true,
		BlockNoChangeThreshold: 3,
	})
	if err != nil {
		t.Fatalf("创建 runner 失败: %v", err)
	}

	report, err := runner.Run(context.Background())
	if err != nil {
		t.Fatalf("运行 monkey 失败: %v", err)
	}
	if report.StopReason != StopCompleted {
		t.Fatalf("停止原因错误: %s", report.StopReason)
	}
	// 首次恢复应尝试 BACK，若仍未脱离阻塞，第二次恢复应走 planner → context-aware decider
	if decider.recoveryCalls == 0 {
		t.Fatalf("预期触发恢复决策")
	}
	if decider.lastContext.Mode != enginestate.ModeRecover {
		t.Fatalf("恢复上下文 mode 错误: %s", decider.lastContext.Mode)
	}
	if decider.lastContext.BlockReason == "" {
		t.Fatalf("恢复上下文应包含 block reason")
	}
	if len(decider.lastContext.RecentTrace) == 0 {
		t.Fatalf("恢复上下文应包含 recent trace")
	}
	if len(decider.lastContext.VisitStats.PageVisitCount) == 0 {
		t.Fatalf("恢复上下文应包含访问统计")
	}
}

func TestRunnerPrefersPlannerCandidateAndSkipsLLMOnHighConfidenceMemory(t *testing.T) {
	memoryCandidate := perception.NewCandidate(
		&types.ActionCommand{Act: types.CLICK, Pos: *types.NewRect(0.2, 0.2, 0.4, 0.4)},
		perception.SourceMemory,
		"点击主按钮",
		nil,
	)
	memoryCandidate.Confidence = 0.95

	decider := &plannerAwareRecoveryDecider{
		contextAwareRecoveryDecider: contextAwareRecoveryDecider{
			recoveryAwareDecider: recoveryAwareDecider{
				fakeDecider: fakeDecider{
					commands: []*types.ActionCommand{
						{Act: types.SCROLL_BOTTOM_UP, Pos: *types.NewRect(0, 0, 1, 1)},
					},
				},
				recoveryAction: &types.ActionCommand{Act: types.BACK},
			},
		},
		memoryCandidates:    []perception.Candidate{memoryCandidate},
		heuristicCandidates: nil,
		llmCandidates: []perception.Candidate{
			perception.NewCandidate(&types.ActionCommand{Act: types.BACK}, perception.SourceLLM, "llm back", nil),
		},
	}
	driver := &fakeDriver{pageSource: &fakePageSource{xml: `<node class="MainActivity"/>`}}

	// 需要足够步数：第一次恢复尝试 BACK，若仍未脱离阻塞，第二次恢复走 planner
	runner, err := NewRunner(decider, driver, Config{
		MaxSteps:               10,
		StepInterval:           0,
		KeepStepRecords:        true,
		StopOnCrash:            true,
		StopOnANR:              true,
		BlockNoChangeThreshold: 3,
	})
	if err != nil {
		t.Fatalf("创建 runner 失败: %v", err)
	}

	report, err := runner.Run(context.Background())
	if err != nil {
		t.Fatalf("运行 monkey 失败: %v", err)
	}
	if report.StopReason != StopCompleted {
		t.Fatalf("停止原因错误: %s", report.StopReason)
	}
	// 首次恢复尝试 BACK，第二次恢复走 planner → memory provider
	if decider.memoryCalls == 0 {
		t.Fatalf("预期调用 memory provider")
	}
	if decider.llmCalls != 0 {
		t.Fatalf("高置信 memory 命中后不应调用 llm provider，实际: %d", decider.llmCalls)
	}
	if decider.recoveryCalls != 0 {
		t.Fatalf("planner 命中后不应回退到 decider 恢复接口，实际调用: %d", decider.recoveryCalls)
	}
	if driver.clickCount == 0 {
		t.Fatalf("预期执行 planner 提供的 CLICK 恢复动作")
	}
}

func TestRunnerRecordsRecoveryOutcomeAfterPlannerSelection(t *testing.T) {
	memoryCandidate := perception.NewCandidate(
		&types.ActionCommand{Act: types.CLICK, Pos: *types.NewRect(0.1, 0.1, 0.2, 0.2)},
		perception.SourceMemory,
		"memory_click",
		map[string]string{"memory_key": "k1"},
	)
	decider := &plannerAwareRecoveryDecider{
		memoryCandidates: []perception.Candidate{memoryCandidate},
	}
	driver := &fakeDriver{pageSource: &fakePageSource{xml: `<node class="MainActivity"/>`}}

	runner, err := NewRunner(decider, driver, Config{
		StepInterval: 0,
		StopOnCrash:  true,
		StopOnANR:    true,
	})
	if err != nil {
		t.Fatalf("创建 runner 失败: %v", err)
	}

	// 第一轮恢复：BACK
	runner.handleBlockDetected("same_page_no_change")
	runner.handleBlockDetected("same_page_no_change")
	cmd, err := runner.nextCommandWithRecovery(1, session.PageSnapshot{
		PageName: "MainActivity",
		XML:      `<hierarchy/>`,
	}, "MainActivity", session.ActionInput{
		XMLDescOfGuiTree: `<hierarchy/>`,
	})
	if err != nil {
		t.Fatalf("获取恢复动作失败: %v", err)
	}
	if cmd == nil || cmd.Act != types.BACK {
		t.Fatalf("首次恢复应尝试 BACK，实际: %+v", cmd)
	}

	// 第二轮恢复：planner → memory candidate
	runner.handleBlockDetected("same_page_no_change")
	runner.handleBlockDetected("same_page_no_change")
	cmd, err = runner.nextCommandWithRecovery(2, session.PageSnapshot{
		PageName: "MainActivity",
		XML:      `<hierarchy/>`,
	}, "MainActivity", session.ActionInput{
		XMLDescOfGuiTree: `<hierarchy/>`,
	})
	if err != nil {
		t.Fatalf("获取恢复动作失败: %v", err)
	}
	if cmd == nil || cmd.Act != types.CLICK {
		t.Fatalf("预期 planner 返回 CLICK，实际: %+v", cmd)
	}

	runner.recordRecoveryOutcome(true)
	if decider.outcomeCalls != 1 {
		t.Fatalf("预期写回一次恢复结果，实际: %d", decider.outcomeCalls)
	}
	if !decider.lastOutcomeEscaped {
		t.Fatalf("预期写回 escaped=true")
	}
	if decider.lastOutcomeContext.Mode != enginestate.ModeRecover {
		t.Fatalf("写回上下文 mode 错误: %s", decider.lastOutcomeContext.Mode)
	}
	if decider.lastOutcomeContext.BlockReason != "same_page_no_change" {
		t.Fatalf("写回上下文 block reason 错误: %s", decider.lastOutcomeContext.BlockReason)
	}
	if decider.lastOutcomeItem.Source != perception.SourceMemory {
		t.Fatalf("写回候选来源错误: %s", decider.lastOutcomeItem.Source)
	}
}

func TestRunnerRecordRecoveryOutcomeSkipsWhenNoRecoveryAttempt(t *testing.T) {
	decider := &plannerAwareRecoveryDecider{}
	driver := &fakeDriver{pageSource: &fakePageSource{xml: `<node class="MainActivity"/>`}}
	runner, err := NewRunner(decider, driver, Config{
		StepInterval: 0,
		StopOnCrash:  true,
		StopOnANR:    true,
	})
	if err != nil {
		t.Fatalf("创建 runner 失败: %v", err)
	}

	runner.recordRecoveryOutcome(false)
	if decider.outcomeCalls != 0 {
		t.Fatalf("没有恢复尝试时不应写回，实际: %d", decider.outcomeCalls)
	}
}

func TestRunnerRecoveryPlannerDoesNotUseLLMConfig(t *testing.T) {
	decider := &plannerAwareRecoveryDecider{
		llmCandidates: []perception.Candidate{
			perception.NewCandidate(&types.ActionCommand{Act: types.BACK}, perception.SourceLLM, "llm_back", nil),
		},
	}
	runner, err := NewRunner(decider, &fakeDriver{}, Config{
		StepInterval:        0,
		StopOnCrash:         true,
		StopOnANR:           true,
		LLMBudgetMaxCalls:   1,
		LLMBudgetWindowStep: 100,
	})
	if err != nil {
		t.Fatalf("创建 runner 失败: %v", err)
	}

	planner := runner.getRecoveryPlanner()
	if planner == nil {
		t.Fatalf("预期创建 recovery planner")
	}

	_, err = planner.BuildRecoveryCandidates(enginestate.BuildTraversalContext(enginestate.BuildInput{
		Step:        1,
		Mode:        enginestate.ModeRecover,
		PageName:    "PageA",
		BlockReason: "same_page_no_change",
	}))
	if err != nil {
		t.Fatalf("第一次构建恢复候选失败: %v", err)
	}
	_, err = planner.BuildRecoveryCandidates(enginestate.BuildTraversalContext(enginestate.BuildInput{
		Step:        2,
		Mode:        enginestate.ModeRecover,
		PageName:    "PageA",
		BlockReason: "same_page_no_change",
	}))
	if err != nil {
		t.Fatalf("第二次构建恢复候选失败: %v", err)
	}

	if decider.llmCalls != 0 {
		t.Fatalf("llm 不应再参与恢复决策，实际调用: %d", decider.llmCalls)
	}
	if runner.recoveryLLMCallCount != 0 || runner.recoveryLLMDeniedCount != 0 {
		t.Fatalf("llm 恢复统计应为 0，实际 calls=%d denied=%d", runner.recoveryLLMCallCount, runner.recoveryLLMDeniedCount)
	}
}

func TestRunnerRecoveryPlannerDoesNotPassKnownActionsToLLMContext(t *testing.T) {
	successCmd := &types.ActionCommand{Act: types.BACK}
	failedCmd := &types.ActionCommand{Act: types.CLICK, Pos: *types.NewRect(0.1, 0.1, 0.2, 0.2)}
	decider := &plannerAwareRecoveryDecider{
		llmCandidates: []perception.Candidate{
			perception.NewCandidate(successCmd, perception.SourceLLM, "llm_back", nil),
		},
		persistedFailed: map[string]bool{
			failedCmd.ToJSON(): true,
		},
		persistedSuccess: map[string]bool{
			successCmd.ToJSON(): true,
		},
	}
	runner, err := NewRunner(decider, &fakeDriver{}, Config{
		StepInterval:      0,
		StopOnCrash:       true,
		StopOnANR:         true,
		LLMBudgetMaxCalls: 1,
	})
	if err != nil {
		t.Fatalf("创建 runner 失败: %v", err)
	}

	runner.pendingBlockRecovery = true
	_, err = runner.nextCommandWithRecovery(1, session.PageSnapshot{
		PageName: "MainActivity",
		XML:      `<hierarchy/>`,
	}, "MainActivity", session.ActionInput{XMLDescOfGuiTree: `<hierarchy/>`})
	if err != nil {
		t.Fatalf("恢复取动作失败: %v", err)
	}
	if len(decider.lastLLMContext.KnownFailedActions) != 0 || len(decider.lastLLMContext.KnownSuccessActions) != 0 {
		t.Fatalf("llm 不应再参与恢复决策上下文: %+v", decider.lastLLMContext)
	}
}

func TestRunnerRecoveryPlannerUsesFusedCandidateRanking(t *testing.T) {
	lowMemory := perception.NewCandidate(
		&types.ActionCommand{Act: types.BACK},
		perception.SourceMemory,
		"memory_back",
		nil,
	)
	lowMemory.Confidence = 0.1
	highHeuristic := perception.NewCandidate(
		&types.ActionCommand{Act: types.CLICK, Pos: *types.NewRect(0.2, 0.2, 0.4, 0.4)},
		perception.SourceHeuristic,
		"heuristic_click",
		nil,
	)
	highHeuristic.Confidence = 0.9
	highHeuristic.EscapeScore = 0.8

	decider := &plannerAwareRecoveryDecider{
		contextAwareRecoveryDecider: contextAwareRecoveryDecider{
			recoveryAwareDecider: recoveryAwareDecider{
				fakeDecider: fakeDecider{
					commands: []*types.ActionCommand{
						{Act: types.SCROLL_BOTTOM_UP, Pos: *types.NewRect(0, 0, 1, 1)},
					},
				},
			},
		},
		memoryCandidates:    []perception.Candidate{lowMemory},
		heuristicCandidates: []perception.Candidate{highHeuristic},
	}
	driver := &fakeDriver{pageSource: &fakePageSource{xml: `<node class="MainActivity"/>`}}

	// 需要足够步数：第一次恢复尝试 BACK，若仍未脱离阻塞，第二次恢复走 planner
	runner, err := NewRunner(decider, driver, Config{
		MaxSteps:               10,
		StepInterval:           0,
		KeepStepRecords:        true,
		StopOnCrash:            true,
		StopOnANR:              true,
		BlockNoChangeThreshold: 3,
	})
	if err != nil {
		t.Fatalf("创建 runner 失败: %v", err)
	}

	report, err := runner.Run(context.Background())
	if err != nil {
		t.Fatalf("运行 monkey 失败: %v", err)
	}
	if report.StopReason != StopCompleted {
		t.Fatalf("停止原因错误: %s", report.StopReason)
	}
	// 首次恢复是 BACK，第二次恢复走 planner → 融合排序选高分 CLICK
	if driver.clickCount == 0 {
		t.Fatalf("融合排序后应优先执行高分 CLICK 候选")
	}
}

func TestRunnerRecoveryPlannerPenalizesKnownFailedAction(t *testing.T) {
	backCandidate := perception.NewCandidate(
		&types.ActionCommand{Act: types.BACK},
		perception.SourceMemory,
		"memory_back",
		nil,
	)
	backCandidate.Confidence = 0.95

	clickCandidate := perception.NewCandidate(
		&types.ActionCommand{Act: types.CLICK, Pos: *types.NewRect(0.2, 0.2, 0.4, 0.4)},
		perception.SourceHeuristic,
		"heuristic_click",
		nil,
	)
	clickCandidate.Confidence = 0.6
	clickCandidate.EscapeScore = 0.2

	decider := &plannerAwareRecoveryDecider{
		memoryCandidates:    []perception.Candidate{backCandidate},
		heuristicCandidates: []perception.Candidate{clickCandidate},
	}
	runner, err := NewRunner(decider, &fakeDriver{}, Config{
		StepInterval: 0,
		StopOnCrash:  true,
		StopOnANR:    true,
	})
	if err != nil {
		t.Fatalf("创建 runner 失败: %v", err)
	}

	runner.handleBlockDetected("same_page_no_change")
	runner.handleBlockDetected("same_page_no_change")
	first, err := runner.nextCommandWithRecovery(1, session.PageSnapshot{
		PageName: "MainActivity",
		XML:      `<hierarchy/>`,
	}, "MainActivity", session.ActionInput{XMLDescOfGuiTree: `<hierarchy/>`})
	if err != nil {
		t.Fatalf("第一次恢复取动作失败: %v", err)
	}
	if first == nil || first.Act != types.BACK {
		t.Fatalf("第一次应优先选择高分 BACK，实际: %+v", first)
	}

	runner.recordRecoveryOutcome(false)

	runner.handleBlockDetected("same_page_no_change")
	second, err := runner.nextCommandWithRecovery(2, session.PageSnapshot{
		PageName: "MainActivity",
		XML:      `<hierarchy/>`,
	}, "MainActivity", session.ActionInput{XMLDescOfGuiTree: `<hierarchy/>`})
	if err != nil {
		t.Fatalf("第二次恢复取动作失败: %v", err)
	}
	if second == nil || second.Act != types.CLICK {
		t.Fatalf("失败惩罚后应避开 BACK 并选择 CLICK，实际: %+v", second)
	}
}

func TestRunnerRecoveryPlannerPenalizesPersistedFailedAction(t *testing.T) {
	backCandidate := perception.NewCandidate(
		&types.ActionCommand{Act: types.BACK},
		perception.SourceMemory,
		"memory_back",
		nil,
	)
	backCandidate.Confidence = 0.95

	clickCandidate := perception.NewCandidate(
		&types.ActionCommand{Act: types.CLICK, Pos: *types.NewRect(0.2, 0.2, 0.4, 0.4)},
		perception.SourceHeuristic,
		"heuristic_click",
		nil,
	)
	clickCandidate.Confidence = 0.6
	clickCandidate.EscapeScore = 0.2

	decider := &plannerAwareRecoveryDecider{
		memoryCandidates:    []perception.Candidate{backCandidate},
		heuristicCandidates: []perception.Candidate{clickCandidate},
		persistedFailed: map[string]bool{
			(&types.ActionCommand{Act: types.BACK}).ToJSON(): true,
		},
	}
	runner, err := NewRunner(decider, &fakeDriver{}, Config{
		StepInterval: 0,
		StopOnCrash:  true,
		StopOnANR:    true,
	})
	if err != nil {
		t.Fatalf("创建 runner 失败: %v", err)
	}

	// 第一轮恢复：BACK
	runner.handleBlockDetected("same_page_no_change")
	runner.handleBlockDetected("same_page_no_change")
	cmd, err := runner.nextCommandWithRecovery(1, session.PageSnapshot{
		PageName: "MainActivity",
		XML:      `<hierarchy/>`,
	}, "MainActivity", session.ActionInput{XMLDescOfGuiTree: `<hierarchy/>`})
	if err != nil {
		t.Fatalf("恢复取动作失败: %v", err)
	}
	if cmd == nil || cmd.Act != types.BACK {
		t.Fatalf("首次恢复应尝试 BACK，实际: %+v", cmd)
	}

	// 第二轮恢复：planner 惩罚了持久化失败的 BACK，应选择 CLICK
	runner.handleBlockDetected("same_page_no_change")
	runner.handleBlockDetected("same_page_no_change")
	cmd, err = runner.nextCommandWithRecovery(2, session.PageSnapshot{
		PageName: "MainActivity",
		XML:      `<hierarchy/>`,
	}, "MainActivity", session.ActionInput{XMLDescOfGuiTree: `<hierarchy/>`})
	if err != nil {
		t.Fatalf("恢复取动作失败: %v", err)
	}
	if cmd == nil || cmd.Act != types.CLICK {
		t.Fatalf("持久化失败惩罚后应避开 BACK 并选择 CLICK，实际: %+v", cmd)
	}
}

func TestRunnerCooldownModeSuppressesImmediateRecover(t *testing.T) {
	runner, err := NewRunner(&fakeDecider{}, &fakeDriver{}, Config{
		StepInterval:          0,
		StopOnCrash:           true,
		StopOnANR:             true,
		RecoveryCooldownSteps: 2,
	})
	if err != nil {
		t.Fatalf("创建 runner 失败: %v", err)
	}

	runner.handleBlockDetected("same_page_no_change")
	runner.handleBlockDetected("same_page_no_change")
	if runner.recoveryState.Mode() != TraversalModeRecover {
		t.Fatalf("预期进入 Recover，实际: %s", runner.recoveryState.Mode())
	}

	runner.handleProgress(true)
	if runner.recoveryState.Mode() != TraversalModeCooldown {
		t.Fatalf("恢复进展后预期进入 Cooldown，实际: %s", runner.recoveryState.Mode())
	}

	runner.handleBlockDetected("same_page_no_change")
	if runner.recoveryState.Mode() != TraversalModeSuspectBlocked {
		t.Fatalf("冷却期间遇阻塞应先进入 SuspectBlocked，实际: %s", runner.recoveryState.Mode())
	}
	if runner.pendingBlockRecovery {
		t.Fatalf("冷却期间遇阻塞不应立刻 pending recover")
	}
}
