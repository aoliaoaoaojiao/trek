package preference

import (
	"Trek/internal/fastbot/core/types"
	"math/rand"
	"time"
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

type Preference struct {
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

var instance *Preference

func init() {
	rand.Seed(time.Now().UnixNano())
	instance = &Preference{
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

func GetInstance() *Preference {
	return instance
}

func (p *Preference) ResolvePageAndGetSpecifiedAction(pageName string, rootXML types.IElement) types.IAction {
	if rootXML != nil {
		p.resolvePage(pageName, rootXML)
	}

	var returnAction types.IAction = nil
	if len(p.currentActions) == 0 {
		for _, customEvent := range p.customEvents {
			eventRate := rand.Float64()
			if eventRate < customEvent.Prob && customEvent.Times > 0 && customEvent.PageName == pageName {
				p.currentActions = nil
				p.currentActions = make([]types.IAction, len(customEvent.Actions))
				for i, action := range customEvent.Actions {
					p.currentActions[i] = action
				}
				customEvent.Times--
				break
			}
		}
	}

	if len(p.currentActions) > 0 {
		frontAction := p.currentActions[0]
		p.currentActions = p.currentActions[1:]

		if customAction, ok := frontAction.(*CustomAction); ok {
			if rootXML != nil && !p.patchActionBounds(customAction, rootXML) {
				return nil
			}
			return customAction
		}
	}

	return returnAction
}

func (p *Preference) resolvePage(pageName string, rootXML types.IElement) {
	p.cachePageTexts(rootXML)
}

func (p *Preference) cachePageTexts(rootXML types.IElement) {
	if rootXML == nil {
		return
	}
	text := rootXML.GetText()
	if text != "" {
		p.pageTextsCache = append(p.pageTextsCache, text)
	}
	for _, child := range rootXML.GetChildren() {
		p.cachePageTexts(child)
	}
}

func (p *Preference) patchActionBounds(action *CustomAction, rootXML types.IElement) bool {
	return true
}

func (p *Preference) SkipAllActionsFromModel() bool {
	return p.skipAllActionsFromModel
}

func (p *Preference) PatchOperate(operate *types.DeviceOperateWrapper) {
	if !p.doInputFuzzing {
		return
	}

	if operate.Editable && operate.Text == "" && (operate.Act == types.CLICK || operate.Act == types.LONG_CLICK) {
		if p.randomInputText && len(p.inputTexts) > 0 {
			randIdx := rand.Intn(len(p.inputTexts))
			operate.Text = p.inputTexts[randIdx]
		} else {
			rate := rand.Float64() * 100
			if len(p.fuzzingTexts) > 0 && rate < 50 {
				randIdx := rand.Intn(len(p.fuzzingTexts))
				operate.Text = p.fuzzingTexts[randIdx]
			} else if rate < 85 && len(p.pageTextsCache) > 0 {
				randIdx := rand.Intn(len(p.pageTextsCache))
				operate.Text = p.pageTextsCache[randIdx]
			}
		}
	}
}

func (p *Preference) LoadMixResMapping(resourceMappingPath string) {
	p.resMapping = make(map[string]string)
}

func (p *Preference) CheckPointIsInBlackRects(pageName string, pointX int, pointY int) bool {
	if rects, ok := p.blackRects[pageName]; ok {
		for _, rect := range rects {
			if pointX >= rect[0] && pointX <= rect[2] && pointY >= rect[1] && pointY <= rect[3] {
				return true
			}
		}
	}
	return false
}
