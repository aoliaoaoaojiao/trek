package session

import (
	"fmt"
	"strings"
	"trek/internal/engine/core/types"
	engineruntime "trek/internal/engine/runtime"
	"trek/logger"
)

// Config 描述一次单线程会话初始化参数。
type Config struct {
	PackageName string
	Algorithm   types.AlgorithmType
	DeviceType  types.DeviceType
}

// ActionInput 描述下一步决策输入，支持 XML 与截图双通道。
type ActionInput struct {
	XMLDescOfGuiTree string
	Screenshot       []byte
}

// PageInfo 表示页面名称和对应 XML 信息。
type PageInfo struct {
	PageName string
	XML      string
}

// PageSnapshot 描述脚本插件可见的页面快照。
type PageSnapshot struct {
	PageName   string
	XML        string
	Screenshot []byte
}

// StepResultInput 描述一步动作执行后的复盘信息。
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

// Session 是对外稳定会话入口，屏蔽内部全局模型细节。
type Session struct {
	config Config
}

// NewSession 创建新会话并初始化默认配置。
func NewSession(config Config) *Session {
	if config.Algorithm == 0 {
		config.Algorithm = types.Reuse
	}
	if config.DeviceType == 0 {
		config.DeviceType = types.Phone
	}

	session := &Session{config: config}
	session.Reset()
	return session
}

// Reset 重置内部模型并重新初始化 agent。
func (s *Session) Reset() {
	engineruntime.ResetModel()
	engineruntime.InitAgent(s.config.Algorithm, s.config.PackageName, s.config.DeviceType)
}

// LoadConfigFile 加载运行时配置文件（主入口）。
func (s *Session) LoadConfigFile(path string) error {
	if engineruntime.GetModel() == nil {
		s.Reset()
	}
	return engineruntime.LoadConfigFile(path)
}

// Deprecated: 请使用 LoadConfigFile。
func (s *Session) LoadPreferenceFile(path string) error {
	return s.LoadConfigFile(path)
}

// NextActionJSON 返回 JSON 形式的下一步动作（兼容接口）。
func (s *Session) NextActionJSON(pageName string, xmlDescOfGuiTree string) (string, error) {
	operate, err := s.NextAction(pageName, xmlDescOfGuiTree)
	if err != nil {
		return "", err
	}
	return operate.ToJSON(), nil
}

// NextActionJSONWithInput 返回 JSON 形式的下一步动作。
func (s *Session) NextActionJSONWithInput(pageName string, input ActionInput) (string, error) {
	operate, err := s.NextActionWithInput(pageName, input)
	if err != nil {
		return "", err
	}
	return operate.ToJSON(), nil
}

// NextAction 返回结构化下一步动作（主路径）。
func (s *Session) NextAction(pageName string, xmlDescOfGuiTree string) (*types.ActionCommand, error) {
	return s.NextActionWithInput(pageName, ActionInput{XMLDescOfGuiTree: xmlDescOfGuiTree})
}

// NextActionWithInput 基于 XML/截图输入返回下一步动作。
func (s *Session) NextActionWithInput(pageName string, input ActionInput) (*types.ActionCommand, error) {
	if strings.TrimSpace(pageName) == "" {
		return nil, fmt.Errorf("pageName 不能为空")
	}
	if strings.TrimSpace(input.XMLDescOfGuiTree) == "" && len(input.Screenshot) == 0 {
		return nil, fmt.Errorf("xmlDescOfGuiTree 和 screenshot 不能同时为空")
	}

	operate := engineruntime.GetActionOptWithInput(pageName, input.XMLDescOfGuiTree, input.Screenshot)
	if operate == nil {
		return nil, fmt.Errorf("未生成有效动作")
	}

	logger.Infof("session next action: page=%s cmd={%s}", pageName, operate.DetailLogString())

	return operate, nil
}

// SetObservationMode 设置感知模式（xml-only / image-only / hybrid）。
func (s *Session) SetObservationMode(mode string) error {
	return engineruntime.SetObservationMode(mode)
}

// GetObservationMode 返回当前感知模式。
func (s *Session) GetObservationMode() string {
	return engineruntime.GetObservationMode()
}

// CheckPointInBlackRects 判断坐标点是否在黑名单区域内。
func (s *Session) CheckPointInBlackRects(pageName string, point types.Point) bool {
	return engineruntime.CheckPointIsInBlackRects(pageName, float32(point.X), float32(point.Y))
}

// NativeVersion 返回当前引擎原生版本。
func (s *Session) NativeVersion() string {
	return engineruntime.GetNativeVersion()
}

// TransformPageInfoWithInput 使用 Goja 配置脚本改造页面信息并返回新结果（支持截图输入）。
func (s *Session) TransformPageInfoWithInput(pageName string, input ActionInput) (PageInfo, error) {
	if strings.TrimSpace(pageName) == "" {
		return PageInfo{}, fmt.Errorf("pageName 不能为空")
	}
	if strings.TrimSpace(input.XMLDescOfGuiTree) == "" && len(input.Screenshot) == 0 {
		return PageInfo{}, fmt.Errorf("xmlDescOfGuiTree 和 screenshot 不能同时为空")
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

// OnStepResult 通知 Goja 插件一步执行结果。
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
