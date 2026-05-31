package coordinator

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"trek/internal/engine/core/types"
	"trek/internal/engine/decision"
	"trek/internal/engine/decision/monkey"
	_ "trek/internal/engine/decision/reuse"
	_ "trek/internal/engine/decision/uctbandit"
	"trek/internal/engine/decision/shared/elements"
	"trek/internal/engine/memory"
	"trek/internal/engine/pagecache"
	"trek/internal/engine/perception"
	candidateproviders "trek/internal/engine/perception/providers"
	engineruntime "trek/internal/engine/runtime"
	enginestate "trek/internal/engine/state"
	"trek/internal/engine/traversal"
	visionfingerprint "trek/internal/vision/fingerprint"
	"trek/logger"
)

// Config 决策协调器配置
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
	PageControlStrategy      string
	PageControlCacheFile     string
	PageControlCacheTTL      time.Duration
}

// ActionInput 动作输入，包含 XML 描述的 GUI 树和可选截图。
type ActionInput struct {
	XMLDescOfGuiTree string
	Screenshot       []byte
}

// PageInfo 页面信息，包含页面名和 XML。
type PageInfo struct {
	PageName          string
	XML               string
	CacheHit          bool            // 是否命中页面理解缓存
	ScriptTransformed bool            // 是否经过 goja 脚本转换
	Element           types.IElement  // 解析后的元素树
}

// PageSnapshot 页面快照，包含页面名、XML 和截图。
type PageSnapshot struct {
	PageName   string
	XML        string
	Screenshot []byte
	Signature  string          // pageSignature 缓存，避免每步重复 FNV 哈希
	Element    types.IElement  // 解析后的元素树
}

// StepResultInput 步骤结果输入，用于向引擎报告步骤执行结果。
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

// Coordinator 对外稳定协调入口。
type Coordinator struct {
	config           Config
	runtime          *engineruntime.Runtime
	memoryStore      *memory.Store
	memoryProvider   *memory.Provider
	pageControlStore *pagecache.Store
	ocrProvider      candidateProvider
	llmProvider      pageControlCandidateProvider
	traversalAlgo    traversal.TraversalAlgorithm
	pageControlCache sync.Map
}

type pageControlCacheEntry struct {
	SyntheticXML string
	RefreshedAt  time.Time
	HitCount     int
}

type candidateProvider interface {
	BuildCandidates(ctx enginestate.TraversalContext) ([]perception.Candidate, error)
}

type pageControlCandidateProvider interface {
	DetectPageControls(ctx enginestate.TraversalContext) ([]perception.Candidate, error)
}

type stateReader interface {
	GetCurrentState() types.IState
}

// New 创建新的协调器。
func New(config Config) (*Coordinator, error) {
	if config.Algorithm == 0 {
		config.Algorithm = decision.AlgorithmReuse
	}
	if config.DeviceType == 0 {
		config.DeviceType = types.Phone
	}

	coordinator := &Coordinator{
		config:  config,
		runtime: engineruntime.New(config.PackageName),
	}
	coordinator.initRecoveryMemoryProvider()
	coordinator.initExploreOCRProvider()
	coordinator.initPageControlLLMProvider()
	coordinator.initPageControlCacheStore()

	// 在 Reset 之前设置生命周期上下文，供插件 onInit/onDestroy 使用
	coordinator.runtime.SetLifecycleContext(coordinator.runtime.NewLifecycleContext())

	if err := coordinator.Reset(); err != nil {
		return nil, err
	}
	return coordinator, nil
}

// NewSession 为旧名称兼容入口，后续优先使用 New。
func NewSession(config Config) (*Coordinator, error) {
	return New(config)
}

