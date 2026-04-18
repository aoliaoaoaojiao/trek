package engine

import (
	"fmt"
	"strings"
	"trek/internal/engine/core/types"
	engineruntime "trek/internal/engine/runtime"
)

// Config 描述一次单线程引擎会话的初始化参数。
type Config struct {
	PackageName string
	Algorithm   types.AlgorithmType
	DeviceType  types.DeviceType
}

// Session 为调用方提供稳定的单线程入口，屏蔽内部全局模型细节。
type Session struct {
	config Config
}

// NewSession 创建新的单线程引擎会话。
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

// Reset 重置内部模型并重新初始化 agent，适合一次新任务开始前调用。
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
// LoadPreferenceFile 兼容旧命名。
func (s *Session) LoadPreferenceFile(path string) error {
	return s.LoadConfigFile(path)
}

// NextActionJSON 根据页面名称和 Android XML 计算下一步操作 JSON（兼容接口）。
func (s *Session) NextActionJSON(pageName string, xmlDescOfGuiTree string) (string, error) {
	operate, err := s.NextAction(pageName, xmlDescOfGuiTree)
	if err != nil {
		return "", err
	}
	return operate.ToJSON(), nil
}

// NextAction 返回结构化的下一步操作（主路径，不经过 JSON 回转）。
func (s *Session) NextAction(pageName string, xmlDescOfGuiTree string) (*types.DeviceOperateWrapper, error) {
	if strings.TrimSpace(pageName) == "" {
		return nil, fmt.Errorf("pageName 不能为空")
	}
	if strings.TrimSpace(xmlDescOfGuiTree) == "" {
		return nil, fmt.Errorf("xmlDescOfGuiTree 不能为空")
	}

	operate := engineruntime.GetActionOpt(pageName, xmlDescOfGuiTree)
	if operate == nil {
		return nil, fmt.Errorf("未生成有效动作")
	}
	return operate, nil
}

// CheckPointInBlackRects 判断点位是否落在黑名单矩形内。
func (s *Session) CheckPointInBlackRects(pageName string, point types.Point) bool {
	return engineruntime.CheckPointIsInBlackRects(pageName, float32(point.X), float32(point.Y))
}

// NativeVersion 返回当前原生引擎版本。
func (s *Session) NativeVersion() string {
	return engineruntime.GetNativeVersion()
}
