package session

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
	"trek/internal/engine/candidate"
	"trek/internal/engine/decision"
	types2 "trek/internal/engine/decision/shared/types"
	"trek/internal/engine/memory"
	enginestate "trek/internal/engine/state"
	"trek/internal/engine/traversal"
)

func closeSessionMemoryStore(s *Session) {
	if s == nil || s.memoryStore == nil {
		return
	}
	_ = s.memoryStore.Close()
}

type mockTraversalAlgorithm struct {
	proposeFn func(ctx enginestate.TraversalContext) ([]candidate.Candidate, error)
	selectFn  func(ctx enginestate.TraversalContext, candidates []candidate.Candidate) (*types2.ActionCommand, error)
	observeFn func(ctx enginestate.TraversalContext, action *types2.ActionCommand, outcome traversal.ActionOutcome) error
}

func (m *mockTraversalAlgorithm) Name() string { return "mock" }

func (m *mockTraversalAlgorithm) ProposeCandidates(ctx enginestate.TraversalContext) ([]candidate.Candidate, error) {
	if m != nil && m.proposeFn != nil {
		return m.proposeFn(ctx)
	}
	return nil, nil
}

func (m *mockTraversalAlgorithm) SelectAction(ctx enginestate.TraversalContext, candidates []candidate.Candidate) (*types2.ActionCommand, error) {
	if m != nil && m.selectFn != nil {
		return m.selectFn(ctx, candidates)
	}
	return nil, nil
}

func (m *mockTraversalAlgorithm) ObserveOutcome(ctx enginestate.TraversalContext, action *types2.ActionCommand, outcome traversal.ActionOutcome) error {
	if m != nil && m.observeFn != nil {
		return m.observeFn(ctx, action, outcome)
	}
	return nil
}

func TestSessionNextAction(t *testing.T) {
	session := NewSession(Config{
		PackageName: "com.demo",
		Algorithm:   decision.AlgorithmReuse,
		DeviceType:  types2.Phone,
	})

	action, err := session.NextAction("LoginActivity", `
<hierarchy>
	<node class="android.widget.FrameLayout" resource-id="" content-desc="" text="" clickable="false" long-clickable="false" checkable="false" enabled="true" bounds="[0,0][1080,1920]">
		<node class="android.widget.Button" resource-id="com.demo:id/login" content-desc="鐧诲綍" text="鐧诲綍" clickable="true" long-clickable="false" checkable="false" enabled="true" bounds="[10,20][110,120]"/>
	</node>
</hierarchy>`)
	if err != nil {
		t.Fatalf("鑾峰彇涓嬩竴姝ュ姩浣滃け锟? %v", err)
	}

	if action == nil {
		t.Fatalf("棰勬湡杩斿洖闈炵┖鍔ㄤ綔")
	}
}

func TestSessionCheckPointInBlackRects(t *testing.T) {
	session := NewSession(Config{PackageName: "com.demo"})

	configPath := filepath.Join(t.TempDir(), "mix.js")
	configContent := `const config = {
  black_rects: {
    LoginActivity: [[0, 0, 100, 100]]
  }
};`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("鍐欏叆娴嬭瘯鏂囦欢澶辫触: %v", err)
	}
	if err := session.LoadConfigFile(configPath); err != nil {
		t.Fatalf("鍔犺浇閰嶇疆澶辫触: %v", err)
	}

	if !session.CheckPointInBlackRects("LoginActivity", types2.Point{X: 50, Y: 50}) {
		t.Fatalf("鐐逛綅搴斿懡涓粦鍚嶅崟鍖哄煙")
	}
}