// Close 关闭协调器持有的持久化资源。
func (s *Coordinator) Close() error {
	if s == nil {
		return nil
	}
	var firstErr error
	if s.memoryStore != nil {
		if err := s.memoryStore.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if s.pageControlStore != nil {
		if err := s.pageControlStore.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (s *Coordinator) initRecoveryMemoryProvider() {
	path := strings.TrimSpace(s.config.RecoveryMemoryFile)
	if path == "" {
		path = strings.TrimSpace(os.Getenv("RECOVERY_MEMORY_FILE"))
	}
	if path == "" {
		return
	}
	store, err := memory.NewStore(path)
	if err != nil {
		logger.Warnf("coordinator 初始化 recovery memory 失败: path=%s err=%v", path, err)
		return
	}
	s.memoryStore = store
	s.memoryProvider = memory.NewProvider(store)
	logger.Infof("coordinator recovery memory 已启用: path=%s", path)
}

func (s *Coordinator) initExploreOCRProvider() {
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
		logger.Warnf("coordinator 初始化 explore ocr provider 失败: endpoint=%s err=%v", endpoint, err)
		return
	}
	s.ocrProvider = provider
	logger.Infof("coordinator explore ocr provider 已启用: endpoint=%s", endpoint)
}

func (s *Coordinator) initPageControlLLMProvider() {
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
			logger.Warnf("coordinator 初始化 page control llm provider 失败: endpoint=%s err=%v", endpoint, err)
			return
		}
		s.llmProvider = provider
		logger.Infof("coordinator page control llm provider 已启用: endpoint=%s", endpoint)
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
		logger.Warnf("coordinator 初始化 openai page control llm provider 失败: model=%s err=%v", openAIModel, err)
		return
	}
	s.llmProvider = provider
	logger.Infof("coordinator openai page control llm provider 已启用: model=%s", openAIModel)
}

func (s *Coordinator) initPageControlCacheStore() {
	path := strings.TrimSpace(s.config.PageControlCacheFile)
	if path == "" {
		path = strings.TrimSpace(os.Getenv("PAGE_CONTROL_CACHE_FILE"))
	}
	if path == "" {
		return
	}
	store, err := pagecache.NewStore(path)
	if err != nil {
		logger.Warnf("coordinator 初始化页面控件持久化缓存失败: path=%s err=%v", path, err)
		return
	}
	s.pageControlStore = store
	logger.Infof("coordinator 页面控件持久化缓存已启用: path=%s", path)
}

// Reset 重置引擎模型和脚本插件状态
func (s *Coordinator) Reset() error {
	s.runtime.ResetModel()
	if err := s.runtime.InitAgent(s.config.Algorithm, s.config.PackageName, s.config.DeviceType); err != nil {
		return fmt.Errorf("重置时初始化决策代理失败: %w", err)
	}
	s.initTraversalAlgorithm()
	return nil
}

// LoadConfigFile 加载 JS 配置文件
func (s *Coordinator) LoadConfigFile(path string) error {
	if s.runtime.GetModel() == nil {
		s.runtime.ResetModel()
		if err := s.runtime.InitAgent(s.config.Algorithm, s.config.PackageName, s.config.DeviceType); err != nil {
			return fmt.Errorf("初始化决策代理失败: %w", err)
		}
		s.initTraversalAlgorithm()
	} else if s.traversalAlgo == nil {
		s.initTraversalAlgorithm()
	}
	return s.runtime.LoadConfigFile(path)
}

