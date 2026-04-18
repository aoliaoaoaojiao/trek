package engine

import (
	"fmt"
	"strings"
	"trek/internal/engine/core/types"
	"trek/internal/engine/run"
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
	run.ResetModel()
	run.InitAgent(s.config.Algorithm, s.config.PackageName, s.config.DeviceType)
}

// LoadPreferenceFile 加载偏好配置文件。
func (s *Session) LoadPreferenceFile(path string) error {
	model := run.GetModel()
	if model == nil {
		s.Reset()
		model = run.GetModel()
	}
	if model == nil || model.GetPreference() == nil {
		return fmt.Errorf("偏好配置实例不可用")
	}
	return model.GetPreference().LoadMixResMapping(path)
}

// NextActionJSON 根据页面名称和 Android XML 计算下一步操作 JSON。
func (s *Session) NextActionJSON(pageName string, xmlDescOfGuiTree string) (string, error) {
	if strings.TrimSpace(pageName) == "" {
		return "", fmt.Errorf("pageName 不能为空")
	}
	if strings.TrimSpace(xmlDescOfGuiTree) == "" {
		return "", fmt.Errorf("xmlDescOfGuiTree 不能为空")
	}

	actionJSON := run.GetAction(pageName, xmlDescOfGuiTree)
	if actionJSON == "" {
		return "", fmt.Errorf("未生成有效动作")
	}
	return actionJSON, nil
}

// NextAction 返回结构化的下一步操作。
func (s *Session) NextAction(pageName string, xmlDescOfGuiTree string) (*types.DeviceOperateWrapper, error) {
	actionJSON, err := s.NextActionJSON(pageName, xmlDescOfGuiTree)
	if err != nil {
		return nil, err
	}

	operate := types.NewDeviceOperateWrapperFromJSON(actionJSON)
	return operate, nil
}

// CheckPointInBlackRects 判断点位是否落在黑名单矩形内。
func (s *Session) CheckPointInBlackRects(pageName string, point types.Point) bool {
	return run.CheckPointIsInBlackRects(pageName, float32(point.X), float32(point.Y))
}

// NativeVersion 返回当前原生引擎版本。
func (s *Session) NativeVersion() string {
	return run.GetNativeVersion()
}
