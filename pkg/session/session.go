package session

import (
	"fmt"
	"os"
	"strings"
	"time"
	"trek/internal/engine/candidate"
	candidateproviders "trek/internal/engine/candidate/providers"
	"trek/internal/engine/decision"
	"trek/internal/engine/decision/monkey"
	"trek/internal/engine/decision/shared/types"
	"trek/internal/engine/memory"
	engineruntime "trek/internal/engine/runtime"
	enginestate "trek/internal/engine/state"
	"trek/internal/engine/traversal"
	"trek/logger"
)

// Config 决策会话配置
type Config struct {
	PackageName              string
	Algorithm                decision.AlgorithmType
	DeviceType               types.DeviceType
	RecoveryMemoryFile       string
	ExploreOCREndpoint       string
	ExploreOCRAPIKey         string
	ExploreOCRTimeout        time.Duration
	RecoveryLLMEndpoint      string
	RecoveryLLMAPIKey        string
	RecoveryLLMModel         string
	RecoveryLLMOpenAIModel   string
	RecoveryLLMOpenAIAPIKey  string
	RecoveryLLMOpenAIBaseURL string
	RecoveryLLMTimeout       time.Duration
}

// ActionInput 动作输入，包含 XML 描述的 GUI 树和可选截图
type ActionInput struct {
	XMLDescOfGuiTree string
	Screenshot       []byte
}

// PageInfo 页面信息，包含页面名和 XML
type PageInfo struct {
	PageName string
	XML      string
}

// PageSnapshot 页面快照，包含页面名、XML 和截图
type PageSnapshot struct {
	PageName   string
	XML        string
	Screenshot []byte
	Signature  string // pageSignature 缓存，避免每步重复 FNV 哈希
}

// StepResultInput 步骤结果输入，用于向引擎报告步骤执行结果
type StepResultInput struct {
	Step       int
	Action     *types.ActionCommand
	Success    bool
	Error      string
	DurationMs int64
	Crash      bool
	ANR        bool
	Before     PageSnapshot
	After      *PageSnapshot
}

// Session 对外稳定会话入口
type Session struct {
	config         Config
	memoryStore    *memory.Store
	memoryProvider *memory.Provider
	ocrProvider    candidateProvider
	llmProvider    recoveryLLMCandidateProvider
	traversalAlgo  traversal.TraversalAlgorithm
}

type candidateProvider interface {
	BuildCandidates(ctx enginestate.TraversalContext) ([]candidate.Candidate, error)
}

type recoveryLLMCandidateProvider interface {
	BuildCandidates(ctx enginestate.TraversalContext) ([]candidate.Candidate, error)
}

type stateReader interface {
	GetCurrentState() types.IState
}

// NewSession 创建新会话
func NewSession(config Config) (*Session, error) {
	if config.Algorithm == 0 {
		config.Algorithm = decision.AlgorithmReuse
	}
	if config.DeviceType == 0 {
		config.DeviceType = types.Phone
	}

	session := &Session{config: config}
	session.initRecoveryMemoryProvider()
	session.initExploreOCRProvider()
	session.initRecoveryLLMProvider()

	// 在 Reset 之前设置生命周期上下文，供插件 onInit/onDestroy 使用
	engineruntime.SetLifecycleContext(engineruntime.NewLifecycleContext(config.PackageName))

	if err := session.Reset(); err != nil {
		return nil, err
	}
	return session, nil
}

func (s *Session) initRecoveryMemoryProvider() {
	path := strings.TrimSpace(s.config.RecoveryMemoryFile)
	if path == "" {
		path = strings.TrimSpace(os.Getenv("RECOVERY_MEMORY_FILE"))
	}
	if path == "" {
		return
	}
	store, err := memory.NewStore(path)
	if err != nil {
		logger.Warnf("session 初始化 recovery memory 失败: path=%s err=%v", path, err)
		return
	}
	s.memoryStore = store
	s.memoryProvider = memory.NewProvider(store)
	logger.Infof("session recovery memory 已启用: path=%s", path)
}

