package scripting

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/beevik/etree"
	"github.com/dop251/goja"
	"trek/internal/engine/perception/providers"
	enginestate "trek/internal/engine/state"
	"trek/logger"
)

var ErrPluginNotFound = errors.New("脚本未暴露 plugin 对象")

const (
	defaultHTTPTimeout      = 10 * time.Second
	maxHTTPTimeout          = 30 * time.Second
	maxHTTPResponseBodySize = 2 * 1024 * 1024
	maxScriptSleepDuration  = 30 * time.Second
)

type Manager struct {
	source string
	state  map[string]any
	vm     *goja.Runtime
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
	m.vm = vm
	return m, nil
}

// ResolvePageName 调用插件的 resolvePageName 钩子，返回自定义页面名。
func (m *Manager) ResolvePageName(ctx PluginContext) (string, error) {
	value, ok, err := m.callHook("resolvePageName", ctxToMap(ctx))
	if err != nil || !ok || isEmptyJSValue(value) {
		return "", err
	}
	name := strings.TrimSpace(value.String())
	return name, nil
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

func (m *Manager) OnInit(ctx LifecycleContext) error {
	_, _, err := m.callHook("onInit", lifecycleCtxToMap(ctx))
	return err
}

func (m *Manager) OnDestroy(ctx LifecycleContext) error {
	_, _, err := m.callHook("onDestroy", lifecycleCtxToMap(ctx))
	return err
}

func (m *Manager) StateGet(key string) any {
	return m.state[key]
}

func (m *Manager) callHook(name string, args ...any) (goja.Value, bool, error) {
	vm := m.vm
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
			action := actionObject(ActionInput, bounds)
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
		"findByXpath": func(page map[string]any, xpath string) any {
			xml, _ := page["xml"].(string)
			if xml == "" || xpath == "" {
				return nil
			}
			return findNodeByXPath(xml, xpath)
		},
		"excludeByText": func(xml string, text string) string {
			return strings.ReplaceAll(xml, text, "")
		},
		"excludeByResourceId": func(xml string, id string) string {
			return strings.ReplaceAll(xml, id, "")
		},
		"replaceText": func(xml string, from goja.Value, to string) string {
			return patchString(xml, from, to)
		},
		"replaceResourceId": func(xml string, from goja.Value, to string) string {
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
		"increment": func(key string, delta ...int64) int64 {
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

	httpAPI := map[string]any{
		"request": func(options map[string]any) (map[string]any, error) {
			return doScriptHTTPRequest(options)
		},
		"get": func(rawURL string, options ...map[string]any) (map[string]any, error) {
			req := mergeHTTPOptions(options)
			req["method"] = http.MethodGet
			req["url"] = rawURL
			return doScriptHTTPRequest(req)
		},
		"post": func(rawURL string, body any, options ...map[string]any) (map[string]any, error) {
			req := mergeHTTPOptions(options)
			req["method"] = http.MethodPost
			req["url"] = rawURL
			req["body"] = body
			return doScriptHTTPRequest(req)
		},
	}

	ocrAPI := map[string]any{
		"recognize": func(options map[string]any) (any, error) {
			return scriptOCRRecognize(options)
		},
	}

	llmAPI := map[string]any{
		"chat": func(options map[string]any) (string, error) {
			return scriptLLMChat(options)
		},
	}

	return vm.Set("trek", map[string]any{
		"action": actionAPI,
		"page":   pageAPI,
		"store":  stateAPI,
		"log":    logAPI,
		"http":   httpAPI,
		"ocr":    ocrAPI,
		"llm":    llmAPI,
		"sleep":  sleepScript,
	})
}

func actionObject(actionType ActionType, bounds []float64) map[string]any {
	action := map[string]any{"type": string(actionType)}
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

func lifecycleCtxToMap(ctx LifecycleContext) map[string]any {
	return map[string]any{
		"package_name":     ctx.PackageName,
		"page_source_type": ctx.PageSourceType,
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
		"name":      page.Name,
		"page_name": page.Name,
		"xml":       page.XML,
		"nodes":     nodesToMaps(page.Nodes),
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
	if runtime.BlockRecovery != nil && runtime.BlockRecovery.Requested {
		if runtime.BlockRecovery.Reason != "" {
			result["block_recovery"] = runtime.BlockRecovery.Reason
		} else {
			result["block_recovery"] = true
		}
	}
	return result
}

func actionToMap(action Action) map[string]any {
	return map[string]any{
		"type":          string(action.Type),
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
	actionName, _ := exported["type"].(string)
	if actionName == "" {
		actionName, _ = exported["action"].(string)
	}
	if actionName == "" {
		actionName, _ = exported["act"].(string)
	}
	if actionName == "" {
		return nil, fmt.Errorf("脚本动作缺少 type")
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

func envOrDefault(keys ...string) string {
	for _, key := range keys {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			return v
		}
	}
	return ""
}

func screenshotBytesFromJS(value any) []byte {
	switch v := value.(type) {
	case []byte:
		return v
	case []any:
		data := make([]byte, len(v))
		for i, item := range v {
			data[i] = byte(intValue(item))
		}
		return data
	case string:
		data, err := base64.StdEncoding.DecodeString(v)
		if err == nil {
			return data
		}
	}
	return nil
}

// scriptOCRRecognize 调用 OCR 服务识别截图中的文本区域。
//
//trek.ocr.recognize({ screenshot, endpoint?, api_key?, timeout_ms?, headers? })
func scriptOCRRecognize(options map[string]any) (any, error) {
	if options == nil {
		return nil, fmt.Errorf("trek.ocr.recognize 需要 options")
	}

	screenshot := screenshotBytesFromJS(options["screenshot"])
	if len(screenshot) == 0 {
		return nil, fmt.Errorf("trek.ocr.recognize 缺少 screenshot")
	}

	endpoint := stringValue(options["endpoint"])
	if endpoint == "" {
		endpoint = envOrDefault("PADDLEOCR_API_URL")
	}
	if endpoint == "" {
		return nil, fmt.Errorf("trek.ocr.recognize 缺少 endpoint（参数或 PADDLEOCR_API_URL 环境变量）")
	}

	apiKey := stringValue(options["api_key"])
	if apiKey == "" {
		apiKey = envOrDefault("PADDLEOCR_API_KEY")
	}

	timeout := scriptHTTPTimeout(options["timeout_ms"])

	provider, err := providers.NewOCRHTTPProvider(providers.OCRHTTPProviderConfig{
		Endpoint: endpoint,
		APIKey:   apiKey,
		Timeout:  timeout,
		Headers:  scriptHTTPHeaders(options["headers"]),
	})
	if err != nil {
		return nil, fmt.Errorf("trek.ocr.recognize 创建 provider 失败: %w", err)
	}

	candidates, err := provider.BuildCandidates(buildOCRTraversalContext(screenshot))
	if err != nil {
		return nil, fmt.Errorf("trek.ocr.recognize 调用失败: %w", err)
	}

	regions := make([]map[string]any, 0, len(candidates))
	for _, c := range candidates {
		var bounds []float64
		if c.Command != nil {
			bounds = []float64{c.Command.Pos.Left, c.Command.Pos.Top, c.Command.Pos.Right, c.Command.Pos.Bottom}
		}
		regions = append(regions, map[string]any{
			"text":       c.Intent,
			"confidence": c.Confidence,
			"bounds":     bounds,
		})
	}
	return regions, nil
}

func buildOCRTraversalContext(screenshot []byte) enginestate.TraversalContext {
	return enginestate.TraversalContext{
		Screenshot: screenshot,
	}
}

// scriptLLMChat 调用 LLM 多模态对话，返回文本响应。
//
//trek.llm.chat({ prompt, screenshot?, endpoint?, api_key?, model?, timeout_ms?, headers?, max_tokens? })
func scriptLLMChat(options map[string]any) (string, error) {
	if options == nil {
		return "", fmt.Errorf("trek.llm.chat 需要 options")
	}

	prompt := strings.TrimSpace(stringValue(options["prompt"]))
	if prompt == "" {
		return "", fmt.Errorf("trek.llm.chat 缺少 prompt")
	}

	screenshot := screenshotBytesFromJS(options["screenshot"])

	endpoint := stringValue(options["endpoint"])
	if endpoint == "" {
		endpoint = envOrDefault("LLM_API_URL", "OPENAI_API_URL")
	}
	if endpoint == "" {
		return "", fmt.Errorf("trek.llm.chat 缺少 endpoint（参数或 LLM_API_URL / OPENAI_API_URL 环境变量）")
	}

	apiKey := stringValue(options["api_key"])
	if apiKey == "" {
		apiKey = envOrDefault("LLM_API_KEY", "OPENAI_API_KEY")
	}

	model := stringValue(options["model"])
	if model == "" {
		model = envOrDefault("LLM_MODEL", "OPENAI_MODEL")
	}

	timeout := time.Duration(scriptHTTPTimeout(options["timeout_ms"]))
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	maxTokens := intValue(options["max_tokens"])
	if maxTokens <= 0 {
		maxTokens = 4096
	}

	payload, err := buildLLMChatPayload(prompt, screenshot, maxTokens)
	if err != nil {
		return "", fmt.Errorf("trek.llm.chat 构建请求失败: %w", err)
	}

	body, status, err := postLLMRequest(endpoint, apiKey, payload, timeout)
	if err != nil {
		return "", fmt.Errorf("trek.llm.chat 请求失败: %w", err)
	}
	if status < 200 || status >= 300 {
		return "", fmt.Errorf("trek.llm.chat 响应异常: status=%d body=%s", status, truncateScriptText(string(body), 512))
	}

	text, err := extractOpenAIChatContent(body)
	if err != nil {
		return "", fmt.Errorf("trek.llm.chat 解析响应失败: %w", err)
	}
	return text, nil
}

// buildLLMChatPayload 构建 OpenAI Chat Completions 格式的请求载荷。
func buildLLMChatPayload(prompt string, screenshot []byte, maxTokens int) ([]byte, error) {
	userContent := []map[string]any{
		{"type": "text", "text": prompt},
	}
	if len(screenshot) > 0 {
		b64 := base64.StdEncoding.EncodeToString(screenshot)
		userContent = append(userContent, map[string]any{
			"type": "image_url",
			"image_url": map[string]any{
				"url":    "data:image/png;base64," + b64,
				"detail": "high",
			},
		})
	}

	payload := map[string]any{
		"max_tokens": maxTokens,
		"messages": []map[string]any{
			{"role": "user", "content": userContent},
		},
	}
	return json.Marshal(payload)
}

// postLLMRequest 发送 HTTP POST 请求到 LLM 端点。
func postLLMRequest(endpoint, apiKey string, payload []byte, timeout time.Duration) ([]byte, int, error) {
	client := &http.Client{Timeout: timeout}
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	const maxBodySize = 50 * 1024 * 1024
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodySize))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return body, resp.StatusCode, nil
}

// extractOpenAIChatContent 从 OpenAI Chat Completions 响应中提取 assistant 消息文本。
func extractOpenAIChatContent(body []byte) (string, error) {
	var response struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return "", err
	}
	if len(response.Choices) == 0 {
		return "", fmt.Errorf("响应中缺少 choices")
	}
	text := strings.TrimSpace(response.Choices[0].Message.Content)
	if text == "" {
		return "", fmt.Errorf("响应内容为空")
	}
	return text, nil
}

func truncateScriptText(text string, max int) string {
	if max <= 0 || len(text) <= max {
		return text
	}
	if max <= 3 {
		return text[:max]
	}
	return text[:max-3] + "..."
}

func sleepScript(milliseconds int64) error {
	if milliseconds <= 0 {
		return nil
	}
	duration := time.Duration(milliseconds) * time.Millisecond
	if duration > maxScriptSleepDuration {
		return fmt.Errorf("trek.sleep 超过最大暂停时长: %s", maxScriptSleepDuration)
	}
	time.Sleep(duration)
	return nil
}

func mergeHTTPOptions(options []map[string]any) map[string]any {
	if len(options) == 0 || options[0] == nil {
		return map[string]any{}
	}
	result := make(map[string]any, len(options[0])+2)
	for key, value := range options[0] {
		result[key] = value
	}
	return result
}

func doScriptHTTPRequest(options map[string]any) (map[string]any, error) {
	if options == nil {
		return nil, fmt.Errorf("trek.http.request 需要 options")
	}
	method := strings.ToUpper(strings.TrimSpace(stringValue(options["method"])))
	if method == "" {
		method = http.MethodGet
	}
	rawURL := strings.TrimSpace(stringValue(options["url"]))
	if rawURL == "" {
		return nil, fmt.Errorf("trek.http.request 缺少 url")
	}
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("trek.http.request url 非法: %w", err)
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return nil, fmt.Errorf("trek.http.request 仅支持 http/https: %s", parsedURL.Scheme)
	}

	body, err := scriptHTTPBody(options["body"])
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(method, rawURL, body)
	if err != nil {
		return nil, err
	}
	for key, value := range scriptHTTPHeaders(options["headers"]) {
		req.Header.Set(key, value)
	}

	client := &http.Client{Timeout: scriptHTTPTimeout(options["timeout_ms"])}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	limited := io.LimitReader(resp.Body, maxHTTPResponseBodySize+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if len(data) > maxHTTPResponseBodySize {
		return nil, fmt.Errorf("trek.http 响应体超过上限: %d bytes", maxHTTPResponseBodySize)
	}

	return map[string]any{
		"status":      resp.StatusCode,
		"status_text": resp.Status,
		"ok":          resp.StatusCode >= 200 && resp.StatusCode < 300,
		"headers":     responseHeadersToMap(resp.Header),
		"body":        string(data),
		"bytes":       data,
	}, nil
}

func scriptHTTPBody(value any) (io.Reader, error) {
	if value == nil {
		return nil, nil
	}
	switch v := value.(type) {
	case string:
		return strings.NewReader(v), nil
	case []byte:
		return bytes.NewReader(v), nil
	case []any:
		data := make([]byte, 0, len(v))
		for _, item := range v {
			data = append(data, byte(intValue(item)))
		}
		return bytes.NewReader(data), nil
	default:
		return nil, fmt.Errorf("trek.http body 仅支持 string / byte[]")
	}
}

func scriptHTTPHeaders(value any) map[string]string {
	result := make(map[string]string)
	if value == nil {
		return result
	}
	headers, ok := value.(map[string]any)
	if !ok {
		return result
	}
	for key, raw := range headers {
		name := strings.TrimSpace(key)
		text := strings.TrimSpace(fmt.Sprint(raw))
		if name == "" || text == "" {
			continue
		}
		result[name] = text
	}
	return result
}

func scriptHTTPTimeout(value any) time.Duration {
	timeout := defaultHTTPTimeout
	if value != nil {
		if ms := intValue(value); ms > 0 {
			timeout = time.Duration(ms) * time.Millisecond
		}
	}
	if timeout > maxHTTPTimeout {
		return maxHTTPTimeout
	}
	return timeout
}

func responseHeadersToMap(headers http.Header) map[string]string {
	result := make(map[string]string, len(headers))
	for key, values := range headers {
		result[key] = strings.Join(values, ", ")
	}
	return result
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

func isEmptyJSValue(v goja.Value) bool {
	return v == nil || goja.IsUndefined(v) || goja.IsNull(v)
}

func findNodeByXPath(xml string, xpath string) map[string]any {
	doc := etree.NewDocument()
	if err := doc.ReadFromString(xml); err != nil {
		return nil
	}
	elem := doc.FindElement(xpath)
	if elem == nil {
		return nil
	}
	return elementToNodeMap(elem)
}

func elementToNodeMap(elem *etree.Element) map[string]any {
	boundsStr := elem.SelectAttrValue("bounds", "")
	var bounds [4]float64
	if boundsStr != "" {
		if b, ok := parseBoundsString(boundsStr); ok {
			bounds = b
		}
	}
	return map[string]any{
		"text":         elem.SelectAttrValue("text", ""),
		"resource_id":  elem.SelectAttrValue("resource-id", ""),
		"content_desc": elem.SelectAttrValue("content-desc", ""),
		"class_name":   elem.SelectAttrValue("class", ""),
		"bounds":       []float64{bounds[0], bounds[1], bounds[2], bounds[3]},
		"clickable":    elem.SelectAttrValue("clickable", "false") == "true",
		"enabled":      elem.SelectAttrValue("enabled", "false") == "true",
		"editable":     elem.SelectAttrValue("editable", "false") == "true",
		"xpath":        buildXPath(elem),
	}
}

func buildXPath(node *etree.Element) string {
	if node == nil {
		return ""
	}
	parent := node.Parent()
	if parent == nil || strings.TrimSpace(parent.Tag) == "" {
		return fmt.Sprintf("/%s[1]", node.Tag)
	}
	index := 0
	for _, sibling := range parent.ChildElements() {
		if sibling.Tag == node.Tag {
			index++
		}
		if sibling == node {
			break
		}
	}
	if index <= 0 {
		index = 1
	}
	return fmt.Sprintf("%s/%s[%d]", buildXPath(parent), node.Tag, index)
}

func parseBoundsString(s string) ([4]float64, bool) {
	parts := strings.Split(s, "][")
	if len(parts) != 2 {
		return [4]float64{}, false
	}
	lt := strings.Split(strings.Trim(parts[0], "[]"), ",")
	rt := strings.Split(strings.Trim(parts[1], "[]"), ",")
	if len(lt) != 2 || len(rt) != 2 {
		return [4]float64{}, false
	}
	vals := make([]float64, 4)
	for i, s := range append(lt, rt...) {
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return [4]float64{}, false
		}
		vals[i] = v
	}
	return [4]float64{vals[0], vals[1], vals[2], vals[3]}, true
}
