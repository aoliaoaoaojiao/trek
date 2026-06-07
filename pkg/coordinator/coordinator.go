package coordinator

import (
	"bytes"
	"context"
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
	"trek/internal/engine/perception/providers/llm"
	"trek/internal/engine/perception/deeplocate"
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

	// Insight 模型配置（用于页面控件检测等轻量任务）
	InsightLLMEndpoint      string
	InsightLLMAPIKey        string
	InsightLLMModel         string
	InsightLLMOpenAIModel   string
	InsightLLMOpenAIAPIKey  string
	InsightLLMOpenAIBaseURL string
	PageControlStrategy      string
	PageControlCacheFile     string
	PageControlCacheTTL      time.Duration
	PlanCacheTTL             time.Duration // LLM 响应缓存 TTL，默认 5 分钟
	OnPlanCacheHit           func()        // LLM 响应缓存命中回调
	OnPlanCacheMiss          func()        // LLM 响应缓存未命中回调
	ModelFamily              string
	DeepLocateEnabled        bool
	DeepLocateSectionExpandPx int
	DeepLocateSectionMinSize  int
	DeepLocateZoomFactor      int
	VLMAnnotationEnabled     bool
	VLMAnnotationFontScale   int
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
	config               Config
	runtime              *engineruntime.Runtime
	memoryStore          *memory.Store
	memoryProvider       *memory.Provider
	pageControlStore     *pagecache.Store
	ocrProvider          candidateProvider
	llmProvider          pageControlCandidateProvider
	traversalAlgo        traversal.TraversalAlgorithm
	pageControlCache     sync.Map
	pageControlMemory    sync.Map // 页面控件记忆: pageName → syntheticXML（上次成功识别的结果）
	lastPageInfo         *pageInfoCacheEntry // 上次返回的完整 PageInfo，截图指纹相同时直接复用
	consumedFingerprints sync.Map // 消费标记：恢复周期内跳过已消费的缓存条目
	planCache            sync.Map // LLM 响应缓存: key="pageSignature|blockReason" → planCacheEntry
	locateCache          sync.Map // 元素定位缓存: key="intent|pageSignature" → locateCacheEntry
}

type pageControlCacheEntry struct {
	SyntheticXML string
	RefreshedAt  time.Time
	HitCount     int
}

// pageInfoCacheEntry 缓存完整的 PageInfo + 截图指纹，页面未变时零开销复用。
type pageInfoCacheEntry struct {
	Info        PageInfo
	Fingerprint string
}

type planCacheEntry struct {
	Candidates []perception.Candidate
	CreatedAt  time.Time
	HitCount   int
}