func (s *Session) initExploreOCRProvider() {
	endpoint := strings.TrimSpace(s.config.ExploreOCREndpoint)
	if endpoint == "" {
		endpoint = strings.TrimSpace(os.Getenv("PADDLEOCR_API_URL"))
	}
	if endpoint == "" {
		return
	}
	apiKey := strings.TrimSpace(s.config.ExploreOCRAPIKey)
	if apiKey == "" {
		apiKey = strings.TrimSpace(os.Getenv("PADDLEOCR_API_KEY"))
	}
	provider, err := candidateproviders.NewOCRHTTPProvider(candidateproviders.OCRHTTPProviderConfig{
		Endpoint: endpoint,
		APIKey:   apiKey,
		Timeout:  s.config.ExploreOCRTimeout,
	})
	if err != nil {
		logger.Warnf("session 初始化 explore ocr provider 失败: endpoint=%s err=%v", endpoint, err)
		return
	}
	s.ocrProvider = provider
	logger.Infof("session explore ocr provider 已启用: endpoint=%s", endpoint)
}

func (s *Session) initRecoveryLLMProvider() {
	endpoint := strings.TrimSpace(s.config.RecoveryLLMEndpoint)
	if endpoint == "" {
		endpoint = strings.TrimSpace(os.Getenv("LLM_API_URL"))
	}
	apiKey := strings.TrimSpace(s.config.RecoveryLLMAPIKey)
	if apiKey == "" {
		apiKey = strings.TrimSpace(os.Getenv("LLM_API_KEY"))
	}
	model := strings.TrimSpace(s.config.RecoveryLLMModel)
	if model == "" {
		model = strings.TrimSpace(os.Getenv("LLM_MODEL"))
	}

	// 显式 endpoint 优先，便于兼容任意网关。
	if endpoint != "" {
		provider, err := candidateproviders.NewLLMHTTPProvider(candidateproviders.LLMHTTPProviderConfig{
			Endpoint: endpoint,
			APIKey:   apiKey,
			Model:    model,
			Timeout:  s.config.RecoveryLLMTimeout,
		})
		if err != nil {
			logger.Warnf("session 初始化 recovery llm provider 失败: endpoint=%s err=%v", endpoint, err)
			return
		}
		s.llmProvider = provider
		logger.Infof("session recovery llm provider 已启用: endpoint=%s", endpoint)
		return
	}

	openAIModel := strings.TrimSpace(s.config.RecoveryLLMOpenAIModel)
	if openAIModel == "" {
		openAIModel = strings.TrimSpace(os.Getenv("OPENAI_MODEL"))
	}
	if openAIModel == "" {
		return
	}
	openAIKey := strings.TrimSpace(s.config.RecoveryLLMOpenAIAPIKey)
	if openAIKey == "" {
		openAIKey = strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	}
	openAIBaseURL := strings.TrimSpace(s.config.RecoveryLLMOpenAIBaseURL)
	if openAIBaseURL == "" {
		openAIBaseURL = strings.TrimSpace(os.Getenv("OPENAI_API_URL"))
	}
	provider, err := candidateproviders.NewOpenAIResponsesProvider(candidateproviders.OpenAIResponsesProviderConfig{
		BaseURL: openAIBaseURL,
		APIKey:  openAIKey,
		Model:   openAIModel,
		Timeout: s.config.RecoveryLLMTimeout,
	})
	if err != nil {
		logger.Warnf("session 初始化 openai recovery llm provider 失败: model=%s err=%v", openAIModel, err)
		return
	}
	s.llmProvider = provider
	logger.Infof("session openai recovery llm provider 已启用: model=%s", openAIModel)
}

// Reset 重置引擎模型和脚本插件状态
func (s *Session) Reset() error {
	engineruntime.ResetModel()
	if err := engineruntime.InitAgent(s.config.Algorithm, s.config.PackageName, s.config.DeviceType); err != nil {
		return fmt.Errorf("重置时初始化决策代理失败: %w", err)
	}
	s.initTraversalAlgorithm()
	return nil
}

