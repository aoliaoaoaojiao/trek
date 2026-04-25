package scripting

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/dop251/goja"
	"trek/logger"
)

var ErrPluginNotFound = errors.New("脚本未暴露 plugin 对象")

type Manager struct {
	source string
	state  map[string]any
}

func LoadFile(path string) (*Manager, error) {
	if strings.ToLower(filepath.Ext(path)) != ".js" {
		return nil, fmt.Errorf("脚本插件仅支持 .js: %s", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return LoadScript(string(data))
}

func LoadScript(source string) (*Manager, error) {
	m := &Manager{
		source: source,
		state:  make(map[string]any),
	}
	vm, err := m.newRuntime()
	if err != nil {
		return nil, err
	}
	if isEmptyJSValue(vm.Get("plugin")) {
		return nil, ErrPluginNotFound
	}
	return m, nil
}

func (m *Manager) TransformPage(ctx PluginContext) (PageSnapshot, error) {
	value, ok, err := m.callHook("transformPage", ctxToMap(ctx))
	if err != nil || !ok || isEmptyJSValue(value) {
		return ctx.Page, err
	}
	obj := value.ToObject(nil)
	page := ctx.Page
	if v := obj.Get("page_name"); !isEmptyJSValue(v) && strings.TrimSpace(v.String()) != "" {
		page.Name = v.String()
	}
	if v := obj.Get("xml"); !isEmptyJSValue(v) && strings.TrimSpace(v.String()) != "" {
		page.XML = v.String()
	}
	return page, nil
}

func (m *Manager) BeforeDecide(ctx PluginContext) (*Action, bool, error) {
	value, ok, err := m.callHook("beforeDecide", ctxToMap(ctx))
	if err != nil || !ok || isEmptyJSValue(value) {
		return nil, false, err
	}
	action, err := actionFromValue(value)
	if err != nil {
		return nil, false, err
	}
	return action, true, nil
}

func (m *Manager) AfterDecide(ctx PluginContext, action *Action) (*Action, bool, error) {
	if action == nil {
		return nil, false, nil
	}
	value, ok, err := m.callHook("afterDecide", ctxToMap(ctx), actionToMap(*action))
	if err != nil || !ok {
		return nil, false, err
	}
	if isEmptyJSValue(value) {
		return nil, true, nil
	}
	next, err := actionFromValue(value)
	if err != nil {
		return nil, false, err
	}
	return next, true, nil
}

func (m *Manager) OnStepResult(ctx StepResultContext) error {
	_, _, err := m.callHook("onStepResult", stepResultCtxToMap(ctx))
	return err
}

func (m *Manager) StateGet(key string) any {
	return m.state[key]
}

func (m *Manager) callHook(name string, args ...any) (goja.Value, bool, error) {
	vm, err := m.newRuntime()
	if err != nil {
		return nil, false, err
	}
	pluginValue := vm.Get("plugin")
	if isEmptyJSValue(pluginValue) {
		return nil, false, fmt.Errorf("脚本必须暴露 plugin 对象")
	}
	pluginObj := pluginValue.ToObject(vm)
	hookValue := pluginObj.Get(name)
	if isEmptyJSValue(hookValue) {
		return nil, false, nil
	}
	fn, ok := goja.AssertFunction(hookValue)
	if !ok {
		return nil, false, fmt.Errorf("plugin.%s 必须是函数", name)
	}
	values := make([]goja.Value, 0, len(args))
	for _, arg := range args {
		values = append(values, vm.ToValue(arg))
	}
	value, err := fn(pluginValue, values...)
	if err != nil {
		return nil, true, err
	}
	return value, true, nil
}

func (m *Manager) newRuntime() (*goja.Runtime, error) {
	vm := goja.New()
	if err := m.installTrekAPI(vm); err != nil {
		return nil, err
	}
	if _, err := vm.RunString(m.source); err != nil {
		return nil, fmt.Errorf("执行 goja 插件失败: %w", err)
	}
	return vm, nil
}

func (m *Manager) installTrekAPI(vm *goja.Runtime) error {
	actionAPI := map[string]any{
		"click": func(bounds []float64) map[string]any {
			return actionObject(ActionClick, bounds)
		},
		"longClick": func(bounds []float64) map[string]any {
			return actionObject(ActionLongClick, bounds)
		},
		"input": func(bounds []float64, text string, options map[string]any) map[string]any {
			action := actionObject(ActionClick, bounds)
			action["text"] = text
			action["clear"] = boolOption(options, "clear")
			action["adb_input"] = boolOption(options, "adb_input")
			return action
		},
		"back": func() map[string]any {
			return actionObject(ActionBack, nil)
		},
		"scroll": func(direction string, bounds []float64) map[string]any {
			return actionObject(scrollAction(direction), bounds)
		},
		"start": func() map[string]any {
			return actionObject(ActionStart, nil)
		},
		"restart": func(options map[string]any) map[string]any {
			if boolOption(options, "clean") {
				return actionObject(ActionCleanRestart, nil)
			}
			return actionObject(ActionRestart, nil)
		},
		"activate": func() map[string]any {
			return actionObject(ActionActivate, nil)
		},
		"nop": func() map[string]any {
			return actionObject(ActionNOP, nil)
		},
	}

	pageAPI := map[string]any{
		"findByText": func(page map[string]any, text string) any {
			return findNode(page, func(node map[string]any) bool { return node["text"] == text })
		},
		"findByResourceId": func(page map[string]any, id string) any {
			return findNode(page, func(node map[string]any) bool { return node["resource_id"] == id })
		},
		"findByContentDesc": func(page map[string]any, desc string) any {
			return findNode(page, func(node map[string]any) bool { return node["content_desc"] == desc })
		},
		"findByClass": func(page map[string]any, className string) any {
			return findNode(page, func(node map[string]any) bool { return node["class_name"] == className })
		},
		"findAll": func(call goja.FunctionCall) goja.Value {
			page, _ := call.Argument(0).Export().(map[string]any)
			predicate, ok := goja.AssertFunction(call.Argument(1))
			if !ok {
				return vm.ToValue([]any{})
			}
			nodes := nodesFromPageMap(page)
			results := make([]any, 0)
			for _, node := range nodes {
				keep, err := predicate(goja.Undefined(), vm.ToValue(node))
				if err == nil && keep.ToBoolean() {
					results = append(results, node)
				}
			}
			return vm.ToValue(results)
		},
		"removeByText": func(xml string, text string) string {
			return strings.ReplaceAll(xml, text, "")
		},
		"removeByResourceId": func(xml string, id string) string {
			return strings.ReplaceAll(xml, id, "")
		},
		"patchText": func(xml string, from goja.Value, to string) string {
			return patchString(xml, from, to)
		},
		"patchResourceId": func(xml string, from goja.Value, to string) string {
			return patchString(xml, from, to)
		},
		"hasScreenshot": func(page map[string]any) bool {
			_, ok := page["screenshot"].(map[string]any)
			return ok
		},
		"screenshotBytes": func(page map[string]any) any {
			if shot, ok := page["screenshot"].(map[string]any); ok {
				return shot["bytes"]
			}
			return nil
		},
		"screenshotSize": func(page map[string]any) int {
			if shot, ok := page["screenshot"].(map[string]any); ok {
				if size, ok := shot["size"].(int); ok {
					return size
				}
			}
			return 0
		},
	}

	stateAPI := map[string]any{
		"get": func(key string) any {
			return m.state[key]
		},
		"set": func(key string, value any) {
			m.state[key] = value
		},
		"inc": func(key string, delta ...int64) int64 {
			inc := int64(1)
			if len(delta) > 0 {
				inc = delta[0]
			}
			current, _ := m.state[key].(int64)
			current += inc
			m.state[key] = current
			return current
		},
		"delete": func(key string) {
			delete(m.state, key)
		},
		"clear": func() {
			m.state = make(map[string]any)
		},
	}

	logAPI := map[string]any{
		"debug": func(message string) { logger.Debugf("[goja] %s", message) },
		"info":  func(message string) { logger.Infof("[goja] %s", message) },
		"warn":  func(message string) { logger.Warnf("[goja] %s", message) },
		"error": func(message string) { logger.Errorf("[goja] %s", message) },
	}

	return vm.Set("trek", map[string]any{
		"action": actionAPI,
		"page":   pageAPI,
		"state":  stateAPI,
		"log":    logAPI,
	})
}

func actionObject(actionType ActionType, bounds []float64) map[string]any {
	action := map[string]any{"action": string(actionType)}
	if len(bounds) == 4 {
		action["bounds"] = bounds
	}
	return action
}

func scrollAction(direction string) ActionType {
	switch direction {
	case "top_down":
		return ActionScrollTopDown
	case "left_right":
		return ActionScrollLeftRight
	case "right_left":
		return ActionScrollRightLeft
	case "bottom_up_n":
		return ActionScrollBottomUpN
	default:
		return ActionScrollBottomUp
	}
}

func boolOption(options map[string]any, key string) bool {
	if options == nil {
		return false
	}
	val, _ := options[key].(bool)
	return val
}

func ctxToMap(ctx PluginContext) map[string]any {
	return map[string]any{
		"page":    pageToMap(ctx.Page),
		"runtime": runtimeToMap(ctx.Runtime),
	}
}

func stepResultCtxToMap(ctx StepResultContext) map[string]any {
	base := ctxToMap(ctx.PluginContext)
	base["result"] = map[string]any{
		"step":        ctx.Result.Step,
		"action":      actionToMap(ctx.Result.Action),
		"success":     ctx.Result.Success,
		"error":       ctx.Result.Error,
		"duration_ms": ctx.Result.DurationMs,
		"crash":       ctx.Result.Crash,
		"anr":         ctx.Result.ANR,
		"before":      pageToMap(ctx.Result.Before),
		"after":       pagePtrToMap(ctx.Result.After),
	}
	return base
}

func pageToMap(page PageSnapshot) map[string]any {
	result := map[string]any{
		"name":  page.Name,
		"xml":   page.XML,
		"nodes": nodesToMaps(page.Nodes),
	}
	if page.Screenshot != nil {
		result["screenshot"] = map[string]any{
			"bytes":  append([]byte(nil), page.Screenshot.Bytes...),
			"mime":   page.Screenshot.MIME,
			"size":   len(page.Screenshot.Bytes),
			"width":  page.Screenshot.Width,
			"height": page.Screenshot.Height,
		}
	}
	return result
}

func pagePtrToMap(page *PageSnapshot) any {
	if page == nil {
		return nil
	}
	return pageToMap(*page)
}

func nodesToMaps(nodes []PageNode) []map[string]any {
	result := make([]map[string]any, 0, len(nodes))
	for _, node := range nodes {
		result = append(result, map[string]any{
			"text":         node.Text,
			"resource_id":  node.ResourceID,
			"content_desc": node.ContentDesc,
			"class_name":   node.ClassName,
			"bounds":       []float64{node.Bounds[0], node.Bounds[1], node.Bounds[2], node.Bounds[3]},
			"clickable":    node.Clickable,
			"enabled":      node.Enabled,
			"editable":     node.Editable,
			"path":         node.Path,
			"xpath":        node.XPath,
		})
	}
	return result
}

func runtimeToMap(runtime RuntimeContext) map[string]any {
	result := map[string]any{
		"step":                 runtime.Step,
		"package_name":         runtime.PackageName,
		"page_source_type":     runtime.PageSourceType,
		"last_error":           runtime.LastError,
		"consecutive_failures": runtime.ConsecutiveFailures,
		"page_visit_count":     runtime.PageVisitCount,
		"action_count":         runtime.ActionCount,
	}
	if runtime.LastAction != nil {
		result["last_action"] = actionToMap(*runtime.LastAction)
	}
	if runtime.BlockRecovery != nil {
		result["block_recovery"] = map[string]any{
			"requested": runtime.BlockRecovery.Requested,
			"reason":    runtime.BlockRecovery.Reason,
		}
	}
	return result
}

func actionToMap(action Action) map[string]any {
	return map[string]any{
		"action":        string(action.Type),
		"bounds":        []float64{action.Bounds[0], action.Bounds[1], action.Bounds[2], action.Bounds[3]},
		"text":          action.Text,
		"clear":         action.Clear,
		"adb_input":     action.ADBInput,
		"allow_fuzzing": action.AllowFuzzing,
		"throttle":      action.Throttle,
		"wait_time":     action.WaitTime,
	}
}

func actionFromValue(value goja.Value) (*Action, error) {
	exported, ok := value.Export().(map[string]any)
	if !ok {
		return nil, fmt.Errorf("脚本动作必须是对象")
	}
	actionName, _ := exported["action"].(string)
	if actionName == "" {
		actionName, _ = exported["act"].(string)
	}
	if actionName == "" {
		return nil, fmt.Errorf("脚本动作缺少 action")
	}
	action := &Action{
		Type:         ActionType(actionName),
		Text:         stringValue(exported["text"]),
		Clear:        boolValue(exported["clear"]),
		ADBInput:     boolValue(exported["adb_input"]),
		AllowFuzzing: true,
		Throttle:     intValue(exported["throttle"]),
		WaitTime:     intValue(exported["wait_time"]),
	}
	if v, ok := exported["allow_fuzzing"]; ok {
		action.AllowFuzzing = boolValue(v)
	}
	if bounds, ok := boundsValue(exported["bounds"]); ok {
		action.Bounds = bounds
	}
	return action, nil
}

func boundsValue(value any) ([4]float64, bool) {
	var result [4]float64
	if value == nil {
		return result, false
	}
	switch v := value.(type) {
	case []any:
		if len(v) != 4 {
			return result, false
		}
		for i := range v {
			result[i] = floatValue(v[i])
		}
		return result, true
	case []float64:
		if len(v) != 4 {
			return result, false
		}
		copy(result[:], v)
		return result, true
	default:
		return result, false
	}
}

func floatValue(value any) float64 {
	switch v := value.(type) {
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case float32:
		return float64(v)
	case float64:
		return v
	default:
		return 0
	}
}

func intValue(value any) int {
	return int(floatValue(value))
}

func boolValue(value any) bool {
	v, _ := value.(bool)
	return v
}

func stringValue(value any) string {
	v, _ := value.(string)
	return v
}

func patchString(text string, from goja.Value, to string) string {
	if from == nil {
		return text
	}
	pattern := from.String()
	if strings.HasPrefix(pattern, "/") {
		lastSlash := strings.LastIndex(pattern, "/")
		if lastSlash > 0 {
			re, err := regexp.Compile(pattern[1:lastSlash])
			if err == nil {
				return re.ReplaceAllString(text, to)
			}
		}
	}
	return strings.ReplaceAll(text, pattern, to)
}

func findNode(page map[string]any, match func(map[string]any) bool) any {
	for _, node := range nodesFromPageMap(page) {
		if match(node) {
			return node
		}
	}
	return nil
}

func nodesFromPageMap(page map[string]any) []map[string]any {
	rawNodes, _ := page["nodes"].([]any)
	nodes := make([]map[string]any, 0, len(rawNodes))
	for _, rawNode := range rawNodes {
		if node, ok := rawNode.(map[string]any); ok {
			nodes = append(nodes, node)
		}
	}
	return nodes
}

func isEmptyJSValue(v goja.Value) bool {
	return v == nil || goja.IsUndefined(v) || goja.IsNull(v)
}
