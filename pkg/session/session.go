package session

import (
	"fmt"
	"strings"
	"trek/internal/engine/core/types"
	engineruntime "trek/internal/engine/runtime"
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
	model := engineruntime.GetModel()
	if model == nil {
		s.Reset()
		model = engineruntime.GetModel()
	}
	if model == nil || model.GetConfigManager() == nil {
		return fmt.Errorf("配置实例不可用")
	}
	return model.GetConfigManager().LoadResourceMapping(path)
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