// LoadConfigFile 加载 JS 配置文件
func (s *Session) LoadConfigFile(path string) error {
	if engineruntime.GetModel() == nil {
		engineruntime.ResetModel()
		if err := engineruntime.InitAgent(s.config.Algorithm, s.config.PackageName, s.config.DeviceType); err != nil {
			return fmt.Errorf("初始化决策代理失败: %w", err)
		}
		s.initTraversalAlgorithm()
	} else if s.traversalAlgo == nil {
		s.initTraversalAlgorithm()
	}
	return engineruntime.LoadConfigFile(path)
}

func (s *Session) initTraversalAlgorithm() {
	s.traversalAlgo = nil
	model := engineruntime.GetModel()
	if model == nil {
		return
	}
	rawAgent := model.GetAgent(decision.DefaultDeviceID)
	agent, ok := rawAgent.(types.IAgent)
	if !ok || agent == nil {
		return
	}

	var provider traversal.StateProvider
	if reader, ok := rawAgent.(stateReader); ok && reader != nil {
		provider = &sessionStateProvider{reader: reader}
	}

	switch s.config.Algorithm {
	case decision.AlgorithmReuse:
		s.traversalAlgo = traversal.NewReuseAdapter(agent, provider)
	case decision.AlgorithmUctBandit:
		s.traversalAlgo = traversal.NewUCTBanditAdapter(agent, provider)
	case decision.AlgorithmRandom:
		s.traversalAlgo = monkey.NewMonkeyAdapter(agent, provider)
	default:
		s.traversalAlgo = nil
	}
}

// Deprecated: 使用 LoadConfigFile 代替
func (s *Session) LoadPreferenceFile(path string) error {
	return s.LoadConfigFile(path)
}

// NextActionJSON 返回 JSON 格式的下一步动作
func (s *Session) NextActionJSON(pageName string, xmlDescOfGuiTree string) (string, error) {
	operate, err := s.NextAction(pageName, xmlDescOfGuiTree)
	if err != nil {
		return "", err
	}
	return operate.ToJSON(), nil
}

// NextActionJSONWithInput 返回 JSON 格式的下一步动作（带输入）
func (s *Session) NextActionJSONWithInput(pageName string, input ActionInput) (string, error) {
	operate, err := s.NextActionWithInput(pageName, input)
	if err != nil {
		return "", err
	}
	return operate.ToJSON(), nil
}

// NextAction 返回下一步动作
func (s *Session) NextAction(pageName string, xmlDescOfGuiTree string) (*types.ActionCommand, error) {
	return s.NextActionWithInput(pageName, ActionInput{XMLDescOfGuiTree: xmlDescOfGuiTree})
}

// NextActionWithInput 返回下一步动作（带输入）
func (s *Session) NextActionWithInput(pageName string, input ActionInput) (*types.ActionCommand, error) {
	if strings.TrimSpace(pageName) == "" {
		return nil, fmt.Errorf("pageName is empty")
	}
	if strings.TrimSpace(input.XMLDescOfGuiTree) == "" && len(input.Screenshot) == 0 {
		return nil, fmt.Errorf("xmlDescOfGuiTree and screenshot are both empty")
	}

	operate := engineruntime.GetActionOptWithInput(pageName, input.XMLDescOfGuiTree, input.Screenshot)
	if operate == nil {
		return nil, fmt.Errorf("get nil action from engine runtime")
	}

	logger.Infof("session next action: page=%s cmd={%s}", pageName, operate.DetailLogString())

	return operate, nil
}

// NextBlockRecoveryAction 在 Runner 检测到阻塞后提供兜底动作。
// 该链路会显式标记 block_recovery 上下文，便于插件触发 LLM 规划。
func (s *Session) NextBlockRecoveryAction(pageName string, input ActionInput) (*types.ActionCommand, error) {
	if strings.TrimSpace(pageName) == "" {
		return nil, fmt.Errorf("pageName 不能为空")
	}
	if strings.TrimSpace(input.XMLDescOfGuiTree) == "" && len(input.Screenshot) == 0 {
		return nil, fmt.Errorf("xmlDescOfGuiTree 和 screenshot 不能同时为空")
	}

	operate := engineruntime.GetBlockRecoveryActionOptWithInput(pageName, input.XMLDescOfGuiTree, input.Screenshot)
	if operate == nil {
		return nil, nil
	}
	if isAppRestartAction(operate.Act) {
		logger.Warnf("session block recovery rejected app restart action: page=%s act=%s", pageName, operate.Act.String())
		return nil, nil
	}

	logger.Infof("session block recovery action: page=%s cmd={%s}", pageName, operate.DetailLogString())
	return operate, nil
}

