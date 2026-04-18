package configruntime

import (
	"math/rand"
	"time"
	"trek/internal/engine/core/types"
)

type CustomAction struct {
	types.StatefulAction
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

// Manager 持有运行时配置执行态（偏好、规则命中缓存、输入 fuzz 等）。
type Manager struct {
	currentActions          []types.IAction
	customEvents            []*CustomEvent
	randomInputText         bool
	doInputFuzzing          bool
	inputTexts              []string
	fuzzingTexts            []string
	pageTextsCache          []string
	skipAllActionsFromModel bool
	resMapping              map[string]string
	blackRects              map[string][][4]int
}

var instance *Manager

func init() {
	rand.Seed(time.Now().UnixNano())
	instance = &Manager{
		currentActions:          make([]types.IAction, 0),
		customEvents:            make([]*CustomEvent, 0),
		randomInputText:         false,
		doInputFuzzing:          true,
		inputTexts:              make([]string, 0),
		fuzzingTexts:            make([]string, 0),
		pageTextsCache:          make([]string, 0),
		skipAllActionsFromModel: false,
		resMapping:              make(map[string]string),
		blackRects:              make(map[string][][4]int),
	}
}

func GetInstance() *Manager {
	return instance
}

func (m *Manager) ResolvePageAndGetSpecifiedAction(pageName string, rootXML types.IElement) types.IAction {
	if rootXML != nil {
		m.resolvePage(pageName, rootXML)
	}

	var returnAction types.IAction = nil
	if len(m.currentActions) == 0 {
		for _, customEvent := range m.customEvents {
			eventRate := rand.Float64()
			if eventRate < customEvent.Prob && customEvent.Times > 0 && customEvent.PageName == pageName {
				m.currentActions = nil
				m.currentActions = make([]types.IAction, len(customEvent.Actions))
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

	return returnAction
}

func (m *Manager) resolvePage(pageName string, rootXML types.IElement) {
	m.cachePageTexts(rootXML)
}

func (m *Manager) cachePageTexts(rootXML types.IElement) {
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

func (m *Manager) patchActionBounds(action *CustomAction, rootXML types.IElement) bool {
	return true
}

func (m *Manager) SkipAllActionsFromModel() bool {
	return m.skipAllActionsFromModel
}

func (m *Manager) PatchOperate(operate *types.DeviceOperateWrapper) {
	if !m.doInputFuzzing {
		return
	}

	if operate.Editable && operate.Text == "" && (operate.Act == types.CLICK || operate.Act == types.LONG_CLICK) {
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

// LoadResourceMapping 加载资源映射配置（主入口）。
func (m *Manager) LoadResourceMapping(resourceMappingPath string) error {
	m.resMapping = make(map[string]string)
	_ = resourceMappingPath
	return nil
}

// Deprecated: 请使用 LoadResourceMapping。
// LoadMixResMapping 兼容旧命名。
func (m *Manager) LoadMixResMapping(resourceMappingPath string) error {
	return m.LoadResourceMapping(resourceMappingPath)
}



func (m *Manager) CheckPointIsInBlackRects(pageName string, pointX int, pointY int) bool {
	if rects, ok := m.blackRects[pageName]; ok {
		for _, rect := range rects {
			if pointX >= rect[0] && pointX <= rect[2] && pointY >= rect[1] && pointY <= rect[3] {
				return true
			}
		}
	}
	return false
}
