package plugin

import (
	"fmt"
	types2 "trek/internal/engine/decision/shared/types"

	"trek/internal/scripting"
)

type PluginContext = scripting.PluginContext
type PageSnapshot = scripting.PageSnapshot
type RuntimeContext = scripting.RuntimeContext
type BlockRecoveryContext = scripting.BlockRecoveryContext
type Screenshot = scripting.Screenshot
type PageNode = scripting.PageNode
type StepResult = scripting.StepResult
type StepResultContext = scripting.StepResultContext

type Adapter struct {
	manager *scripting.Manager
}

func LoadFile(path string) (*Adapter, error) {
	manager, err := scripting.LoadFile(path)
	if err != nil {
		return nil, err
	}
	return NewAdapterFromManager(manager), nil
}

func NewAdapterFromManager(manager *scripting.Manager) *Adapter {
	return &Adapter{manager: manager}
}

func (a *Adapter) TransformPage(ctx PluginContext) (PageSnapshot, error) {
	if a == nil || a.manager == nil {
		return ctx.Page, nil
	}
	return a.manager.TransformPage(ctx)
}

func (a *Adapter) BeforeDecide(ctx PluginContext) (*types2.ActionCommand, bool, error) {
	if a == nil || a.manager == nil {
		return nil, false, nil
	}
	action, handled, err := a.manager.BeforeDecide(ctx)
	if err != nil || !handled || action == nil {
		return nil, handled, err
	}
	cmd, err := ToActionCommand(*action)
	return cmd, true, err
}

func (a *Adapter) AfterDecide(ctx PluginContext, cmd *types2.ActionCommand) (*types2.ActionCommand, bool, error) {
	if a == nil || a.manager == nil || cmd == nil {
		return cmd, false, nil
	}
	action := FromActionCommand(cmd)
	next, handled, err := a.manager.AfterDecide(ctx, &action)
	if err != nil || !handled {
		return cmd, handled, err
	}
	if next == nil {
		return nil, true, nil
	}
	result, err := ToActionCommand(*next)
	return result, true, err
}

func (a *Adapter) OnStepResult(ctx StepResultContext) error {
	if a == nil || a.manager == nil {
		return nil
	}
	return a.manager.OnStepResult(ctx)
}

func ToActionCommand(action scripting.Action) (*types2.ActionCommand, error) {
	cmd := types2.NewActionCommand()
	actionType, ok := toEngineActionType(action.Type)
	if !ok {
		return nil, fmt.Errorf("涓嶆敮鎸佺殑鑴氭湰鍔ㄤ綔: %s", action.Type)
	}
	cmd.Act = actionType
	cmd.Pos = types2.Rect{
		Left:   action.Bounds[0],
		Top:    action.Bounds[1],
		Right:  action.Bounds[2],
		Bottom: action.Bounds[3],
	}
	cmd.Text = action.Text
	cmd.Clear = action.Clear
	cmd.AdbInput = action.ADBInput
	cmd.AllowFuzzing = action.AllowFuzzing
	cmd.Throttle = float32(action.Throttle)
	cmd.WaitTime = action.WaitTime
	return cmd, nil
}

func FromActionCommand(cmd *types2.ActionCommand) scripting.Action {
	if cmd == nil {
		return scripting.Action{Type: scripting.ActionNOP}
	}
	return scripting.Action{
		Type:         fromEngineActionType(cmd.Act),
		Bounds:       [4]float64{cmd.Pos.Left, cmd.Pos.Top, cmd.Pos.Right, cmd.Pos.Bottom},
		Text:         cmd.Text,
		Clear:        cmd.Clear,
		ADBInput:     cmd.AdbInput,
		AllowFuzzing: cmd.AllowFuzzing,
		Throttle:     int(cmd.Throttle),
		WaitTime:     cmd.WaitTime,
	}
}

func toEngineActionType(actionType scripting.ActionType) (types2.ActionType, bool) {
	switch actionType {
	case scripting.ActionNOP:
		return types2.NOP, true
	case scripting.ActionBack:
		return types2.BACK, true
	case scripting.ActionClick:
		return types2.CLICK, true
	case scripting.ActionLongClick:
		return types2.LONG_CLICK, true
	case scripting.ActionScrollTopDown:
		return types2.SCROLL_TOP_DOWN, true
	case scripting.ActionScrollBottomUp:
		return types2.SCROLL_BOTTOM_UP, true
	case scripting.ActionScrollLeftRight:
		return types2.SCROLL_LEFT_RIGHT, true
	case scripting.ActionScrollRightLeft:
		return types2.SCROLL_RIGHT_LEFT, true
	case scripting.ActionScrollBottomUpN:
		return types2.SCROLL_BOTTOM_UP_N, true
	case scripting.ActionStart:
		return types2.START, true
	case scripting.ActionRestart:
		return types2.RESTART, true
	case scripting.ActionCleanRestart:
		return types2.CLEAN_RESTART, true
	case scripting.ActionActivate:
		return types2.ACTIVATE, true
	default:
		return types2.NOP, false
	}
}

func fromEngineActionType(actionType types2.ActionType) scripting.ActionType {
	switch actionType {
	case types2.BACK:
		return scripting.ActionBack
	case types2.CLICK:
		return scripting.ActionClick
	case types2.LONG_CLICK:
		return scripting.ActionLongClick
	case types2.SCROLL_TOP_DOWN:
		return scripting.ActionScrollTopDown
	case types2.SCROLL_BOTTOM_UP:
		return scripting.ActionScrollBottomUp
	case types2.SCROLL_LEFT_RIGHT:
		return scripting.ActionScrollLeftRight
	case types2.SCROLL_RIGHT_LEFT:
		return scripting.ActionScrollRightLeft
	case types2.SCROLL_BOTTOM_UP_N:
		return scripting.ActionScrollBottomUpN
	case types2.START:
		return scripting.ActionStart
	case types2.RESTART:
		return scripting.ActionRestart
	case types2.CLEAN_RESTART:
		return scripting.ActionCleanRestart
	case types2.ACTIVATE:
		return scripting.ActionActivate
	default:
		return scripting.ActionNOP
	}
}