// NextBlockRecoveryActionWithContext 在 Runner 检测到阻塞后提供兜底动作（上下文感知版本）。
func (s *Session) NextBlockRecoveryActionWithContext(ctx enginestate.TraversalContext, input ActionInput) (*types.ActionCommand, error) {
	return s.NextBlockRecoveryAction(ctx.PageName, input)
}

// BuildMemoryRecoveryCandidates 将 memory 经验转为恢复候选，供 runner 的 RecoveryPlanner 调用。
func (s *Session) BuildMemoryRecoveryCandidates(ctx enginestate.TraversalContext) ([]candidate.Candidate, error) {
	if s == nil || s.memoryProvider == nil {
		return nil, nil
	}
	return s.memoryProvider.BuildCandidates(ctx)
}

// BuildHeuristicRecoveryCandidates 通过插件 block_recovery 链路生成启发式恢复候选。
func (s *Session) BuildHeuristicRecoveryCandidates(ctx enginestate.TraversalContext) ([]candidate.Candidate, error) {
	if s == nil {
		return nil, nil
	}
	if strings.TrimSpace(ctx.XML) == "" && len(ctx.Screenshot) == 0 {
		return nil, nil
	}

	pageName := strings.TrimSpace(ctx.PageName)
	if pageName == "" {
		pageName = "UnknownPage"
	}

	operate := engineruntime.GetBlockRecoveryActionOptWithInput(pageName, ctx.XML, ctx.Screenshot)
	if operate == nil || isAppRestartAction(operate.Act) {
		return nil, nil
	}

	cmd := *operate
	return []candidate.Candidate{
		{
			Command: &cmd,
			Source:  candidate.SourceHeuristic,
			Intent:  "plugin_block_recovery",
			Metadata: map[string]string{
				"provider": "session_plugin",
			},
		},
	}, nil
}

// BuildKnownFailedRecoveryActions 返回恢复阶段已知失败动作，用于候选融合惩罚。
func (s *Session) BuildKnownFailedRecoveryActions(ctx enginestate.TraversalContext) (map[string]bool, error) {
	result := make(map[string]bool)
	if s == nil || s.memoryStore == nil {
		return result, nil
	}
	items := s.memoryStore.Find(ctx)
	for _, item := range items {
		if item.Candidate.Command == nil {
			continue
		}
		if item.FailCount > item.SuccessCount || strings.EqualFold(item.Outcome, memory.OutcomeFailed) {
			result[item.Candidate.Command.ToJSON()] = true
		}
	}
	return result, nil
}

// BuildKnownSuccessfulRecoveryActions 返回恢复阶段已知成功动作，用于提示词增强。
func (s *Session) BuildKnownSuccessfulRecoveryActions(ctx enginestate.TraversalContext) (map[string]bool, error) {
	result := make(map[string]bool)
	if s == nil || s.memoryStore == nil {
		return result, nil
	}
	items := s.memoryStore.Find(ctx)
	for _, item := range items {
		if item.Candidate.Command == nil {
			continue
		}
		if item.SuccessCount > item.FailCount || strings.EqualFold(item.Outcome, memory.OutcomeEscaped) {
			result[item.Candidate.Command.ToJSON()] = true
		}
	}
	return result, nil
}

// BuildLLMRecoveryCandidates 调用外部 LLM 服务构建恢复候选。
func (s *Session) BuildLLMRecoveryCandidates(ctx enginestate.TraversalContext) ([]candidate.Candidate, error) {
	if s == nil || s.llmProvider == nil {
		return nil, nil
	}
	return s.llmProvider.BuildCandidates(ctx)
}