type locateCacheEntry struct {
	Candidate perception.Candidate
	CreatedAt time.Time
	HitCount  int
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
	// 优先使用 Insight 模型配置（轻量级任务），否则回退到 Recovery 模型配置
	endpoint := strings.TrimSpace(s.config.InsightLLMEndpoint)
	if endpoint == "" {
		endpoint = strings.TrimSpace(s.config.RecoveryLLMEndpoint)
	}
	if endpoint == "" {
		endpoint = strings.TrimSpace(os.Getenv("LLM_API_URL"))
	}
	apiKey := strings.TrimSpace(s.config.InsightLLMAPIKey)
	if apiKey == "" {
		apiKey = strings.TrimSpace(s.config.RecoveryLLMAPIKey)
	}
	if apiKey == "" {
		apiKey = strings.TrimSpace(os.Getenv("LLM_API_KEY"))
	}
	model := strings.TrimSpace(s.config.InsightLLMModel)
	if model == "" {
		model = strings.TrimSpace(s.config.RecoveryLLMModel)
	}
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
			ModelFamily:       s.config.ModelFamily,
			AnnotationEnabled: s.config.VLMAnnotationEnabled,
			AnnotationFontScale: s.config.VLMAnnotationFontScale,
		})
		if err != nil {
			logger.Warnf("coordinator 初始化 page control llm provider 失败: endpoint=%s err=%v", endpoint, err)
			return
		}
		s.llmProvider = provider
		logger.Infof("coordinator page control llm provider 已启用: endpoint=%s model=%s", endpoint, model)
		return
	}

	openAIModel := strings.TrimSpace(s.config.InsightLLMOpenAIModel)
	if openAIModel == "" {
		openAIModel = strings.TrimSpace(s.config.RecoveryLLMOpenAIModel)
	}
	if openAIModel == "" {
		openAIModel = strings.TrimSpace(os.Getenv("OPENAI_MODEL"))
	}
	if openAIModel == "" {
		return
	}
	openAIKey := strings.TrimSpace(s.config.InsightLLMOpenAIAPIKey)
	if openAIKey == "" {
		openAIKey = strings.TrimSpace(s.config.RecoveryLLMOpenAIAPIKey)
	}
	if openAIKey == "" {
		openAIKey = strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	}
	openAIBaseURL := strings.TrimSpace(s.config.InsightLLMOpenAIBaseURL)
	if openAIBaseURL == "" {
		openAIBaseURL = strings.TrimSpace(s.config.RecoveryLLMOpenAIBaseURL)
	}
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

	// 启动后台清理：定期删除过期缓存记录，最大有效期 3 天
	store.StartCleanup(context.Background(), pagecache.CleanupOptions{
		BaseTTL:      s.resolvePageControlCacheTTL(),
		MaxTTL:       maxPageControlCacheTTL,
		CleanupEvery: 10 * time.Minute,
		CleanupFn: func(count int) {
			logger.Infof("page cache 后台清理完成: 删除 %d 条过期记录", count)
		},
	})

	statsTotal, statsExpired, statsErr := store.Stats()
	if statsErr == nil {
		logger.Infof("coordinator 页面控件持久化缓存已启用: path=%s total=%d expired=%d", path, statsTotal, statsExpired)
	} else {
		logger.Infof("coordinator 页面控件持久化缓存已启用: path=%s", path)
	}
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