func TestSessionTransformPageInfoWithInput(t *testing.T) {
	session := NewSession(Config{PackageName: "com.demo"})

	configPath := filepath.Join(t.TempDir(), "mix.js")
	configContent := `const plugin = {
  transformPage(ctx) {
    return {
      page_name: ctx.page.name + "_v2",
      xml: ctx.page.xml.replace("foo", "bar"),
    }
  }
};`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("鍐欏叆娴嬭瘯鏂囦欢澶辫触: %v", err)
	}
	if err := session.LoadConfigFile(configPath); err != nil {
		t.Fatalf("鍔犺浇閰嶇疆澶辫触: %v", err)
	}

	info, err := session.TransformPageInfoWithInput("MainActivity", ActionInput{XMLDescOfGuiTree: `<node text="foo"/>`})
	if err != nil {
		t.Fatalf("TransformPageInfoWithInput 澶辫触: %v", err)
	}
	if info.PageName != "MainActivity_v2" {
		t.Fatalf("椤甸潰鍚嶆敼閫犵粨鏋滀笉绗﹀悎棰勬湡: %s", info.PageName)
	}
	if info.XML != `<node text="bar"/>` {
		t.Fatalf("xml 鏀归€犵粨鏋滀笉绗﹀悎棰勬湡: %s", info.XML)
	}
}

func TestSessionBeforeDecideUsesGojaPluginAction(t *testing.T) {
	session := NewSession(Config{PackageName: "com.demo"})

	configPath := filepath.Join(t.TempDir(), "plugin.js")
	configContent := `const plugin = {
  beforeDecide(ctx) {
    return trek.action.back()
  }
};`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("鍐欏叆娴嬭瘯鏂囦欢澶辫触: %v", err)
	}
	if err := session.LoadConfigFile(configPath); err != nil {
		t.Fatalf("鍔犺浇閰嶇疆澶辫触: %v", err)
	}

	action, err := session.NextAction("MainActivity", `<hierarchy><node class="android.widget.TextView" bounds="[0,0][10,10]"/></hierarchy>`)
	if err != nil {
		t.Fatalf("鑾峰彇鍔ㄤ綔澶辫触: %v", err)
	}
	if action.Act != types2.BACK {
		t.Fatalf("鎻掍欢搴旇鐩栦负 BACK锛屽疄锟? %s", action.Act.String())
	}
}

func TestSessionOnStepResultFeedsGojaPluginState(t *testing.T) {
	session := NewSession(Config{PackageName: "com.demo"})

	configPath := filepath.Join(t.TempDir(), "plugin.js")
	configContent := `const plugin = {
  onStepResult(ctx) {
    if (ctx.result.crash && ctx.result.after.xml.indexOf("After") >= 0) {
      trek.state.set("should_back", true)
    }
  },
  beforeDecide(ctx) {
    if (trek.state.get("should_back")) return trek.action.back()
    return null
  }
};`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("鍐欏叆娴嬭瘯鏂囦欢澶辫触: %v", err)
	}
	if err := session.LoadConfigFile(configPath); err != nil {
		t.Fatalf("鍔犺浇閰嶇疆澶辫触: %v", err)
	}

	after := PageSnapshot{PageName: "After", XML: `<hierarchy text="After"/>`, Screenshot: []byte{1, 2, 3}}
	if err := session.OnStepResult(StepResultInput{
		Step:    1,
		Action:  &types2.ActionCommand{Act: types2.CLICK, Pos: *types2.NewRect(0, 0, 10, 10)},
		Success: false,
		Crash:   true,
		Before:  PageSnapshot{PageName: "Before", XML: `<hierarchy/>`},
		After:   &after,
	}); err != nil {
		t.Fatalf("OnStepResult 澶辫触: %v", err)
	}

	action, err := session.NextAction("MainActivity", `<hierarchy><node class="android.widget.TextView" bounds="[0,0][10,10]"/></hierarchy>`)
	if err != nil {
		t.Fatalf("鑾峰彇鍔ㄤ綔澶辫触: %v", err)
	}
	if action.Act != types2.BACK {
		t.Fatalf("onStepResult 鐘舵€佸簲椹卞姩涓嬩竴锟?BACK锛屽疄锟? %s", action.Act.String())
	}
}

