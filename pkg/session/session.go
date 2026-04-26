package session

import (
	"fmt"
	"os"
	"strings"
	"time"
	"trek/internal/engine/candidate"
	candidateproviders "trek/internal/engine/candidate/providers"
	"trek/internal/engine/decision"
	"trek/internal/engine/decision/shared/types"
	"trek/internal/engine/memory"
	engineruntime "trek/internal/engine/runtime"
	enginestate "trek/internal/engine/state"
	"trek/logger"
)

// Config 闂佺顕х换妤呭醇椤忓懐鈻旈柍褜鍓欓埢搴ㄥ灳閸愯尙顦扮紓浣瑰礃濞夋洟鍩㈤崗鐓庮嚤婵ê纾Ο鍫ユ煕閹烘挾绠撴い顐ｅ姍瀹曠娀寮借濡﹢鏌℃担钘夌劷闁?
type Config struct {
	PackageName              string
	Algorithm                decision.AlgorithmType
	DeviceType               types.DeviceType
	RecoveryMemoryFile       string
	RecoveryLLMEndpoint      string
	RecoveryLLMAPIKey        string
	RecoveryLLMModel         string
	RecoveryLLMOpenAIModel   string
	RecoveryLLMOpenAIAPIKey  string
	RecoveryLLMOpenAIBaseURL string
	RecoveryLLMTimeout       time.Duration
}

// ActionInput 闂佺顕х换妤呭醇椤忓懐鈻旈悗锝傛櫇椤忚鲸鎱ㄥ┑鍕姎闁哥姴鎳愮划鐢稿冀椤掑倻鐐曢梺绋跨箞閸庤崵妲愬┑瀣哗妞ゆ牗绋戦惁?XML 婵炴垶鎸哥€涒晠宕洪崨鏉戠倞闁诡垎鍐憰闂備緡鍋呭畝鍛婄妤ｅ啫违?
type ActionInput struct {
	XMLDescOfGuiTree string
	Screenshot       []byte
}

// PageInfo 闁荤偞绋忛崝搴ㄥΦ濮橆厹浜滈柣銏犳啞濡椼劑鏌涘顒傂ょ悮銊╂煕濠婂啳瀚版い鏇ㄥ枟閹?XML 婵烇絽娲犻崜婵囧閸涙潙违?
type PageInfo struct {
	PageName string
	XML      string
}

// PageSnapshot 闂佺顕х换妤呭醇椤忓牊鍤囨慨姗嗗幗閹烽亶鏌熺紒妯虹瑐婵炲棎鍨藉畷锝夘敍濠垫劖缍勯梺姹囧妼鐎氫即濡存径鎰棃闁靛繒濮佃ぐ銉╂煟閹炬ぞ璁查崑?
type PageSnapshot struct {
	PageName   string
	XML        string
	Screenshot []byte
}

// StepResultInput 闂佺顕х换妤呭醇椤忓懐鈻旈柍褜鍓欓～銏ゅΨ閵夈儛妤€霉閿濆棛鐭嬪褏鏅幃鎵沪閼恒儱鈧敻鏌ｉ妸銉ヮ仼妞わ附鐓￠幆鍕熼柅娑氱畾闂佽鍙庨崹顒勫焵?
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

// Session 闂佸搫瀚烽崹浼搭敋椤旇棄绶為柡宥庡€撻弮鍌楀亾鐟欏嫮鐓紒杈ㄥ閹风姴鈹戦崱妤€姹查梺鍛婄懕缂嶅洨妲愬┑鍫滄勃闊洦纰嶉弶鎼佹煕閹邦剚鍤囬柛搴㈡尦瀹曟濡搁妷褎鎷卞┑鈽嗗灙閳ь剙纾埀顒勵棑缁辨帡宕卞顑╂繈鏌?
type Session struct {
	config         Config
	memoryStore    *memory.Store
	memoryProvider *memory.Provider
	llmProvider    recoveryLLMCandidateProvider
}

type recoveryLLMCandidateProvider interface {
	BuildCandidates(ctx enginestate.TraversalContext) ([]candidate.Candidate, error)
}

// NewSession 闂佸憡甯楃粙鎴犵磽閹捐妫樺Λ棰佽兌缁愭鎮归崶銊х畵闁艰崵鍠栧畷姘攽閸♀晜缍忛梺鍛婄墬閻楃姷鍒掗婊勫闁靛牆顦敮宕囩磽閸愭儳鏋旈柍?
func NewSession(config Config) *Session {
	if config.Algorithm == 0 {
		config.Algorithm = decision.AlgorithmReuse
	}
	if config.DeviceType == 0 {
		config.DeviceType = types.Phone
	}

	session := &Session{config: config}
	session.initRecoveryMemoryProvider()
	session.initRecoveryLLMProvider()
	session.Reset()
	return session
}

