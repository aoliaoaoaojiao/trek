package config

import (
	"fmt"
	"math/rand"
	"path/filepath"
	"strings"
	"time"
	coretypes "trek/internal/engine/core/types"
	"trek/internal/scripting"
	"trek/logger"
)

// 编译期接口检查
var _ coretypes.ConfigProvider = (*Manager)(nil)
var _ coretypes.StaticConfigProvider = (*Manager)(nil)

type CustomAction struct {
	coretypes.StatefulAction
	Xpath              string
	ResourceID         string
	ContentDescription string
	Text               string
	Classname          string
	PageName           string
	Command            string
	Bounds             []float64
	AllowFuzzing       bool
	ClearText          bool
	Throttle           int
	WaitTime           int
	AdbInput           bool
}

type CustomEvent struct {
	Prob     float64
	Times    int
	PageName string
	Actions  []*CustomAction
}

type Manager struct {
	currentActions          []coretypes.IAction
	customEvents            []*CustomEvent
	randomInputText         bool
	doInputFuzzing          bool
	inputTexts              []string
	fuzzingTexts            []string
	pageTextsCache          []string
	skipAllActionsFromModel bool
	blackRects              []scripting.BlackRect
	staticConfig            scripting.StaticConfig
	screenWidth             int // 屏幕宽度（像素），用于归一化坐标转换
	screenHeight            int // 屏幕高度（像素），用于归一化坐标转换
}

var instance *Manager

func init() {
	rand.Seed(time.Now().UnixNano())
	instance = &Manager{
		currentActions:          make([]coretypes.IAction, 0),
		customEvents:            make([]*CustomEvent, 0),
		randomInputText:         false,
		doInputFuzzing:          true,
		inputTexts:              make([]string, 0),
		fuzzingTexts:            make([]string, 0),
		pageTextsCache:          make([]string, 0),
		skipAllActionsFromModel: false,
		blackRects:              nil,
		staticConfig:            scripting.StaticConfig{},
	}
}

func GetInstance() *Manager {
	return instance
}

func (m *Manager) ResolvePageAndGetSpecifiedAction(pageName string, rootXML coretypes.IElement) coretypes.IAction {
	if rootXML != nil {
		m.resolvePage(rootXML)
	}

	if len(m.currentActions) == 0 {
		for _, customEvent := range m.customEvents {
			eventRate := rand.Float64()
			if eventRate < customEvent.Prob && customEvent.Times > 0 && customEvent.PageName == pageName {
				m.currentActions = make([]coretypes.IAction, len(customEvent.Actions))
				for i, action := range customEvent.Actions {
					m.currentActions[i] = action
				}
				customEvent.Times--
				break
			}
		}
	}

	if len(m.currentActions) > 0 {
		frontAction := m.currentActions[0]
		m.currentActions = m.currentActions[1:]

		if customAction, ok := frontAction.(*CustomAction); ok {
			if rootXML != nil && !m.patchActionBounds(customAction, rootXML) {
				return nil
			}
			return customAction
		}
	}

	return nil
}

func (m *Manager) resolvePage(rootXML coretypes.IElement) {
	m.cachePageTexts(rootXML)
}

func (m *Manager) cachePageTexts(rootXML coretypes.IElement) {
	if rootXML == nil {
		return
	}
	text := rootXML.GetText()
	if text != "" {
		m.pageTextsCache = append(m.pageTextsCache, text)
	}
	for _, child := range rootXML.GetChildren() {
		m.cachePageTexts(child)
	}
}

func (m *Manager) patchActionBounds(action *CustomAction, rootXML coretypes.IElement) bool {
	_ = action
	_ = rootXML
	return true
}

func (m *Manager) SkipAllActionsFromModel() bool {
	return m.skipAllActionsFromModel
}

func (m *Manager) PatchOperate(operate *coretypes.ActionCommand) {
	if !m.doInputFuzzing {
		return
	}

	if operate.Editable && operate.Text == "" && (operate.Act == coretypes.CLICK || operate.Act == coretypes.LONG_CLICK) {
		if m.randomInputText && len(m.inputTexts) > 0 {
			randIdx := rand.Intn(len(m.inputTexts))
			operate.Text = m.inputTexts[randIdx]
		} else {
			rate := rand.Float64() * 100
			if len(m.fuzzingTexts) > 0 && rate < 50 {
				randIdx := rand.Intn(len(m.fuzzingTexts))
				operate.Text = m.fuzzingTexts[randIdx]
			} else if rate < 85 && len(m.pageTextsCache) > 0 {
				randIdx := rand.Intn(len(m.pageTextsCache))
				operate.Text = m.pageTextsCache[randIdx]
			}
		}
	}
}