// BuildAlgorithmCandidates 将统一算法适配器产出的候选暴露给 Runner 的探索融合链路。
func (s *Session) BuildAlgorithmCandidates(ctx enginestate.TraversalContext) ([]candidate.Candidate, error) {
	if s == nil {
		return nil, nil
	}
	items := make([]candidate.Candidate, 0)
	if s.traversalAlgo == nil {
		s.initTraversalAlgorithm()
	}
	if s.traversalAlgo != nil {
		algoItems, err := s.traversalAlgo.ProposeCandidates(ctx)
		if err != nil {
			return nil, err
		}
		items = append(items, algoItems...)
	}
	if s.ocrProvider != nil && len(ctx.Screenshot) > 0 {
		ocrItems, err := s.ocrProvider.BuildCandidates(ctx)
		if err != nil {
			logger.Warnf("session 构建 OCR 候选失败: page=%s err=%v", ctx.PageName, err)
		} else {
			items = append(items, ocrItems...)
		}
	}
	if len(items) == 0 {
		return nil, nil
	}
	return items, nil
}

// SelectRecoveryAction 基于 traversal 算法从融合恢复候选中选择最终动作。
func (s *Session) SelectRecoveryAction(ctx enginestate.TraversalContext, candidates []candidate.Candidate) (*types.ActionCommand, error) {
	if s == nil || len(candidates) == 0 {
		return nil, nil
	}
	if s.traversalAlgo == nil {
		s.initTraversalAlgorithm()
	}
	if s.traversalAlgo == nil {
		return nil, nil
	}
	return s.traversalAlgo.SelectAction(ctx, candidates)
}

// ObserveTraversalOutcome 回写统一动作结果，供 traversal 算法在线学习。
func (s *Session) ObserveTraversalOutcome(ctx enginestate.TraversalContext, action *types.ActionCommand, outcome traversal.ActionOutcome) error {
	if s == nil || action == nil {
		return nil
	}
	if s.traversalAlgo == nil {
		s.initTraversalAlgorithm()
	}
	if s.traversalAlgo == nil {
		return nil
	}
	return s.traversalAlgo.ObserveOutcome(ctx, action, outcome)
}

// RecordRecoveryMemoryOutcome 写回一次恢复动作结果，用于跨会话复用。
func (s *Session) RecordRecoveryMemoryOutcome(ctx enginestate.TraversalContext, item candidate.Candidate, escaped bool) error {
	if s == nil || s.memoryStore == nil || item.Command == nil {
		return nil
	}
	traceSignature := enginestate.BuildTraceSignature(ctx.RecentTrace)
	outcome := memory.OutcomeFailed
	successCount := 0
	failCount := 1
	escapeScore := 0.0
	if escaped {
		outcome = memory.OutcomeEscaped
		successCount = 1
		failCount = 0
		escapeScore = 1
	}
	record := memory.RecoveryMemoryRecord{
		MemoryKey:        memory.BuildMemoryKey(ctx.PageSignature, ctx.BlockReason, traceSignature, string(ctx.Mode)),
		PageSignature:    ctx.PageSignature,
		ClusterSignature: ctx.ClusterSignature,
		BlockReason:      ctx.BlockReason,
		TraceSignature:   traceSignature,
		Mode:             string(ctx.Mode),
		Candidate:        item,
		Outcome:          outcome,
		EscapeScore:      escapeScore,
		SuccessCount:     successCount,
		FailCount:        failCount,
		LastUsedAt:       time.Now(),
		CreatedAt:        time.Now(),
	}
	return s.memoryStore.AppendOutcome(record)
}