func (s *Session) initRecoveryMemoryProvider() {
	path := strings.TrimSpace(s.config.RecoveryMemoryFile)
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

func (s *Session) initRecoveryLLMProvider() {
	endpoint := strings.TrimSpace(s.config.RecoveryLLMEndpoint)
	apiKey := strings.TrimSpace(s.config.RecoveryLLMAPIKey)
	if apiKey == "" {
		apiKey = strings.TrimSpace(os.Getenv("TREK_RECOVERY_LLM_API_KEY"))
	}

	// 显式 endpoint 优先，便于兼容任意网关。
	if endpoint != "" {
		provider, err := candidateproviders.NewLLMHTTPProvider(candidateproviders.LLMHTTPProviderConfig{
			Endpoint: endpoint,
			APIKey:   apiKey,
			Model:    strings.TrimSpace(s.config.RecoveryLLMModel),
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
		return
	}
	openAIKey := strings.TrimSpace(s.config.RecoveryLLMOpenAIAPIKey)
	if openAIKey == "" {
		openAIKey = strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	}
	provider, err := candidateproviders.NewOpenAIResponsesProvider(candidateproviders.OpenAIResponsesProviderConfig{
		BaseURL: strings.TrimSpace(s.config.RecoveryLLMOpenAIBaseURL),
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

// Reset 闂備焦褰冪粔鍫曟偪閸℃稑绀冮柛娑欐綑閸斻儲淇婇妞诲亾瀹曞洠鍋撻悜鐣屽祦闁割偅娲栧▍銏ゆ煛閸屾稒绶查柛銊ョ仛閹便劎鈧綆浜滈?agent闂?
func (s *Session) Reset() {
	engineruntime.ResetModel()
	engineruntime.InitAgent(s.config.Algorithm, s.config.PackageName, s.config.DeviceType)
}

// LoadConfigFile 闂佸憡姊绘慨鎯归崶銊︿氦闁归偊鍨奸弨浠嬫煛閸愩劎鍩ｉ柛妯稿€楃槐鏃堫敊閼恒儳鈧喖霉閻樹警鍟囩紒杈ㄧ懄缁嬪鎷犻幓鎺戞辈闂佸憡鐟辩紞鍥╂濮樿泛违?
func (s *Session) LoadConfigFile(path string) error {
	if engineruntime.GetModel() == nil {
		s.Reset()
	}
	return engineruntime.LoadConfigFile(path)
}

// Deprecated: 闁荤姴娲╁〒瑙勭箾閸ヮ剚鍋?LoadConfigFile闂?
func (s *Session) LoadPreferenceFile(path string) error {
	return s.LoadConfigFile(path)
}

// NextActionJSON 闁哄鏅滈弻銊ッ?JSON 閻熸粏鍩囬崹鍦閿熺姵鍎嶉柛鏇ㄤ簽閻熸挸鈽夐幘顖氫壕濠殿喗绺块崕杈ㄦ叏閳哄倹濯存繝濠冨姉缁€鍕煕韫囨洖甯舵い鏃€鍔欓獮鎺楀Ψ閵夈儳绋夐梺鎸庣☉椤︻參鍩€?
func (s *Session) NextActionJSON(pageName string, xmlDescOfGuiTree string) (string, error) {
	operate, err := s.NextAction(pageName, xmlDescOfGuiTree)
	if err != nil {
		return "", err
	}
	return operate.ToJSON(), nil
}

// NextActionJSONWithInput 闁哄鏅滈弻銊ッ?JSON 閻熸粏鍩囬崹鍦閿熺姵鍎嶉柛鏇ㄤ簽閻熸挸鈽夐幘顖氫壕濠殿喗绺块崕杈ㄦ叏閳哄倹濯存繝褍绨遍崑?
func (s *Session) NextActionJSONWithInput(pageName string, input ActionInput) (string, error) {
	operate, err := s.NextActionWithInput(pageName, input)
	if err != nil {
		return "", err
	}
	return operate.ToJSON(), nil
}

// NextAction 闁哄鏅滈弻銊ッ洪弽顐ょ＜闁规儳顕埀顒夊灦瀹曠娀寮介敂鍓ф啴婵炴垶鎸撮崑鎾存叏濠靛嫬鍔氬┑顔规櫆閹峰懎顭ㄩ幇顔绢槱婵炴垶鎸诲Σ鎺楁儗閹屽殫闁告洖澧庣粈鍡涙煏?
func (s *Session) NextAction(pageName string, xmlDescOfGuiTree string) (*types.ActionCommand, error) {
	return s.NextActionWithInput(pageName, ActionInput{XMLDescOfGuiTree: xmlDescOfGuiTree})
}

// NextActionWithInput 闂佺硶鏅炲銊ц姳?XML/闂佽鎯屾禍婊兠瑰Ο缁樼秶闁规儳鍟垮鎶藉级閳哄倹鐓ユ繛鍙夌墬缁嬪鈧絺鏅濋杈ㄦ叏濠靛嫬鍔氬┑顔规櫆閹峰懎鈹冩惔鎾充壕?
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

// BuildMemoryRecoveryCandidates 将 memory 经验转为恢复候选，供 runner 的 RecoveryPlanner 调用。
func (s *Session) BuildMemoryRecoveryCandidates(ctx enginestate.TraversalContext) ([]candidate.Candidate, error) {
	if s == nil || s.memoryProvider == nil {
		return nil, nil
	}
	return s.memoryProvider.BuildCandidates(ctx)
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

// BuildLLMRecoveryCandidates 调用外部 LLM 服务构建恢复候选。
func (s *Session) BuildLLMRecoveryCandidates(ctx enginestate.TraversalContext) ([]candidate.Candidate, error) {
	if s == nil || s.llmProvider == nil {
		return nil, nil
	}
	return s.llmProvider.BuildCandidates(ctx)
}

// RecordRecoveryMemoryOutcome 写回一次恢复动作结果，用于跨会话复用。
func (s *Session) RecordRecoveryMemoryOutcome(ctx enginestate.TraversalContext, item candidate.Candidate, escaped bool) error {
	if s == nil || s.memoryStore == nil || item.Command == nil {
		return nil
	}
	traceSignature := buildTraceSignature(ctx.RecentTrace)
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
		MemoryKey:        memory.BuildMemoryKey(ctx.PageSignature, ctx.BlockReason, traceSignature, ctx.Mode),
		PageSignature:    ctx.PageSignature,
		ClusterSignature: ctx.ClusterSignature,
		BlockReason:      ctx.BlockReason,
		TraceSignature:   traceSignature,
		Mode:             ctx.Mode,
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

// GetObservationMode 闁哄鏅滈弻銊ッ洪弽顑句汗闁规儳鍟块·鍛存煙閹殿喖鏋旈柣鎾偓婢勭喖鍨惧畷鍥╊攨闂?
func (s *Session) GetObservationMode() string {
	return engineruntime.GetObservationMode()
}

// CheckPointInBlackRects 闂佸憡甯囬崐鏍蓟閸ヮ剙閿ら柟閭﹀幘閸ㄥジ鏌ｉ幇顖ｆ綈婵″弶鎮傚畷銉╂晝閳ь剙锕㈤鍜佹付闁瑰瓨绻冮崐鎶芥煕濡や焦绀€閻忓浚鍨跺畷娲偄瀹勭増鏆ラ梺?
func (s *Session) CheckPointInBlackRects(pageName string, point types.Point) bool {
	return engineruntime.CheckPointIsInBlackRects(pageName, float32(point.X), float32(point.Y))
}

// NativeVersion 闁哄鏅滈弻銊ッ洪弽顑句汗闁规儳鍟块·鍛偓娈垮枟濞叉﹢骞栭幖浣稿偍闁绘梻鍎ら弲鎼佹煟濡も偓閻楀﹤锕㈡导鏉懳?
func (s *Session) NativeVersion() string {
	return engineruntime.GetNativeVersion()
}

// TransformPageInfoWithInput 婵炶揪缍€濞夋洟寮?Goja 闂備焦婢樼粔鍫曟偪閸℃稒鍤囨慨姗嗗幗閹烽亶鏌￠埀顒冦亹閹哄棗浜鹃柣妯荤湽閳ь剙顦靛Λ鍐綖椤撴繄绠氶梺璇″弾閸ㄤ即鎳熼悢鍛婁氦闁哄倹瀵х粈鈧梺鍝勫€规竟鍡欏垝閵娾晛鍑犳繝濠冨姉缁€鍕煛閳ь剟顢涘☉妯兼Х闂佽鎯屾禍婊兠瑰Ο缁樼秶闁规儳鍟垮鎶芥煥濞戞﹩妲堕柍?
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

// OnStepResult 闂備緡鍋呭銊╂偂?Goja 闂佸湱绮敮濠傗枎閵忥紕鈻旈柍褜鍓欓～銏ゅΨ閿曗偓閳锋棃鎮跺☉妯肩伇缂侇喓鍔戝绋库攦鎼存挸浜?
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

func buildTraceSignature(trace []enginestate.ActionTrace) string {
	if len(trace) == 0 {
		return ""
	}
	parts := make([]string, 0, len(trace))
	for _, item := range trace {
		key := strings.TrimSpace(item.ActionKey)
		if key == "" {
			continue
		}
		parts = append(parts, key)
	}
	return strings.Join(parts, ">")
}
