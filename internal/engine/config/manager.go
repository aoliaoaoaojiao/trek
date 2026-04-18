package config

import (
	"errors"
	"fmt"
	"github.com/dop251/goja"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
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
		m.resolvePage(rootXML)
	}

	if len(m.currentActions) == 0 {
		for _, customEvent := range m.customEvents {
			eventRate := rand.Float64()
			if eventRate < customEvent.Prob && customEvent.Times > 0 && customEvent.PageName == pageName {
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

	return nil
}

func (m *Manager) resolvePage(rootXML types.IElement) {
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
	_ = action
	_ = rootXML
	return true
}

func (m *Manager) SkipAllActionsFromModel() bool {
	return m.skipAllActionsFromModel
}

func (m *Manager) PatchOperate(operate *types.ActionCommand) {
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
	m.blackRects = make(map[string][][4]int)

	if resourceMappingPath == "" {
		return nil
	}
	if strings.ToLower(filepath.Ext(resourceMappingPath)) != ".js" {
		return fmt.Errorf("配置文件仅支持 Goja 脚本格式(.js): %s", resourceMappingPath)
	}

	scriptBytes, err := os.ReadFile(resourceMappingPath)
	if err != nil {
		return err
	}

	vm := goja.New()
	if _, err = vm.RunString(string(scriptBytes)); err != nil {
		return fmt.Errorf("执行 goja 配置脚本失败: %w", err)
	}

	cfgValue := vm.Get("config")
	if isEmptyJSValue(cfgValue) {
		cfgValue = vm.Get("CONFIG")
	}
	if isEmptyJSValue(cfgValue) {
		return errors.New("配置脚本必须导出 config 或 CONFIG 对象")
	}

	cfgObj := cfgValue.ToObject(vm)

	if resMappingValue := cfgObj.Get("res_mapping"); !isEmptyJSValue(resMappingValue) {
		resMappingObj := resMappingValue.ToObject(vm)
		for _, key := range resMappingObj.Keys() {
			m.resMapping[key] = toStringValue(resMappingObj.Get(key))
		}
	}

	blackRectsValue := cfgObj.Get("black_rects")
	if isEmptyJSValue(blackRectsValue) {
		return nil
	}

	blackRectsObj := blackRectsValue.ToObject(vm)
	for _, pageName := range blackRectsObj.Keys() {
		rectsValue := blackRectsObj.Get(pageName)
		rectsArray := rectsValue.ToObject(vm)
		rectKeys := rectsArray.Keys()
		pageRects := make([][4]int, 0, len(rectKeys))
		for _, rectKey := range rectKeys {
			rectValue := rectsArray.Get(rectKey)
			rectArray := rectValue.ToObject(vm)
			rectItemKeys := rectArray.Keys()
			if len(rectItemKeys) != 4 {
				return fmt.Errorf("black_rects[%s][%s] 长度必须为4", pageName, rectKey)
			}
			var rect [4]int
			for i, itemKey := range rectItemKeys {
				intVal, convErr := toIntValue(rectArray.Get(itemKey))
				if convErr != nil {
					return fmt.Errorf("black_rects[%s][%s][%d] 非法: %w", pageName, rectKey, i, convErr)
				}
				rect[i] = intVal
			}
			pageRects = append(pageRects, rect)
		}
		m.blackRects[pageName] = pageRects
	}

	return nil
}

func toStringValue(v goja.Value) string {
	if isEmptyJSValue(v) {
		return ""
	}
	return v.String()
}

func toIntValue(v goja.Value) (int, error) {
	if isEmptyJSValue(v) {
		return 0, errors.New("值不能为空")
	}
	f := v.ToFloat()
	if f != float64(int(f)) {
		return 0, fmt.Errorf("值必须是整数: %v", f)
	}
	return int(f), nil
}

func isEmptyJSValue(v goja.Value) bool {
	return v == nil || goja.IsUndefined(v) || goja.IsNull(v)
}

// Deprecated: 请使用 LoadResourceMapping。
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