func (m *Manager) LoadResourceMapping(resourceMappingPath string) error {
	m.blackRects = nil
	m.customEvents = make([]*CustomEvent, 0)
	m.currentActions = make([]coretypes.IAction, 0)
	m.skipAllActionsFromModel = false
	m.staticConfig = scripting.StaticConfig{}

	if resourceMappingPath == "" {
		return nil
	}
	if strings.ToLower(filepath.Ext(resourceMappingPath)) != ".js" {
		return fmt.Errorf("配置文件仅支持 Goja 脚本格式(.js): %s", resourceMappingPath)
	}

	staticConfig, err := scripting.LoadStaticConfigFile(resourceMappingPath)
	if err != nil {
		return err
	}
	m.staticConfig = staticConfig
	m.blackRects = staticConfig.BlackRects
	m.skipAllActionsFromModel = staticConfig.SkipAll
	if staticConfig.ScrollInferThreshold > 0 {
		coretypes.ScrollInferThreshold = staticConfig.ScrollInferThreshold
	}
	if staticConfig.Log.FileLevel != "" {
		if err := logger.SetFileLevel(staticConfig.Log.FileLevel); err != nil {
			return fmt.Errorf("设置文件日志级别失败: %w", err)
		}
	}
	return nil
}

func (m *Manager) GetStaticConfig() scripting.StaticConfig {
	if m == nil {
		return scripting.StaticConfig{}
	}
	return m.staticConfig
}

func (m *Manager) GetUCTBanditConfig() coretypes.UCTBanditStaticConfig {
	if m == nil {
		return coretypes.UCTBanditStaticConfig{}
	}
	uctCfg := m.staticConfig.UCTBandit
	return coretypes.UCTBanditStaticConfig{
		TwoStateLoopPenalty:    uctCfg.TwoStateLoopPenalty,
		EdgeRepeatPenalty:      uctCfg.EdgeRepeatPenalty,
		EdgeRepeatThreshold:    uctCfg.EdgeRepeatThreshold,
		ActionCooldownPenalty:  uctCfg.ActionCooldownPenalty,
		RecentActionWindow:     uctCfg.RecentActionWindow,
		LoopEscapeExploreBoost: uctCfg.LoopEscapeExploreBoost,
	}
}

func (m *Manager) GetReuseConfig() coretypes.ReuseStaticConfig {
	if m == nil {
		return coretypes.ReuseStaticConfig{}
	}
	reuseCfg := m.staticConfig.Reuse
	return coretypes.ReuseStaticConfig{
		Epsilon:                reuseCfg.Epsilon,
		Gamma:                  reuseCfg.Gamma,
		NStep:                  reuseCfg.NStep,
		ModelSavePath:          reuseCfg.ModelSavePath,
		EnableModelPersistence: reuseCfg.EnableModelPersistence,
		ResetModelOnStart:      reuseCfg.ResetModelOnStart,
	}
}

func (m *CustomAction) ToActionCommand() *coretypes.ActionCommand {
	operate := m.StatefulAction.ToOperate()
	operate.Text = m.Text
	operate.Clear = m.ClearText
	operate.AdbInput = m.AdbInput
	operate.AllowFuzzing = m.AllowFuzzing
	operate.WaitTime = m.WaitTime
	operate.Throttle = float32(m.Throttle)
	if len(m.Bounds) == 4 {
		operate.Pos = coretypes.Rect{
			Left:   m.Bounds[0],
			Top:    m.Bounds[1],
			Right:  m.Bounds[2],
			Bottom: m.Bounds[3],
		}
	}
	return operate
}

// Deprecated: use LoadResourceMapping.
func (m *Manager) LoadMixResMapping(resourceMappingPath string) error {
	return m.LoadResourceMapping(resourceMappingPath)
}

// SetScreenSize 设置屏幕分辨率，用于归一化坐标转换。
func (m *Manager) SetScreenSize(width, height int) {
	if m == nil {
		return
	}
	m.screenWidth = width
	m.screenHeight = height
}

// GetScreenSize 获取屏幕分辨率。
func (m *Manager) GetScreenSize() (int, int) {
	if m == nil {
		return 0, 0
	}
	return m.screenWidth, m.screenHeight
}

func (m *Manager) CheckPointIsInBlackRects(pageName string, pointX int, pointY int) bool {
	for _, br := range m.blackRects {
		// page_name 为空或未设置时，视为全局屏蔽区域，匹配所有页面
		if br.PageName != "" && br.PageName != pageName {
			continue
		}

		// 根据坐标系类型进行比较
		if br.Normalized {
			// 归一化坐标：将像素坐标转换为归一化坐标进行比较
			if m.screenWidth <= 0 || m.screenHeight <= 0 {
				// 未设置屏幕分辨率，跳过归一化坐标的检查
				continue
			}
			normX := float64(pointX) / float64(m.screenWidth)
			normY := float64(pointY) / float64(m.screenHeight)
			bf := br.BoundsFloat
			if normX >= bf[0] && normX <= bf[2] && normY >= bf[1] && normY <= bf[3] {
				return true
			}
		} else {
			// 像素坐标：直接比较
			b := br.Bounds
			if pointX >= b[0] && pointX <= b[2] && pointY >= b[1] && pointY <= b[3] {
				return true
			}
		}
	}
	return false
}