// RecordCandidateEnhancementOutcome 写回候选增强动作结果，沉淀正负样本。
func (s *Session) RecordCandidateEnhancementOutcome(ctx enginestate.TraversalContext, item candidate.Candidate, improved bool) error {
	if s == nil || s.memoryStore == nil || item.Command == nil {
		return nil
	}
	traceSignature := enginestate.BuildTraceSignature(ctx.RecentTrace)
	outcome := memory.OutcomeFailed
	successCount := 0
	failCount := 1
	escapeScore := 0.0
	if improved {
		outcome = memory.OutcomeEscaped
		successCount = 1
		failCount = 0
		escapeScore = 1
	}
	blockReason := strings.TrimSpace(ctx.BlockReason)
	if blockReason == "" {
		blockReason = memory.BlockReasonCandidateEnhancement
	}
	record := memory.RecoveryMemoryRecord{
		MemoryKey:        memory.BuildMemoryKey(ctx.PageSignature, blockReason, traceSignature, string(ctx.Mode)),
		PageSignature:    ctx.PageSignature,
		ClusterSignature: ctx.ClusterSignature,
		BlockReason:      blockReason,
		TraceSignature:   traceSignature,
		Mode:             string(ctx.Mode),
		Candidate:        item,
		Outcome:          outcome,
		EscapeScore:      escapeScore,
		SuccessCount:     successCount,
		FailCount:        failCount,
		LastUsedAt:       time.Now(),
		CreatedAt:        time.Now(),
	}
	return s.memoryStore.AppendOutcome(record)
}

// SetObservationMode 设置感知模式（xml-only / image-only / hybrid）。
func (s *Session) SetObservationMode(mode string) error {
	return engineruntime.SetObservationMode(mode)
}

// GetObservationMode 获取当前感知模式
func (s *Session) GetObservationMode() string {
	return engineruntime.GetObservationMode()
}

// CheckPointInBlackRects 检查点是否在黑名单区域内
func (s *Session) CheckPointInBlackRects(pageName string, point types.Point) bool {
	return engineruntime.CheckPointIsInBlackRects(pageName, float32(point.X), float32(point.Y))
}

// NativeVersion 获取被测应用的原生版本号
func (s *Session) NativeVersion() string {
	return engineruntime.GetNativeVersion()
}

// ResolvePageNameWithInput 通过 Goja 插件的 resolvePageName 钩子解析自定义页面名。
func (s *Session) ResolvePageNameWithInput(pageName string, input ActionInput) (string, error) {
	name, err := engineruntime.ResolvePageNameWithInput(pageName, input.XMLDescOfGuiTree, input.Screenshot)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(name), nil
}

// TransformPageInfoWithInput 通过 Goja 插件转换页面信息
func (s *Session) TransformPageInfoWithInput(pageName string, input ActionInput) (PageInfo, error) {
	if strings.TrimSpace(pageName) == "" {
		return PageInfo{}, fmt.Errorf("pageName is empty")
	}
	if strings.TrimSpace(input.XMLDescOfGuiTree) == "" && len(input.Screenshot) == 0 {
		return PageInfo{}, fmt.Errorf("xmlDescOfGuiTree and screenshot are both empty")
	}
	newPage, newXML, err := engineruntime.TransformPageInfoWithInput(pageName, input.XMLDescOfGuiTree, input.Screenshot)
	if err != nil {
		return PageInfo{}, err
	}
	return PageInfo{
		PageName: newPage,
		XML:      newXML,
	}, nil
}

// OnStepResult 通过 Goja 插件通知步骤结果
func (s *Session) OnStepResult(input StepResultInput) error {
	runtimeInput := engineruntime.StepResultInput{
		Step:       input.Step,
		Action:     input.Action,
		Success:    input.Success,
		Error:      input.Error,
		DurationMs: input.DurationMs,
		Crash:      input.Crash,
		ANR:        input.ANR,
		Before: engineruntime.PageSnapshotInput{
			PageName:   input.Before.PageName,
			XML:        input.Before.XML,
			Screenshot: input.Before.Screenshot,
		},
	}
	if input.After != nil {
		runtimeInput.After = &engineruntime.PageSnapshotInput{
			PageName:   input.After.PageName,
			XML:        input.After.XML,
			Screenshot: input.After.Screenshot,
		}
	}
	return engineruntime.OnStepResult(runtimeInput)
}

func isAppRestartAction(act types.ActionType) bool {
	return act == types.START || act == types.RESTART || act == types.CLEAN_RESTART
}

type sessionStateProvider struct {
	reader stateReader
}

func (p *sessionStateProvider) CurrentState() types.IState {
	if p == nil || p.reader == nil {
		return nil
	}
	return p.reader.GetCurrentState()
}