// BuildLLMRecoveryCandidates 通过 LLM provider 构建恢复候选。
// 用于重复阻塞场景下直接调用 LLM 规划，跳过常规恢复流程。
// 内置 LLM 响应缓存：相同 pageSignature+blockReason 的结果缓存 5 分钟。
func (s *Coordinator) BuildLLMRecoveryCandidates(ctx enginestate.TraversalContext) ([]perception.Candidate, error) {
	if s == nil || s.llmProvider == nil {
		return nil, nil
	}

	// 1. 尝试加载缓存
	if cached, ok := s.LoadPlanCache(ctx.PageSignature, ctx.BlockReason); ok {
		logger.Debugf("coordinator LLM 响应缓存命中: page=%s block=%s count=%d", ctx.PageName, ctx.BlockReason, len(cached))
		if s.config.OnPlanCacheHit != nil {
			s.config.OnPlanCacheHit()
		}
		return cached, nil
	}

	// 2. 缓存未命中，调用 LLM
	if provider, ok := s.llmProvider.(interface {
		BuildCandidates(ctx enginestate.TraversalContext) ([]perception.Candidate, error)
	}); ok {
		candidates, err := provider.BuildCandidates(ctx)
		if err != nil {
			return nil, err
		}
		// 3. 存储到缓存
		if len(candidates) > 0 {
			s.StorePlanCache(ctx.PageSignature, ctx.BlockReason, candidates)
		}
		if s.config.OnPlanCacheMiss != nil {
			s.config.OnPlanCacheMiss()
		}
		return candidates, nil
	}
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

	// chain 策略：raw → 缓存 → OCR+LLM 并行
	if strategy == pageControlStrategyChain {
		return s.resolveChainPageControlInfo(info, resolvedPageName, resolvedXML, input.Screenshot, forceRefresh)
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

	// memory fallback：同页面上次识别结果复用（跳过 LLM）
	if resolvedPageName != "" {
		if v, ok := s.pageControlMemory.Load(resolvedPageName); ok {
			if prevXML, ok := v.(string); ok && strings.TrimSpace(prevXML) != "" {
				if elem, err := elements.CreateAndroidElementFromXml(prevXML); err == nil {
					logger.Debugf("coordinator 页面控件 memory 命中: page=%s", resolvedPageName)
					info.XML = prevXML
					info.Element = elem
					info.CacheHit = true
					s.storePageControlCache(strategy, input.Screenshot, prevXML)
					return info, nil
				}
			}
		}
	}

	textOnly := resolvedXML != ""
	candidates, err := s.buildPageControlCandidates(strategy, enginestate.TraversalContext{
		Mode:       "Explore",
		PageName:   resolvedPageName,
		XML:        resolvedXML,
		Screenshot: input.Screenshot,
		TextOnly:   textOnly,
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
	// 存入 memory 供下次复用
	if resolvedPageName != "" {
		s.pageControlMemory.Store(resolvedPageName, syntheticXML)
	}
	s.storePageControlCache(strategy, input.Screenshot, syntheticXML)
	info.XML = syntheticXML
	info.Element = elem
	return info, nil
}

func (s *Coordinator) loadPageControlCache(strategy string, screenshot []byte, forceRefresh bool) (string, bool) {
	// 计算一次指纹，同时用于缓存键和消费标记检查
	fingerprint := pageControlCacheFingerprint(screenshot)
	if strategy != pageControlStrategyOCR && strategy != pageControlStrategyLLM {
		return "", false
	}
	if fingerprint == "" {
		return "", false
	}
	cacheKey := strategy + "|" + fingerprint
	if forceRefresh {
		logger.Debugf("coordinator 页面控件缓存主动刷新: reason=block_recovery strategy=%s", strategy)
		return "", false
	}
	// 消费标记检查：恢复周期内跳过已消费的缓存条目，强制重新识别
	if s.isCacheConsumedFingerprint(fingerprint) {
		logger.Debugf("coordinator 页面控件缓存已消费，跳过: strategy=%s", strategy)
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
	if strategy != pageControlStrategyOCR && strategy != pageControlStrategyLLM {
		return
	}
	if strings.TrimSpace(syntheticXML) == "" {
		return
	}
	// 计算一次指纹，同时用于缓存键和持久化存储
	fingerprint := pageControlCacheFingerprint(screenshot)
	if fingerprint == "" {
		return
	}
	cacheKey := strategy + "|" + fingerprint
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
			Fingerprint:  fingerprint,
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

// MarkCacheConsumed 将指定截图对应的缓存指纹标记为已消费。
// 消费标记有 5 秒有效期，超时后自动失效，让记忆曲线 TTL 接管。
func (s *Coordinator) MarkCacheConsumed(screenshot []byte) {
	if s == nil || len(screenshot) == 0 {
		return
	}
	fingerprint := pageControlCacheFingerprint(screenshot)
	if fingerprint == "" {
		return
	}
	s.consumedFingerprints.Store(fingerprint, time.Now())
}

// IsCacheConsumed 检查指定截图对应的缓存指纹是否已被消费。
func (s *Coordinator) IsCacheConsumed(screenshot []byte) bool {
	if s == nil || len(screenshot) == 0 {
		return false
	}
	fingerprint := pageControlCacheFingerprint(screenshot)
	return s.isCacheConsumedFingerprint(fingerprint)
}

// isCacheConsumedFingerprint 检查指定指纹是否已被消费（5 秒内有效，超时自动失效）。
func (s *Coordinator) isCacheConsumedFingerprint(fingerprint string) bool {
	if s == nil || fingerprint == "" {
		return false
	}
	v, loaded := s.consumedFingerprints.Load(fingerprint)
	if !loaded {
		return false
	}
	ts, ok := v.(time.Time)
	if !ok {
		return false
	}
	if time.Since(ts) > 5*time.Second {
		s.consumedFingerprints.Delete(fingerprint)
		return false
	}
	return true
}

// ResetConsumedMarks 清除所有消费标记，通常在页面变化时调用。
func (s *Coordinator) ResetConsumedMarks() {
	if s == nil {
		return
	}
	s.consumedFingerprints.Range(func(key, value any) bool {
		s.consumedFingerprints.Delete(key)
		return true
	})
}

// LoadPlanCache 加载 LLM 响应缓存。
func (s *Coordinator) LoadPlanCache(pageSignature, blockReason string) ([]perception.Candidate, bool) {
	if s == nil || pageSignature == "" {
		return nil, false
	}
	key := pageSignature + "|" + blockReason
	cached, ok := s.planCache.Load(key)
	if !ok {
		return nil, false
	}
	entry, ok := cached.(planCacheEntry)
	if !ok {
		return nil, false
	}
	// TTL 检查：使用配置值，默认 5 分钟
	ttl := s.resolvePlanCacheTTL()
	if time.Since(entry.CreatedAt) > ttl {
		s.planCache.Delete(key)
		return nil, false
	}
	entry.HitCount++
	s.planCache.Store(key, entry)
	return entry.Candidates, true
}

// resolvePlanCacheTTL 返回 LLM 响应缓存的 TTL。
func (s *Coordinator) resolvePlanCacheTTL() time.Duration {
	if s != nil && s.config.PlanCacheTTL > 0 {
		return s.config.PlanCacheTTL
	}
	return 5 * time.Minute
}

// StorePlanCache 存储 LLM 响应到缓存。
func (s *Coordinator) StorePlanCache(pageSignature, blockReason string, candidates []perception.Candidate) {
	if s == nil || pageSignature == "" || len(candidates) == 0 {
		return
	}
	key := pageSignature + "|" + blockReason
	s.planCache.Store(key, planCacheEntry{
		Candidates: candidates,
		CreatedAt:  time.Now(),
		HitCount:   1,
	})
}

// InvalidatePlanCache 清除指定页面的 LLM 响应缓存。
func (s *Coordinator) InvalidatePlanCache(pageSignature string) {
	if s == nil || pageSignature == "" {
		return
	}
	prefix := pageSignature + "|"
	s.planCache.Range(func(key, value any) bool {
		if k, ok := key.(string); ok && strings.HasPrefix(k, prefix) {
			s.planCache.Delete(key)
		}
		return true
	})
}

// LoadLocateCache 从元素定位缓存加载候选。
// key = "intent|pageSignature"
func (s *Coordinator) LoadLocateCache(intent, pageSignature string) (*perception.Candidate, bool) {
	if s == nil || intent == "" || pageSignature == "" {
		return nil, false
	}
	key := intent + "|" + pageSignature
	cached, ok := s.locateCache.Load(key)
	if !ok {
		return nil, false
	}
	entry, ok := cached.(locateCacheEntry)
	if !ok {
		return nil, false
	}
	// TTL 检查：使用 Plan Cache 的 TTL 配置
	ttl := s.resolvePlanCacheTTL()
	if time.Since(entry.CreatedAt) > ttl {
		s.locateCache.Delete(key)
		return nil, false
	}
	entry.HitCount++
	s.locateCache.Store(key, entry)
	return &entry.Candidate, true
}

// StoreLocateCache 存储元素定位结果到缓存。
func (s *Coordinator) StoreLocateCache(intent, pageSignature string, candidate perception.Candidate) {
	if s == nil || intent == "" || pageSignature == "" {
		return
	}
	key := intent + "|" + pageSignature
	s.locateCache.Store(key, locateCacheEntry{
		Candidate: candidate,
		CreatedAt: time.Now(),
		HitCount:  1,
	})
}

// InvalidateLocateCache 清除指定页面的元素定位缓存。
func (s *Coordinator) InvalidateLocateCache(pageSignature string) {
	if s == nil || pageSignature == "" {
		return
	}
	prefix := "|" + pageSignature
	s.locateCache.Range(func(key, value any) bool {
		if k, ok := key.(string); ok && strings.HasSuffix(k, prefix) {
			s.locateCache.Delete(key)
		}
		return true
	})
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
		candidates, err := s.llmProvider.DetectPageControls(ctx)
		if err != nil {
			return nil, err
		}
		// DeepLocate: 当启用且有候选和有截图时，做两阶段精定位
		if s.config.DeepLocateEnabled && len(candidates) > 0 && len(ctx.Screenshot) > 0 {
			logger.Debugf("[deeplocate] stage1 候选=%d, 最高置信度=%.3f", len(candidates), candidates[0].Confidence)
			refined := s.deepLocatePageControls(ctx, candidates)
			if refined != nil {
				logger.Debugf("[deeplocate] 精定位合并后候选=%d (原始=%d)", len(refined), len(candidates))
				candidates = refined
			} else {
				logger.Debug("[deeplocate] 精定位未返回结果，使用原始候选")
			}
		}
		// 使用 ModelAdapter 做坐标适配
		if s.config.ModelFamily != "" {
			adapter := llm.NewModelAdapter(s.config.ModelFamily)
			for i := range candidates {
				if candidates[i].Command != nil {
					left, top, right, bottom := adapter.AdaptBboxToRect(
						[4]float64{candidates[i].Command.Pos.Left, candidates[i].Command.Pos.Top, candidates[i].Command.Pos.Right, candidates[i].Command.Pos.Bottom},
						0, 0,
					)
					candidates[i].Command.Pos = types.Rect{Left: left, Top: top, Right: right, Bottom: bottom}
				}
			}
		}
		return candidates, nil
	default:
		return nil, nil
	}
}
// deepLocatePageControls 对 LLM 检测结果做 DeepLocate 两阶段精定位。
// 取最高置信度候选作为区域，裁剪放大后重新检测，重投影坐标。
func (s *Coordinator) deepLocatePageControls(ctx enginestate.TraversalContext, candidates []perception.Candidate) []perception.Candidate {
	if len(candidates) == 0 || len(ctx.Screenshot) == 0 {
		return nil
	}

	// 解码截图获取尺寸
	img, _, err := image.Decode(bytes.NewReader(ctx.Screenshot))
	if err != nil {
		return nil
	}
	shotW := img.Bounds().Dx()
	shotH := img.Bounds().Dy()
	if shotW <= 0 || shotH <= 0 {
		return nil
	}

	// 找最高置信度候选作为 section
	bestIdx := 0
	bestConf := candidates[0].Confidence
	for i := 1; i < len(candidates); i++ {
		if candidates[i].Confidence > bestConf {
			bestConf = candidates[i].Confidence
			bestIdx = i
		}
	}
	best := candidates[bestIdx]
	if best.Command == nil {
		return nil
	}

	// 构建 DeepLocate section 配置
	dlCfg := deeplocate.DeepLocateConfig{
		Enabled:         true,
		SectionExpandPx: s.config.DeepLocateSectionExpandPx,
		SectionMinSize:  s.config.DeepLocateSectionMinSize,
		ZoomFactor:      s.config.DeepLocateZoomFactor,
	}
	if dlCfg.SectionExpandPx <= 0 {
		dlCfg.SectionExpandPx = 100
	}
	if dlCfg.SectionMinSize <= 0 {
		dlCfg.SectionMinSize = 400
	}
	if dlCfg.ZoomFactor <= 0 {
		dlCfg.ZoomFactor = 2
	}

	// 候选坐标已在 [0,1] 空间
	vlmResp := deeplocate.VLMResponse{
		Left:   best.Command.Pos.Left,
		Top:    best.Command.Pos.Top,
		Right:  best.Command.Pos.Right,
		Bottom: best.Command.Pos.Bottom,
	}

	// Stage 1: 裁剪 + 放大
	section, err := deeplocate.DoSection(ctx.Screenshot, shotW, shotH, vlmResp, dlCfg)
	if err != nil || section == nil {
		return nil
	}

	// 构建新的 context，替换为放大后的截图
	zoomCtx := ctx
	zoomCtx.Screenshot = section.ZoomImage

	// Stage 2: 在放大区域上重新检测
	refined, err := s.llmProvider.DetectPageControls(zoomCtx)
	if err != nil || len(refined) == 0 {
		return nil
	}

	// 重投影放大区域坐标回全屏空间
	for i := range refined {
		if refined[i].Command == nil {
			continue
		}
		elemResp := deeplocate.VLMResponse{
			Left:   refined[i].Command.Pos.Left,
			Top:    refined[i].Command.Pos.Top,
			Right:  refined[i].Command.Pos.Right,
			Bottom: refined[i].Command.Pos.Bottom,
		}
		result, doErr := deeplocate.DoElement(elemResp, section, shotW, shotH, dlCfg)
		if doErr == nil && result != nil && result.ElementRect != nil {
			refined[i].Command.Pos = types.Rect{
				Left:   result.ElementRect.Left,
				Top:    result.ElementRect.Top,
				Right:  result.ElementRect.Right,
				Bottom: result.ElementRect.Bottom,
			}
		}
	}

	// 如果精定位结果有效，替换原始候选
	// 将非 best 的原始候选与精定位结果合并
	var merged []perception.Candidate
	merged = append(merged, candidates[:bestIdx]...)
	merged = append(merged, candidates[bestIdx+1:]...)
	for i := range refined {
		refined[i].Confidence = refined[i].Confidence * 1.1 // 精定位结果提升置信度
	}
	merged = append(merged, refined...)
	return merged
}

// resolveChainPageControlInfo chain 策略：raw → 缓存 → OCR+LLM 并行兜底。
func (s *Coordinator) resolveChainPageControlInfo(info PageInfo, pageName, xml string, screenshot []byte, forceRefresh bool) (PageInfo, error) {
	// 1. 有 raw XML → 直接用（raw 永不降级）
	//    有 XML 说明来自 uia/poco/mixed，结构化节点树足够使用
	if strings.TrimSpace(xml) != "" {
		logger.Debugf("coordinator chain 策略使用 raw XML: page=%s", pageName)
		return info, nil
	}

	// 2. 没有 raw XML（纯截图模式），需要截图才能继续
	if len(screenshot) == 0 {
		return info, nil
	}

	// 3. 页面指纹缓存命中 → 直接用缓存
	if cachedXML, ok := s.loadPageControlCache(pageControlStrategyChain, screenshot, forceRefresh); ok {
		logger.Debugf("coordinator chain 策略缓存命中: page=%s", pageName)
		info.XML = cachedXML
		info.CacheHit = true
		if elem, err := elements.CreateAndroidElementFromXml(cachedXML); err == nil {
			info.Element = elem
		}
		return info, nil
	}

	// 4. 缓存未命中 → OCR + LLM 并行调用
	logger.Debugf("coordinator chain 策略缓存未命中，启动 OCR+LLM 并行: page=%s", pageName)
	candidates, err := s.buildChainPageControlCandidates(enginestate.TraversalContext{
		Mode:       "Explore",
		PageName:   pageName,
		XML:        xml,
		Screenshot: screenshot,
	})
	if err != nil {
		return PageInfo{}, err
	}

	syntheticXML, elem := buildSyntheticXMLFromCandidates(pageControlStrategyChain, candidates, screenshot)
	if strings.TrimSpace(syntheticXML) == "" {
		return info, nil
	}
	s.storePageControlCache(pageControlStrategyChain, screenshot, syntheticXML)
	info.XML = syntheticXML
	info.Element = elem
	return info, nil
}

// buildChainPageControlCandidates 并行调用 OCR 和 LLM，合并 candidates。
func (s *Coordinator) buildChainPageControlCandidates(ctx enginestate.TraversalContext) ([]perception.Candidate, error) {
	var (
		ocrCandidates []perception.Candidate
		llmCandidates []perception.Candidate
		ocrErr        error
		llmErr        error
		wg            sync.WaitGroup
	)

	hasOCR := s.ocrProvider != nil
	hasLLM := s.llmProvider != nil

	if !hasOCR && !hasLLM {
		return nil, fmt.Errorf("chain 策略需要至少配置 OCR 或 LLM provider")
	}

	if hasOCR {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ocrCandidates, ocrErr = s.ocrProvider.BuildCandidates(ctx)
		}()
	}
	if hasLLM {
		wg.Add(1)
		go func() {
			defer wg.Done()
			llmCandidates, llmErr = s.llmProvider.DetectPageControls(ctx)
		}()
	}

	wg.Wait()

	// 如果两者都失败，返回第一个错误
	if ocrErr != nil && llmErr != nil {
		return nil, fmt.Errorf("OCR 失败: %v; LLM 失败: %v", ocrErr, llmErr)
	}
	// 如果一方失败，用另一方的结果
	if ocrErr != nil {
		logger.Warnf("coordinator chain 策略 OCR 失败，仅用 LLM 结果: %v", ocrErr)
		return llmCandidates, nil
	}
	if llmErr != nil {
		logger.Warnf("coordinator chain 策略 LLM 失败，仅用 OCR 结果: %v", llmErr)
		return ocrCandidates, nil
	}

	return mergeCandidates(ocrCandidates, llmCandidates), nil
}

// mergeCandidates 合并 OCR 和 LLM 的 candidates，按位置近似去重，LLM 优先。
func mergeCandidates(ocrCandidates, llmCandidates []perception.Candidate) []perception.Candidate {
	if len(ocrCandidates) == 0 {
		return llmCandidates
	}
	if len(llmCandidates) == 0 {
		return ocrCandidates
	}

	// 先放入所有 LLM candidates
	merged := make([]perception.Candidate, 0, len(ocrCandidates)+len(llmCandidates))
	merged = append(merged, llmCandidates...)

	// 对每个 OCR candidate，检查是否与已有 candidate 位置重复
	for _, ocr := range ocrCandidates {
		if ocr.Command == nil || ocr.Command.Pos.IsEmpty() {
			continue
		}
		ocrCenter := ocr.Command.Pos.Center()
		isDuplicate := false
		for _, existing := range merged {
			if existing.Command == nil || existing.Command.Pos.IsEmpty() {
				continue
			}
			existingCenter := existing.Command.Pos.Center()
			// 中心距离 < 50px 视为同一控件
			dx := ocrCenter.X - existingCenter.X
			dy := ocrCenter.Y - existingCenter.Y
			if dx*dx+dy*dy < 50*50 {
				isDuplicate = true
				break
			}
		}
		if !isDuplicate {
			merged = append(merged, ocr)
		}
	}

	return merged
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
	pageControlStrategyChain   = "chain"
	defaultPageControlCacheTTL = 1 * time.Hour
	maxPageControlCacheTTL     = 72 * time.Hour // 缓存最大有效期上限（3天）
)

func normalizePageControlStrategy(strategy string) string {
	switch strings.ToLower(strings.TrimSpace(strategy)) {
	case "", pageControlStrategyRaw:
		return pageControlStrategyRaw
	case pageControlStrategyOCR:
		return pageControlStrategyOCR
	case pageControlStrategyLLM:
		return pageControlStrategyLLM
	case pageControlStrategyChain:
		return pageControlStrategyChain
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
		Editable    bool
		Scrollable  bool
		ScrollType  string
	}

	nodes := make([]syntheticNode, 0, len(items))
	for _, item := range items {
		if item.Command == nil {
			continue
		}
		pos := item.Command.Pos
		if pos.IsEmpty() {
			continue
		}
		act := item.Command.Act
		// 仅保留可交互动作：CLICK、LONG_CLICK、INPUT、SCROLL_*
		if act != types.CLICK && act != types.LONG_CLICK && act != types.INPUT &&
			!isScrollAction(act) {
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
		scrollType := scrollTypeForAction(act)
		nodes = append(nodes, syntheticNode{
			Text:        label,
			Class:       resolveSyntheticWidgetClass(label, act),
			ContentDesc: label,
			Bounds:      fmt.Sprintf("[%.6f,%.6f][%.6f,%.6f]", pos.Left, pos.Top, pos.Right, pos.Bottom),
			Editable:    act == types.INPUT,
			Scrollable:  scrollType != "",
			ScrollType:  scrollType,
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
		if node.Scrollable {
			b.WriteString(fmt.Sprintf(`    <node index="%d" text="%s" resource-id="visual_%d" class="%s" content-desc="%s" clickable="false" scrollable="true" scrollType="%s" enabled="true" bounds="%s"/>`,
				i+1,
				escapeXMLAttr(node.Text),
				i+1,
				node.Class,
				escapeXMLAttr(node.ContentDesc),
				node.ScrollType,
				node.Bounds,
			))
		} else {
			editableAttr := ` editable="false"`
			if node.Editable {
				editableAttr = ` editable="true"`
			}
			b.WriteString(fmt.Sprintf(`    <node index="%d" text="%s" resource-id="visual_%d" class="%s" content-desc="%s" clickable="true" long-clickable="false" enabled="true"%s bounds="%s"/>`,
				i+1,
				escapeXMLAttr(node.Text),
				i+1,
				node.Class,
				escapeXMLAttr(node.ContentDesc),
				editableAttr,
				node.Bounds,
			))
		}
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
// 最大不超过 maxPageControlCacheTTL（3天）。
func effectivePageControlCacheTTL(baseTTL time.Duration, hitCount int) time.Duration {
	if hitCount <= 1 {
		return baseTTL
	}
	boost := 1.0 + math.Log(float64(hitCount))
	ttl := time.Duration(float64(baseTTL) * boost)
	if ttl > maxPageControlCacheTTL {
		ttl = maxPageControlCacheTTL
	}
	return ttl
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
	if isScrollAction(act) {
		return "android.widget.ScrollView"
	}
	lower := strings.ToLower(strings.TrimSpace(label))
	switch {
	case strings.Contains(lower, "输入"), strings.Contains(lower, "搜索"), strings.Contains(lower, "search"), strings.Contains(lower, "input"):
		return "android.widget.EditText"
	default:
		return "android.widget.Button"
	}
}

// isScrollAction 判断动作类型是否为滚动类。
func isScrollAction(act types.ActionType) bool {
	switch act {
	case types.SCROLL_BOTTOM_UP, types.SCROLL_TOP_DOWN,
		types.SCROLL_LEFT_RIGHT, types.SCROLL_RIGHT_LEFT,
		types.SCROLL_BOTTOM_UP_N:
		return true
	default:
		return false
	}
}

// scrollTypeForAction 将滚动动作类型映射为 scrollType 属性值。
// 非滚动动作返回空字符串。
func scrollTypeForAction(act types.ActionType) string {
	switch act {
	case types.SCROLL_BOTTOM_UP, types.SCROLL_TOP_DOWN,
		types.SCROLL_BOTTOM_UP_N:
		return "Vertical"
	case types.SCROLL_LEFT_RIGHT, types.SCROLL_RIGHT_LEFT:
		return "Horizontal"
	default:
		return ""
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