func TestSessionNextBlockRecoveryActionUsesPluginBlockRecoveryContext(t *testing.T) {
	session := NewSession(Config{PackageName: "com.demo"})

	configPath := filepath.Join(t.TempDir(), "plugin.js")
	configContent := `const plugin = {
  beforeDecide(ctx) {
    if (ctx.runtime.block_recovery && ctx.runtime.block_recovery.requested) {
      return trek.action.back()
    }
    return null
  }
};`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("写入测试文件失败: %v", err)
	}
	if err := session.LoadConfigFile(configPath); err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}

	action, err := session.NextBlockRecoveryAction("MainActivity", ActionInput{
		XMLDescOfGuiTree: `<hierarchy><node class="android.widget.TextView" bounds="[0,0][10,10]"/></hierarchy>`,
	})
	if err != nil {
		t.Fatalf("获取阻塞恢复动作失败: %v", err)
	}
	if action == nil || action.Act != types2.BACK {
		t.Fatalf("预期阻塞恢复返回 BACK，实际: %+v", action)
	}
}

func TestSessionNextBlockRecoveryActionRejectsRestartActions(t *testing.T) {
	session := NewSession(Config{PackageName: "com.demo"})

	configPath := filepath.Join(t.TempDir(), "plugin.js")
	configContent := `const plugin = {
  beforeDecide(ctx) {
    if (ctx.runtime.block_recovery && ctx.runtime.block_recovery.requested) {
      return trek.action.restart()
    }
    return null
  }
};`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("写入测试文件失败: %v", err)
	}
	if err := session.LoadConfigFile(configPath); err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}

	action, err := session.NextBlockRecoveryAction("MainActivity", ActionInput{
		XMLDescOfGuiTree: `<hierarchy><node class="android.widget.TextView" bounds="[0,0][10,10]"/></hierarchy>`,
	})
	if err != nil {
		t.Fatalf("获取阻塞恢复动作失败: %v", err)
	}
	if action != nil {
		t.Fatalf("阻塞恢复不应返回重启动作，实际: %+v", action)
	}
}