func (s *Coordinator) initTraversalAlgorithm() {
	s.traversalAlgo = nil
	model := s.runtime.GetModel()
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
func (s *Coordinator) LoadPreferenceFile(path string) error {
	return s.LoadConfigFile(path)
}

// NextActionJSON 返回 JSON 格式的下一步动作
func (s *Coordinator) NextActionJSON(pageName string, xmlDescOfGuiTree string) (string, error) {
	operate, err := s.NextAction(pageName, xmlDescOfGuiTree)
	if err != nil {
		return "", err
	}
	return operate.ToJSON(), nil
}

// NextActionJSONWithInput 返回 JSON 格式的下一步动作（带输入）
func (s *Coordinator) NextActionJSONWithInput(pageName string, input ActionInput) (string, error) {
	operate, err := s.NextActionWithInput(pageName, input)
	if err != nil {
		return "", err
	}
	return operate.ToJSON(), nil
}

// NextAction 返回下一步动作
func (s *Coordinator) NextAction(pageName string, xmlDescOfGuiTree string) (*types.ActionCommand, error) {
	return s.NextActionWithInput(pageName, ActionInput{XMLDescOfGuiTree: xmlDescOfGuiTree})
}

// NextActionWithInput 返回下一步动作（带输入）
func (s *Coordinator) NextActionWithInput(pageName string, input ActionInput) (*types.ActionCommand, error) {
	if strings.TrimSpace(pageName) == "" {
		return nil, fmt.Errorf("pageName is empty")
	}
	if strings.TrimSpace(input.XMLDescOfGuiTree) == "" && len(input.Screenshot) == 0 {
		return nil, fmt.Errorf("xmlDescOfGuiTree and screenshot are both empty")
	}
	if strings.TrimSpace(input.XMLDescOfGuiTree) == "" && len(input.Screenshot) > 0 {
		pageInfo, err := s.buildPageInfoByStrategy(pageName, input, false, false)
		if err != nil {
			logger.Warnf("coordinator 基于图片生成页面控件信息失败，继续使用空 XML: page=%s err=%v", pageName, err)
		} else {
			pageName = pageInfo.PageName
			input.XMLDescOfGuiTree = pageInfo.XML
		}
	}

	operate := s.runtime.GetActionOptWithInput(pageName, input.XMLDescOfGuiTree, input.Screenshot)
	if operate == nil {
		return nil, fmt.Errorf("get nil action from engine runtime")
	}

	logger.Infof("coordinator next action: page=%s cmd={%s}", pageName, operate.DetailLogString())

	return operate, nil
}

// NextBlockRecoveryAction 在 Runner 检测到阻塞后提供兜底动作。
// 该链路会显式标记 block_recovery 上下文，便于插件触发自定义恢复规划。
func (s *Coordinator) NextBlockRecoveryAction(pageName string, input ActionInput) (*types.ActionCommand, error) {
	if strings.TrimSpace(pageName) == "" {
		return nil, fmt.Errorf("pageName 不能为空")
	}
	if strings.TrimSpace(input.XMLDescOfGuiTree) == "" && len(input.Screenshot) == 0 {
		return nil, fmt.Errorf("xmlDescOfGuiTree 和 screenshot 不能同时为空")
	}
	if strings.TrimSpace(input.XMLDescOfGuiTree) == "" && len(input.Screenshot) > 0 {
		pageInfo, err := s.buildPageInfoByStrategy(pageName, input, false, true)
		if err != nil {
			logger.Warnf("coordinator 阻塞恢复基于图片生成页面控件信息失败，继续使用空 XML: page=%s err=%v", pageName, err)
		} else {
			pageName = pageInfo.PageName
			input.XMLDescOfGuiTree = pageInfo.XML
		}
	}

	operate := s.runtime.GetBlockRecoveryActionOptWithInput(pageName, input.XMLDescOfGuiTree, input.Screenshot)
	if operate == nil {
		return nil, nil
	}
	if isAppRestartAction(operate.Act) {
		logger.Warnf("coordinator block recovery rejected app restart action: page=%s act=%s", pageName, operate.Act.String())
		return nil, nil
	}

	logger.Infof("coordinator block recovery action: page=%s cmd={%s}", pageName, operate.DetailLogString())
	return operate, nil
}

// NextBlockRecoveryActionWithContext 在 Runner 检测到阻塞后提供兜底动作（上下文感知版本）。
func (s *Coordinator) NextBlockRecoveryActionWithContext(ctx enginestate.TraversalContext, input ActionInput) (*types.ActionCommand, error) {
	return s.NextBlockRecoveryAction(ctx.PageName, input)
}

// BuildMemoryRecoveryCandidates 将 memory 经验转为恢复候选，供 runner 的 RecoveryPlanner 调用。
func (s *Coordinator) BuildMemoryRecoveryCandidates(ctx enginestate.TraversalContext) ([]perception.Candidate, error) {
	if s == nil || s.memoryProvider == nil {
		return nil, nil
	}
	return s.memoryProvider.BuildCandidates(ctx)
}

// BuildHeuristicRecoveryCandidates 通过插件 block_recovery 链路生成启发式恢复候选。
func (s *Coordinator) BuildHeuristicRecoveryCandidates(ctx enginestate.TraversalContext) ([]perception.Candidate, error) {
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

	operate := s.runtime.GetBlockRecoveryActionOptWithInput(pageName, ctx.XML, ctx.Screenshot)
	if operate == nil || isAppRestartAction(operate.Act) {
		return nil, nil
	}

	cmd := *operate
	return []perception.Candidate{
		{
			Command: &cmd,
			Source:  perception.SourceHeuristic,
			Intent:  "plugin_block_recovery",
			Metadata: map[string]string{
				"provider": "session_plugin",
			},
		},
	}, nil
}

// BuildKnownFailedRecoveryActions 返回恢复阶段已知失败动作，用于候选融合惩罚。
func (s *Coordinator) BuildKnownFailedRecoveryActions(ctx enginestate.TraversalContext) (map[string]bool, error) {
	failed, _, err := s.BuildKnownRecoveryActions(ctx)
	return failed, err
}

// BuildKnownSuccessfulRecoveryActions 返回恢复阶段已知成功动作，用于提示词增强。
func (s *Coordinator) BuildKnownSuccessfulRecoveryActions(ctx enginestate.TraversalContext) (map[string]bool, error) {
	_, success, err := s.BuildKnownRecoveryActions(ctx)
	return success, err
}

// BuildKnownRecoveryActions 单次 Find 返回失败/成功两个集合，避免重复遍历记忆库。
func (s *Coordinator) BuildKnownRecoveryActions(ctx enginestate.TraversalContext) (failed, success map[string]bool, err error) {
	failed = make(map[string]bool)
	success = make(map[string]bool)
	if s == nil || s.memoryStore == nil {
		return failed, success, nil
	}
	items := s.memoryStore.Find(ctx)
	for _, item := range items {
		if item.Item.Command == nil {
			continue
		}
		key := item.Item.Command.ToJSON()
		if item.FailCount > item.SuccessCount || strings.EqualFold(item.Outcome, memory.OutcomeFailed) {
			failed[key] = true
		}
		if item.SuccessCount > item.FailCount || strings.EqualFold(item.Outcome, memory.OutcomeEscaped) {
			success[key] = true
		}
	}
	return failed, success, nil
}

// BuildLLMRecoveryCandidates 已废弃。
// LLM 不再直接参与动作决策，仅用于页面控件检测。
func (s *Coordinator) BuildLLMRecoveryCandidates(ctx enginestate.TraversalContext) ([]perception.Candidate, error) {
	_ = ctx
	return nil, nil
}

// BuildAlgorithmCandidates 将统一算法适配器产出的候选暴露给 Runner 的探索融合链路。
func (s *Coordinator) BuildAlgorithmCandidates(ctx enginestate.TraversalContext) ([]perception.Candidate, error) {
	if s == nil {
		return nil, nil
	}
	items := make([]perception.Candidate, 0)
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
			logger.Warnf("coordinator 构建 OCR 候选失败: page=%s err=%v", ctx.PageName, err)
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
func (s *Coordinator) SelectRecoveryAction(ctx enginestate.TraversalContext, candidates []perception.Candidate) (*types.ActionCommand, error) {
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
func (s *Coordinator) ObserveTraversalOutcome(ctx enginestate.TraversalContext, action *types.ActionCommand, outcome traversal.ActionOutcome) error {
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
func (s *Coordinator) RecordRecoveryMemoryOutcome(ctx enginestate.TraversalContext, item perception.Candidate, escaped bool) error {
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
		Item:             item,
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
func (s *Coordinator) RecordCandidateEnhancementOutcome(ctx enginestate.TraversalContext, item perception.Candidate, improved bool) error {
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
		Item:             item,
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
func (s *Coordinator) SetObservationMode(mode string) error {
	return s.runtime.SetObservationMode(mode)
}

// GetObservationMode 获取当前感知模式
func (s *Coordinator) GetObservationMode() string {
	return s.runtime.GetObservationMode()
}

// CheckPointInBlackRects 检查点是否在黑名单区域内
func (s *Coordinator) CheckPointInBlackRects(pageName string, point types.Point) bool {
	return s.runtime.CheckPointIsInBlackRects(pageName, float32(point.X), float32(point.Y))
}

// NativeVersion 获取被测应用的原生版本号
func (s *Coordinator) NativeVersion() string {
	return s.runtime.GetNativeVersion()
}

// ResolvePageNameWithInput 通过 Goja 插件的 resolvePageName 钩子解析自定义页面名。
func (s *Coordinator) ResolvePageNameWithInput(pageName string, input ActionInput) (string, error) {
	name, err := s.runtime.ResolvePageNameWithInput(pageName, input.XMLDescOfGuiTree, input.Screenshot)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(name), nil
}

// TransformPageInfoWithInput 通过 Goja 插件转换页面信息
func (s *Coordinator) TransformPageInfoWithInput(pageName string, input ActionInput) (PageInfo, error) {
	if strings.TrimSpace(pageName) == "" {
		return PageInfo{}, fmt.Errorf("pageName is empty")
	}
	if strings.TrimSpace(input.XMLDescOfGuiTree) == "" && len(input.Screenshot) == 0 {
		return PageInfo{}, fmt.Errorf("xmlDescOfGuiTree and screenshot are both empty")
	}
	return s.buildPageInfoByStrategy(pageName, input, true, false)
}

func (s *Coordinator) buildPageInfoByStrategy(pageName string, input ActionInput, applyPluginTransform bool, forceRefresh bool) (PageInfo, error) {
	resolvedPageName := strings.TrimSpace(pageName)
	resolvedXML := strings.TrimSpace(input.XMLDescOfGuiTree)
	scriptTransformed := false

	// 只有存在脚本插件时才调用转换
	if applyPluginTransform && s.runtime.HasScriptPlugin() {
		newPage, newXML, err := s.runtime.TransformPageInfoWithInput(pageName, input.XMLDescOfGuiTree, input.Screenshot)
		if err != nil {
			return PageInfo{}, err
		}
		// 脚本返回了非空值就算转换
		if strings.TrimSpace(newPage) != "" {
			resolvedPageName = strings.TrimSpace(newPage)
			scriptTransformed = true
		}
		if strings.TrimSpace(newXML) != "" {
			resolvedXML = strings.TrimSpace(newXML)
			scriptTransformed = true
		}
	}

	info := PageInfo{
		PageName:          resolvedPageName,
		XML:               resolvedXML,
		ScriptTransformed: scriptTransformed,
	}

	// 为 raw XML 或脚本转换后的 XML 创建 IElement
	if strings.TrimSpace(resolvedXML) != "" {
		if elem, err := elements.CreateAndroidElementFromXml(resolvedXML); err == nil {
			info.Element = elem
		}
	}

	if len(input.Screenshot) == 0 {
		return info, nil
	}

	strategy := normalizePageControlStrategy(s.config.PageControlStrategy)
	if strategy == pageControlStrategyRaw {
		return info, nil
	}
	if cachedXML, ok := s.loadPageControlCache(strategy, input.Screenshot, forceRefresh); ok {
		logger.Debugf("coordinator 页面控件缓存命中: strategy=%s page=%s", strategy, resolvedPageName)
		info.XML = cachedXML
		info.CacheHit = true
		// 为缓存的 XML 创建 IElement
		if elem, err := elements.CreateAndroidElementFromXml(cachedXML); err == nil {
			info.Element = elem
		}
		return info, nil
	}

	candidates, err := s.buildPageControlCandidates(strategy, enginestate.TraversalContext{
		Mode:       "Explore",
		PageName:   resolvedPageName,
		XML:        resolvedXML,
		Screenshot: input.Screenshot,
	})
	if err != nil {
		if strings.TrimSpace(info.XML) != "" {
			logger.Warnf("coordinator 页面控件策略回退到 raw: strategy=%s page=%s err=%v", strategy, resolvedPageName, err)
			return info, nil
		}
		return PageInfo{}, err
	}

	syntheticXML, elem := buildSyntheticXMLFromCandidates(strategy, candidates, input.Screenshot)
	if strings.TrimSpace(syntheticXML) == "" {
		return info, nil
	}
	s.storePageControlCache(strategy, input.Screenshot, syntheticXML)
	info.XML = syntheticXML
	info.Element = elem
	return info, nil
}

func (s *Coordinator) loadPageControlCache(strategy string, screenshot []byte, forceRefresh bool) (string, bool) {
	cacheKey := pageControlCacheKey(strategy, screenshot)
	if cacheKey == "" {
		return "", false
	}
	if forceRefresh {
		logger.Debugf("coordinator 页面控件缓存主动刷新: reason=block_recovery strategy=%s", strategy)
		return "", false
	}
	cached, ok := s.pageControlCache.Load(cacheKey)
	if !ok {
		if s.pageControlStore != nil {
			if persistent, ok := s.pageControlStore.Get(cacheKey); ok && strings.TrimSpace(persistent.SyntheticXML) != "" {
				// Get() 已递增持久层 HitCount，此处 +1 用于 L1 的 TTL 判断
				hitCount := persistent.HitCount
				if hitCount <= 0 {
					hitCount = 1
				}
				entry := pageControlCacheEntry{
					SyntheticXML: persistent.SyntheticXML,
					RefreshedAt:  chooseCacheRefreshTime(persistent.RefreshedAt, persistent.CreatedAt),
					HitCount:     hitCount,
				}
				if s.isPageControlCacheExpired(entry.RefreshedAt, hitCount) {
					return "", false
				}
				s.pageControlCache.Store(cacheKey, entry)
				return entry.SyntheticXML, true
			}
		}
		return "", false
	}
	entry, ok := cached.(pageControlCacheEntry)
	if !ok || strings.TrimSpace(entry.SyntheticXML) == "" {
		return "", false
	}
	// L1 命中：递增内存命中计数
	entry.HitCount++
	if entry.HitCount <= 1 {
		entry.HitCount = 1
	}
	s.pageControlCache.Store(cacheKey, entry)
	if s.isPageControlCacheExpired(entry.RefreshedAt, entry.HitCount) {
		logger.Debugf("coordinator 页面控件缓存 TTL 已过期: strategy=%s hitCount=%d", strategy, entry.HitCount)
		return "", false
	}
	return entry.SyntheticXML, true
}

func (s *Coordinator) storePageControlCache(strategy string, screenshot []byte, syntheticXML string) {
	cacheKey := pageControlCacheKey(strategy, screenshot)
	if cacheKey == "" || strings.TrimSpace(syntheticXML) == "" {
		return
	}
	now := time.Now()
	s.pageControlCache.Store(cacheKey, pageControlCacheEntry{
		SyntheticXML: syntheticXML,
		RefreshedAt:  now,
		HitCount:     1,
	})
	if s.pageControlStore != nil {
		_ = s.pageControlStore.Put(pagecache.Entry{
			CacheKey:     cacheKey,
			Strategy:     strategy,
			Fingerprint:  pageControlCacheFingerprint(screenshot),
			SyntheticXML: syntheticXML,
			RefreshedAt:  now,
			LastUsedAt:   now,
			CreatedAt:    now,
			HitCount:     1,
		})
	}
}

// InvalidatePageControlCache 使指定截图对应的页面理解缓存失效。
func (s *Coordinator) InvalidatePageControlCache(screenshot []byte) {
	if s == nil || len(screenshot) == 0 {
		return
	}
	strategy := normalizePageControlStrategy(s.config.PageControlStrategy)
	cacheKey := pageControlCacheKey(strategy, screenshot)
	if cacheKey == "" {
		return
	}
	s.pageControlCache.Delete(cacheKey)
	if s.pageControlStore != nil {
		_ = s.pageControlStore.Delete(cacheKey)
	}
}

func (s *Coordinator) buildPageControlCandidates(strategy string, ctx enginestate.TraversalContext) ([]perception.Candidate, error) {
	switch strategy {
	case pageControlStrategyOCR:
		if s == nil || s.ocrProvider == nil {
			return nil, fmt.Errorf("ocr 页面控件策略未启用，请配置 OCR provider")
		}
		return s.ocrProvider.BuildCandidates(ctx)
	case pageControlStrategyLLM:
		if s == nil || s.llmProvider == nil {
			return nil, fmt.Errorf("llm 页面控件策略未启用，请配置 LLM provider")
		}
		return s.llmProvider.DetectPageControls(ctx)
	default:
		return nil, nil
	}
}

// OnStepResult 通过 Goja 插件通知步骤结果
func (s *Coordinator) OnStepResult(input StepResultInput) error {
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
	return s.runtime.OnStepResult(runtimeInput)
}

func isAppRestartAction(act types.ActionType) bool {
	return act == types.START || act == types.RESTART || act == types.CLEAN_RESTART
}

const (
	pageControlStrategyRaw     = "raw"
	pageControlStrategyOCR     = "ocr"
	pageControlStrategyLLM     = "llm"
	defaultPageControlCacheTTL = 30 * time.Minute
)

func normalizePageControlStrategy(strategy string) string {
	switch strings.ToLower(strings.TrimSpace(strategy)) {
	case "", pageControlStrategyRaw:
		return pageControlStrategyRaw
	case pageControlStrategyOCR:
		return pageControlStrategyOCR
	case pageControlStrategyLLM:
		return pageControlStrategyLLM
	default:
		return pageControlStrategyRaw
	}
}

func buildSyntheticXMLFromCandidates(strategy string, items []perception.Candidate, screenshot []byte) (string, types.IElement) {
	type syntheticNode struct {
		Text        string
		Class       string
		ContentDesc string
		Bounds      string
	}

	nodes := make([]syntheticNode, 0, len(items))
	for _, item := range items {
		if item.Command == nil {
			continue
		}
		if item.Command.Act != types.CLICK && item.Command.Act != types.LONG_CLICK && item.Command.Act != types.INPUT {
			continue
		}
		pos := item.Command.Pos
		if pos.IsEmpty() {
			continue
		}
		label := strings.TrimSpace(item.Metadata["ocr_text"])
		if label == "" {
			label = strings.TrimSpace(item.Metadata["llm_control_text"])
		}
		if label == "" {
			label = strings.TrimSpace(item.Metadata["llm_target_hint"])
		}
		if label == "" {
			label = strings.TrimSpace(item.Intent)
		}
		// 截断过长的标签，避免合成 XML 节点名过长
		if len([]rune(label)) > 15 {
			label = string([]rune(label)[:12]) + "..."
		}
		nodes = append(nodes, syntheticNode{
			Text:        label,
			Class:       resolveSyntheticWidgetClass(label, item.Command.Act),
			ContentDesc: label,
			Bounds:      fmt.Sprintf("[%.6f,%.6f][%.6f,%.6f]", pos.Left, pos.Top, pos.Right, pos.Bottom),
		})
	}
	if len(nodes) == 0 {
		return "", nil
	}

	rootExtraAttr := ""
	if strategy == pageControlStrategyLLM {
		rootExtraAttr = ` trek-scroll-infer-disabled="true"`
	}

	var b strings.Builder
	b.WriteString(`<hierarchy rotation="0">`)
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf(`  <node index="0" text="" resource-id="" class="android.widget.FrameLayout" content-desc="" clickable="false" enabled="true"%s bounds="[0,0][1,1]">`, rootExtraAttr))
	b.WriteString("\n")
	for i, node := range nodes {
		b.WriteString(fmt.Sprintf(`    <node index="%d" text="%s" resource-id="visual_%d" class="%s" content-desc="%s" clickable="true" long-clickable="false" enabled="true" bounds="%s"/>`,
			i+1,
			escapeXMLAttr(node.Text),
			i+1,
			node.Class,
			escapeXMLAttr(node.ContentDesc),
			node.Bounds,
		))
		b.WriteString("\n")
	}
	b.WriteString("  </node>\n</hierarchy>")
	syntheticXML := b.String()

	// 创建 IElement 对象
	elem, err := elements.CreateAndroidElementFromXml(syntheticXML)
	if err != nil {
		logger.Warnf("coordinator 创建合成 XML IElement 失败: %v", err)
		return syntheticXML, nil
	}
	return syntheticXML, elem
}

func pageControlCacheKey(strategy string, screenshot []byte) string {
	if strategy != pageControlStrategyOCR && strategy != pageControlStrategyLLM {
		return ""
	}
	fingerprint := pageControlCacheFingerprint(screenshot)
	if fingerprint == "" {
		return ""
	}
	return strategy + "|" + fingerprint
}

func pageControlCacheFingerprint(screenshot []byte) string {
	return strings.TrimSpace(visionfingerprint.Name(screenshot, nil))
}

func (s *Coordinator) resolvePageControlCacheTTL() time.Duration {
	if s == nil {
		return defaultPageControlCacheTTL
	}
	if s.config.PageControlCacheTTL > 0 {
		return s.config.PageControlCacheTTL
	}
	if text := strings.TrimSpace(os.Getenv("PAGE_CONTROL_CACHE_TTL_SECONDS")); text != "" {
		if seconds, err := strconv.Atoi(text); err == nil && seconds > 0 {
			return time.Duration(seconds) * time.Second
		}
	}
	return defaultPageControlCacheTTL
}

func (s *Coordinator) isPageControlCacheExpired(refreshedAt time.Time, hitCount int) bool {
	if refreshedAt.IsZero() {
		return false
	}
	baseTTL := s.resolvePageControlCacheTTL()
	if baseTTL <= 0 {
		return false
	}
	ttl := effectivePageControlCacheTTL(baseTTL, hitCount)
	return time.Since(refreshedAt) >= ttl
}

// effectivePageControlCacheTTL 基于记忆曲线计算动态 TTL。
// 访问越频繁的页面缓存时间越长：effectiveTTL = baseTTL × (1 + ln(hitCount))。
func effectivePageControlCacheTTL(baseTTL time.Duration, hitCount int) time.Duration {
	if hitCount <= 1 {
		return baseTTL
	}
	boost := 1.0 + math.Log(float64(hitCount))
	return time.Duration(float64(baseTTL) * boost)
}

func chooseCacheRefreshTime(values ...time.Time) time.Time {
	for _, value := range values {
		if !value.IsZero() {
			return value
		}
	}
	return time.Time{}
}

func decodeScreenshotSize(data []byte) (int, int) {
	if len(data) == 0 {
		return 0, 0
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return 0, 0
	}
	return cfg.Width, cfg.Height
}

func toPixelBounds(rect types.Rect, width int, height int) ([4]int, bool) {
	var result [4]int
	left := rect.Left
	top := rect.Top
	right := rect.Right
	bottom := rect.Bottom
	if right <= left || bottom <= top {
		return result, false
	}
	if right <= 1 && bottom <= 1 {
		left *= float64(width)
		right *= float64(width)
		top *= float64(height)
		bottom *= float64(height)
	}
	result = [4]int{
		int(left + 0.5),
		int(top + 0.5),
		int(right + 0.5),
		int(bottom + 0.5),
	}
	if result[2] <= result[0] || result[3] <= result[1] {
		return result, false
	}
	return result, true
}

func resolveSyntheticWidgetClass(label string, act types.ActionType) string {
	if act == types.INPUT {
		return "android.widget.EditText"
	}
	lower := strings.ToLower(strings.TrimSpace(label))
	switch {
	case strings.Contains(lower, "输入"), strings.Contains(lower, "搜索"), strings.Contains(lower, "search"), strings.Contains(lower, "input"):
		return "android.widget.EditText"
	default:
		return "android.widget.Button"
	}
}

func escapeXMLAttr(text string) string {
	if text == "" {
		return ""
	}
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(text))
	return strings.ReplaceAll(b.String(), `"`, "&quot;")
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