func TestSessionBuildMemoryRecoveryCandidates(t *testing.T) {
	memoryPath := filepath.Join(t.TempDir(), "recovery.jsonl")
	store, err := memory.NewStore(memoryPath)
	if err != nil {
		t.Fatalf("初始化 memory store 失败: %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})
	record := memory.RecoveryMemoryRecord{
		MemoryKey:        memory.BuildMemoryKey("page.sig", "same_page_no_change", "back", "recover"),
		PageSignature:    "page.sig",
		ClusterSignature: "cluster.sig",
		BlockReason:      "same_page_no_change",
		TraceSignature:   "back",
		Mode:             "recover",
		Candidate: candidate.NewCandidate(
			&types2.ActionCommand{Act: types2.BACK},
			candidate.SourceAlgorithm,
			"回退上一层",
			map[string]string{"seed": "1"},
		),
		Outcome:      memory.OutcomeEscaped,
		EscapeScore:  0.8,
		SuccessCount: 3,
		FailCount:    1,
		LastUsedAt:   time.Now(),
		CreatedAt:    time.Now(),
	}
	if err := store.Append(record); err != nil {
		t.Fatalf("写入 memory 记录失败: %v", err)
	}

	session := NewSession(Config{
		PackageName:        "com.demo",
		RecoveryMemoryFile: memoryPath,
	})
	t.Cleanup(func() {
		closeSessionMemoryStore(session)
	})
	items, err := session.BuildMemoryRecoveryCandidates(enginestate.TraversalContext{
		Mode:             "recover",
		PageSignature:    "page.sig",
		ClusterSignature: "cluster.sig",
		BlockReason:      "same_page_no_change",
		RecentTrace: []enginestate.ActionTrace{
			{ActionKey: "back"},
		},
	})
	if err != nil {
		t.Fatalf("构建 memory 恢复候选失败: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("预期命中 1 条候选，实际: %d", len(items))
	}
	if items[0].Source != candidate.SourceMemory {
		t.Fatalf("候选来源应为 memory，实际: %s", items[0].Source)
	}
	if items[0].Command == nil || items[0].Command.Act != types2.BACK {
		t.Fatalf("候选动作应为 BACK，实际: %+v", items[0].Command)
	}
	if items[0].Metadata["memory_key"] == "" {
		t.Fatalf("候选缺少 memory_key 元数据")
	}
}

func TestSessionBuildHeuristicRecoveryCandidates(t *testing.T) {
	session := NewSession(Config{PackageName: "com.demo"})

	configPath := filepath.Join(t.TempDir(), "plugin.js")
	configContent := `const plugin = {
  beforeDecide(ctx) {
    if (ctx.runtime.block_recovery && ctx.runtime.block_recovery.requested) {
      return trek.action.back()
    }
    return null
  }
};`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("写入测试文件失败: %v", err)
	}
	if err := session.LoadConfigFile(configPath); err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}

	items, err := session.BuildHeuristicRecoveryCandidates(enginestate.TraversalContext{
		Mode:       "Recover",
		PageName:   "MainActivity",
		XML:        `<hierarchy><node class="android.widget.TextView"/></hierarchy>`,
		Screenshot: []byte{1, 2, 3},
	})
	if err != nil {
		t.Fatalf("构建 heuristic 恢复候选失败: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("预期 1 条 heuristic 候选，实际: %d", len(items))
	}
	if items[0].Source != candidate.SourceHeuristic {
		t.Fatalf("候选来源应为 heuristic，实际: %s", items[0].Source)
	}
	if items[0].Command == nil || items[0].Command.Act != types2.BACK {
		t.Fatalf("候选动作应为 BACK，实际: %+v", items[0].Command)
	}
}

func TestSessionBuildHeuristicRecoveryCandidatesRejectRestart(t *testing.T) {
	session := NewSession(Config{PackageName: "com.demo"})

	configPath := filepath.Join(t.TempDir(), "plugin.js")
	configContent := `const plugin = {
  beforeDecide(ctx) {
    if (ctx.runtime.block_recovery && ctx.runtime.block_recovery.requested) {
      return trek.action.restart()
    }
    return null
  }
};`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("写入测试文件失败: %v", err)
	}
	if err := session.LoadConfigFile(configPath); err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}

	items, err := session.BuildHeuristicRecoveryCandidates(enginestate.TraversalContext{
		Mode:     "Recover",
		PageName: "MainActivity",
		XML:      `<hierarchy><node class="android.widget.TextView"/></hierarchy>`,
	})
	if err != nil {
		t.Fatalf("构建 heuristic 恢复候选失败: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("重启动作不应进入恢复候选，实际: %+v", items)
	}
}

func TestSessionSelectRecoveryActionPrefersAlgorithmCandidate(t *testing.T) {
	session := NewSession(Config{
		PackageName: "com.demo",
		Algorithm:   decision.AlgorithmReuse,
		DeviceType:  types2.Phone,
	})
	ctx := enginestate.TraversalContext{Mode: "Recover"}
	items := []candidate.Candidate{
		{
			Command:    &types2.ActionCommand{Act: types2.BACK},
			Source:     candidate.SourceAlgorithm,
			Confidence: 0.3,
		},
		{
			Command:    &types2.ActionCommand{Act: types2.CLICK, Pos: *types2.NewRect(0.1, 0.1, 0.2, 0.2)},
			Source:     candidate.SourceLLM,
			Confidence: 0.9,
		},
	}
	cmd, err := session.SelectRecoveryAction(ctx, items)
	if err != nil {
		t.Fatalf("SelectRecoveryAction 失败: %v", err)
	}
	if cmd == nil || cmd.Act != types2.BACK {
		t.Fatalf("预期优先选择 algorithm 候选 BACK，实际: %+v", cmd)
	}
}

func TestSessionBuildAlgorithmCandidatesDelegatesTraversalAlgorithm(t *testing.T) {
	session := NewSession(Config{
		PackageName: "com.demo",
		Algorithm:   decision.AlgorithmReuse,
		DeviceType:  types2.Phone,
	})
	called := false
	expected := []candidate.Candidate{
		{
			Command:    &types2.ActionCommand{Act: types2.BACK},
			Source:     candidate.SourceAlgorithm,
			Confidence: 0.8,
		},
	}
	session.traversalAlgo = &mockTraversalAlgorithm{
		proposeFn: func(ctx enginestate.TraversalContext) ([]candidate.Candidate, error) {
			called = true
			return expected, nil
		},
	}

	items, err := session.BuildAlgorithmCandidates(enginestate.TraversalContext{
		Mode:     "Explore",
		PageName: "MainActivity",
	})
	if err != nil {
		t.Fatalf("BuildAlgorithmCandidates 失败: %v", err)
	}
	if !called {
		t.Fatalf("预期调用 traversalAlgo.ProposeCandidates")
	}
	if len(items) != 1 || items[0].Command == nil || items[0].Command.Act != types2.BACK {
		t.Fatalf("算法候选返回不符合预期: %+v", items)
	}
}

func TestSessionObserveTraversalOutcomeNoError(t *testing.T) {
	session := NewSession(Config{
		PackageName: "com.demo",
		Algorithm:   decision.AlgorithmReuse,
		DeviceType:  types2.Phone,
	})
	err := session.ObserveTraversalOutcome(
		enginestate.TraversalContext{Mode: "Explore", PageName: "MainActivity"},
		&types2.ActionCommand{Act: types2.CLICK, Pos: *types2.NewRect(0.1, 0.1, 0.2, 0.2)},
		traversal.OutcomeNewState,
	)
	if err != nil {
		t.Fatalf("ObserveTraversalOutcome 不应报错: %v", err)
	}
}

func TestSessionRecordRecoveryMemoryOutcome(t *testing.T) {
	memoryPath := filepath.Join(t.TempDir(), "recovery.jsonl")
	session := NewSession(Config{
		PackageName:        "com.demo",
		RecoveryMemoryFile: memoryPath,
	})
	t.Cleanup(func() {
		closeSessionMemoryStore(session)
	})

	err := session.RecordRecoveryMemoryOutcome(
		enginestate.TraversalContext{
			Mode:             "Recover",
			PageSignature:    "page.sig",
			ClusterSignature: "cluster.sig",
			BlockReason:      "same_page_no_change",
			RecentTrace: []enginestate.ActionTrace{
				{ActionKey: "back"},
				{ActionKey: "click"},
			},
		},
		candidate.Candidate{
			Command: &types2.ActionCommand{Act: types2.BACK},
			Source:  candidate.SourceHeuristic,
		},
		true,
	)
	if err != nil {
		t.Fatalf("写回 recovery memory 失败: %v", err)
	}

	store, err := memory.NewStore(memoryPath)
	if err != nil {
		t.Fatalf("读取 memory store 失败: %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})
	all := store.All()
	if len(all) != 1 {
		t.Fatalf("预期写入 1 条记录，实际: %d", len(all))
	}
	if all[0].Outcome != memory.OutcomeEscaped {
		t.Fatalf("预期 outcome=escaped，实际: %s", all[0].Outcome)
	}
	if all[0].SuccessCount != 1 || all[0].FailCount != 0 {
		t.Fatalf("预期成功/失败计数为 1/0，实际: %d/%d", all[0].SuccessCount, all[0].FailCount)
	}
	if all[0].TraceSignature != "back>click" {
		t.Fatalf("预期 trace 签名为 back>click，实际: %s", all[0].TraceSignature)
	}
}

func TestSessionRecordRecoveryMemoryOutcomeAggregatesCounts(t *testing.T) {
	memoryPath := filepath.Join(t.TempDir(), "recovery.jsonl")
	session := NewSession(Config{
		PackageName:        "com.demo",
		RecoveryMemoryFile: memoryPath,
	})
	t.Cleanup(func() {
		closeSessionMemoryStore(session)
	})
	ctx := enginestate.TraversalContext{
		Mode:             "Recover",
		PageSignature:    "page.sig",
		ClusterSignature: "cluster.sig",
		BlockReason:      "same_page_no_change",
		RecentTrace: []enginestate.ActionTrace{
			{ActionKey: "back"},
		},
	}
	item := candidate.Candidate{
		Command: &types2.ActionCommand{Act: types2.BACK},
		Source:  candidate.SourceMemory,
	}

	if err := session.RecordRecoveryMemoryOutcome(ctx, item, true); err != nil {
		t.Fatalf("写回成功样本失败: %v", err)
	}
	if err := session.RecordRecoveryMemoryOutcome(ctx, item, false); err != nil {
		t.Fatalf("写回失败样本失败: %v", err)
	}

	store, err := memory.NewStore(memoryPath)
	if err != nil {
		t.Fatalf("读取 memory store 失败: %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})
	all := store.All()
	if len(all) != 1 {
		t.Fatalf("聚合后预期 1 条记录，实际: %d", len(all))
	}
	if all[0].SuccessCount != 1 || all[0].FailCount != 1 {
		t.Fatalf("聚合计数错误: success=%d fail=%d", all[0].SuccessCount, all[0].FailCount)
	}
	if all[0].Outcome != memory.OutcomeFailed {
		t.Fatalf("最新 outcome 应为 failed，实际: %s", all[0].Outcome)
	}
}

func TestSessionRecordCandidateEnhancementOutcome(t *testing.T) {
	memoryPath := filepath.Join(t.TempDir(), "recovery.jsonl")
	session := NewSession(Config{
		PackageName:        "com.demo",
		RecoveryMemoryFile: memoryPath,
	})
	t.Cleanup(func() {
		closeSessionMemoryStore(session)
	})
	ctx := enginestate.TraversalContext{
		Mode:             "Explore",
		PageSignature:    "page.sig",
		ClusterSignature: "cluster.sig",
		RecentTrace: []enginestate.ActionTrace{
			{ActionKey: "click"},
		},
	}
	item := candidate.Candidate{
		Command: &types2.ActionCommand{Act: types2.CLICK, Pos: *types2.NewRect(0.1, 0.1, 0.2, 0.2)},
		Source:  candidate.SourceLLM,
	}
	if err := session.RecordCandidateEnhancementOutcome(ctx, item, true); err != nil {
		t.Fatalf("写回候选增强结果失败: %v", err)
	}

	store, err := memory.NewStore(memoryPath)
	if err != nil {
		t.Fatalf("读取 memory store 失败: %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})
	all := store.All()
	if len(all) != 1 {
		t.Fatalf("预期写入 1 条记录，实际: %d", len(all))
	}
	if all[0].BlockReason != memory.BlockReasonCandidateEnhancement {
		t.Fatalf("预期 block_reason=%s，实际: %s", memory.BlockReasonCandidateEnhancement, all[0].BlockReason)
	}
	if all[0].Outcome != memory.OutcomeEscaped || all[0].SuccessCount != 1 {
		t.Fatalf("预期写入 improved 正样本，实际: outcome=%s success=%d", all[0].Outcome, all[0].SuccessCount)
	}
}

func TestSessionBuildKnownFailedRecoveryActions(t *testing.T) {
	memoryPath := filepath.Join(t.TempDir(), "recovery.jsonl")
	session := NewSession(Config{
		PackageName:        "com.demo",
		RecoveryMemoryFile: memoryPath,
	})
	t.Cleanup(func() {
		closeSessionMemoryStore(session)
	})
	ctx := enginestate.TraversalContext{
		Mode:             "Recover",
		PageSignature:    "page.sig",
		ClusterSignature: "cluster.sig",
		BlockReason:      "same_page_no_change",
		RecentTrace: []enginestate.ActionTrace{
			{ActionKey: "back"},
		},
	}
	item := candidate.Candidate{
		Command: &types2.ActionCommand{Act: types2.BACK},
		Source:  candidate.SourceMemory,
	}
	if err := session.RecordRecoveryMemoryOutcome(ctx, item, false); err != nil {
		t.Fatalf("写回失败样本失败: %v", err)
	}

	known, err := session.BuildKnownFailedRecoveryActions(ctx)
	if err != nil {
		t.Fatalf("提取失败动作失败: %v", err)
	}
	if !known[item.Command.ToJSON()] {
		t.Fatalf("预期包含 BACK 失败动作 key")
	}
}

func TestSessionBuildKnownSuccessfulRecoveryActions(t *testing.T) {
	memoryPath := filepath.Join(t.TempDir(), "recovery.jsonl")
	session := NewSession(Config{
		PackageName:        "com.demo",
		RecoveryMemoryFile: memoryPath,
	})
	t.Cleanup(func() {
		closeSessionMemoryStore(session)
	})
	ctx := enginestate.TraversalContext{
		Mode:             "Recover",
		PageSignature:    "page.sig",
		ClusterSignature: "cluster.sig",
		BlockReason:      "same_page_no_change",
		RecentTrace: []enginestate.ActionTrace{
			{ActionKey: "back"},
		},
	}
	item := candidate.Candidate{
		Command: &types2.ActionCommand{Act: types2.BACK},
		Source:  candidate.SourceMemory,
	}
	if err := session.RecordRecoveryMemoryOutcome(ctx, item, true); err != nil {
		t.Fatalf("写回成功样本失败: %v", err)
	}

	known, err := session.BuildKnownSuccessfulRecoveryActions(ctx)
	if err != nil {
		t.Fatalf("提取成功动作失败: %v", err)
	}
	if !known[item.Command.ToJSON()] {
		t.Fatalf("预期包含 BACK 成功动作 key")
	}
}

func TestSessionBuildLLMRecoveryCandidates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method 错误: %s", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("Authorization 错误: %s", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"candidates": []map[string]any{
				{
					"intent":      "返回",
					"action_type": "BACK",
					"confidence":  0.7,
					"reason":      "弹窗疑似遮挡",
				},
			},
		})
	}))
	defer server.Close()

	session := NewSession(Config{
		PackageName:         "com.demo",
		RecoveryLLMEndpoint: server.URL,
		RecoveryLLMAPIKey:   "test-key",
		RecoveryLLMModel:    "gpt-x",
		RecoveryLLMTimeout:  2 * time.Second,
	})
	items, err := session.BuildLLMRecoveryCandidates(enginestate.TraversalContext{
		Step:        5,
		Mode:        "Recover",
		PageName:    "MainActivity",
		BlockReason: "same_page_no_change",
	})
	if err != nil {
		t.Fatalf("构建 llm 恢复候选失败: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("预期 1 条 llm 候选，实际: %d", len(items))
	}
	if items[0].Source != candidate.SourceLLM {
		t.Fatalf("候选来源应为 llm，实际: %s", items[0].Source)
	}
	if items[0].Command == nil || items[0].Command.Act != types2.BACK {
		t.Fatalf("候选动作应为 BACK，实际: %+v", items[0].Command)
	}
	if items[0].Metadata["llm_reason"] == "" {
		t.Fatalf("预期包含 llm_reason 元数据")
	}
}

func TestSessionBuildLLMRecoveryCandidatesWithOpenAIResponsesProvider(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method 错误: %s", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer sk-test" {
			t.Fatalf("Authorization 错误: %s", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"output_text": `{"candidates":[{"intent":"返回","action_type":"BACK","confidence":0.8}]}`,
		})
	}))
	defer server.Close()

	session := NewSession(Config{
		PackageName:              "com.demo",
		RecoveryLLMOpenAIModel:   "gpt-4.1-mini",
		RecoveryLLMOpenAIAPIKey:  "sk-test",
		RecoveryLLMOpenAIBaseURL: server.URL,
		RecoveryLLMTimeout:       2 * time.Second,
	})
	items, err := session.BuildLLMRecoveryCandidates(enginestate.TraversalContext{
		Step:        6,
		Mode:        "Recover",
		PageName:    "MainActivity",
		BlockReason: "same_page_no_change",
	})
	if err != nil {
		t.Fatalf("openai provider 构建候选失败: %v", err)
	}
	if len(items) != 1 || items[0].Command == nil || items[0].Command.Act != types2.BACK {
		t.Fatalf("openai provider 返回候选不符合预期: %+v", items)
	}
}
