package poco

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"trek/pkg/driver/common"

	"github.com/beevik/etree"
	"github.com/google/uuid"
)

var _ common.IPageSource = (*PocoPageSource)(nil)

type PocoPageSource struct {
	engine    Engine
	conn      PocoConnection
	source    string
	rawSource string
	isFrozen  bool
	mu        sync.RWMutex
}

func NewPocoPageSource(engine Engine) (*PocoPageSource, error) {
	return NewPocoPageSourceWith(engine, engine.GetDefaultPort())
}

func NewPocoPageSourceWith(engine Engine, port int) (*PocoPageSource, error) {
	p := &PocoPageSource{
		engine: engine,
	}

	var conn PocoConnection
	if engine.IsWebSocket() {
		conn = NewWebSocketClient(port)
	} else {
		conn = NewSocketClient(port)
	}

	if err := conn.Connect(); err != nil {
		return nil, fmt.Errorf("连接 Poco 失败: %v", err)
	}

	p.conn = conn

	return p, nil
}

func (p *PocoPageSource) DumpPageSource() (string, error) {
	p.mu.RLock()
	if p.isFrozen && p.source != "" {
		p.mu.RUnlock()
		return p.source, nil
	}
	p.mu.RUnlock()

	xmlSource, rawSource, err := p.fetchDumpSources()
	if err != nil {
		return "", err
	}

	p.mu.Lock()
	p.source = xmlSource
	p.rawSource = rawSource
	p.mu.Unlock()

	return xmlSource, nil
}

func (p *PocoPageSource) fetchDumpSources() (xmlSource string, rawSource string, err error) {
	method := "Dump"
	if p.engine == CocosCreator || p.engine == Cocos2dxJs {
		method = "dump"
	}

	req := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      uuid.New().String(),
		"method":  method,
		"params":  []interface{}{true},
	}

	reqData, err := json.Marshal(req)
	if err != nil {
		return "", "", fmt.Errorf("序列化请求失败: %v", err)
	}

	respData, err := p.conn.SendAndReceive(reqData)
	if err != nil {
		return "", "", fmt.Errorf("发送请求失败: %v", err)
	}

	var respMap map[string]interface{}
	if err := json.Unmarshal(respData, &respMap); err != nil {
		return "", "", fmt.Errorf("解析响应失败: %v", err)
	}

	result, ok := respMap["result"]
	if !ok {
		return "", "", fmt.Errorf("响应中未找到 result 字段")
	}

	resultBytes, err := json.Marshal(result)
	if err != nil {
		return "", "", fmt.Errorf("序列化 result 失败: %v", err)
	}

	xmlText, err := buildPocoDumpXML(result)
	if err != nil {
		return "", "", err
	}
	return xmlText, string(resultBytes), nil
}

func (p *PocoPageSource) FreezeSource() {
	p.mu.Lock()
	p.isFrozen = true
	p.mu.Unlock()
}

func (p *PocoPageSource) ThawSource() {
	p.mu.Lock()
	p.isFrozen = false
	p.mu.Unlock()
}

func (p *PocoPageSource) GetScreenSize() (int, int, error) {
	if p.engine != Unity3d && p.engine != Cocos2dxLua {
		return 0, 0, fmt.Errorf("当前引擎不支持获取屏幕尺寸")
	}

	method := "GetScreenSize"
	req := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      uuid.New().String(),
		"method":  method,
		"params":  []interface{}{true},
	}

	reqData, err := json.Marshal(req)
	if err != nil {
		return 0, 0, fmt.Errorf("序列化请求失败: %v", err)
	}

	respData, err := p.conn.SendAndReceive(reqData)
	if err != nil {
		return 0, 0, fmt.Errorf("发送请求失败: %v", err)
	}

	var respMap map[string]interface{}
	if err := json.Unmarshal(respData, &respMap); err != nil {
		return 0, 0, fmt.Errorf("解析响应失败: %v", err)
	}

	result, ok := respMap["result"]
	if !ok {
		return 0, 0, fmt.Errorf("响应中未找到 result 字段")
	}

	resultArray, ok := result.([]interface{})
	if !ok || len(resultArray) < 2 {
		return 0, 0, fmt.Errorf("result 格式错误")
	}

	width := int(resultArray[0].(float64))
	height := int(resultArray[1].(float64))

	return width, height, nil
}

func (p *PocoPageSource) Close() error {
	if p.conn != nil {
		p.conn.Disconnect()
	}
	return nil
}

func (p *PocoPageSource) FindElement(selector string, expression string) (*PocoElement, error) {
	elements, err := p.FindElements(selector, expression)
	if err != nil {
		return nil, err
	}
	if len(elements) == 0 {
		return nil, fmt.Errorf("未找到元素: selector=%s, expression=%s", selector, expression)
	}
	return elements[0], nil
}

func (p *PocoPageSource) FindElements(selector string, expression string) ([]*PocoElement, error) {
	source, err := p.dumpRawPageSource()
	if err != nil {
		return nil, err
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(source), &data); err != nil {
		return nil, fmt.Errorf("解析页面源码失败: %v", err)
	}

	xpath, err := p.parseSelector(selector, expression)
	if err != nil {
		return nil, err
	}

	nodes := findNodesByXPath(data, xpath)

	var elements []*PocoElement
	for _, node := range nodes {
		elements = append(elements, &PocoElement{
			Data: node,
		})
	}

	if len(elements) == 0 {
		return nil, fmt.Errorf("未找到元素: selector=%s, expression=%s", selector, expression)
	}

	return elements, nil
}

func (p *PocoPageSource) parseSelector(selector string, expression string) (string, error) {
	switch selector {
	case "poco":
		var xpath string
		steps := strings.Split(expression, ".")
		for _, step := range steps {
			if strings.HasPrefix(step, "poco") {
				xpath += "//*" + parseAttr(step)
			} else if strings.HasPrefix(step, "child") {
				xpath += "/*" + parseAttr(step)
			}

			if strings.Contains(step, "[") {
				start := strings.Index(step, "[")
				end := strings.Index(step, "]")
				index := step[start+1 : end]
				xpath = fmt.Sprintf("(%s)[%s]", xpath, index)
			}
		}
		return xpath, nil
	case "xpath":
		return expression, nil
	case "cssSelector":
		return "", fmt.Errorf("cssSelector 暂不支持")
	default:
		return "", fmt.Errorf("不支持的 selector: %s", selector)
	}
}

func parseAttr(expr string) string {
	result := "["
	open := strings.Index(expr, "(")
	close := strings.LastIndex(expr, ")")
	if open == -1 || close == -1 {
		return result + "]"
	}

	attrExpr := expr[open+1 : close]
	if strings.HasPrefix(attrExpr, "\"") && strings.HasSuffix(attrExpr, "\"") {
		attrExpr = "name=" + strings.Trim(attrExpr, "\"")
	}

	attrs := strings.Split(attrExpr, ",")
	for i, attr := range attrs {
		if i > 0 {
			result += " and "
		}
		parts := strings.SplitN(attr, "=", 2)
		if len(parts) == 2 {
			field := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(strings.ReplaceAll(parts[1], "\"", ""))
			result += fmt.Sprintf("@%s=\"%s\"", field, value)
		}
	}
	result += "]"
	return strings.Replace(result, " and ]", "]", 1)
}

func findNodesByXPath(data map[string]interface{}, xpath string) []map[string]interface{} {
	var results []map[string]interface{}

	xpath = strings.TrimPrefix(xpath, "/")

	parts := strings.Split(xpath, "/")
	if len(parts) == 0 {
		return results
	}

	first := parts[0]
	if first == "*" {
		if children, ok := data["children"].([]interface{}); ok {
			for _, child := range children {
				if childMap, ok := child.(map[string]interface{}); ok {
					if len(parts) == 1 {
						results = append(results, childMap)
					} else {
						results = append(results, findNodesByXPath(childMap, strings.Join(parts[1:], "/"))...)
					}
				}
			}
		}
	}

	return results
}

func (p *PocoPageSource) dumpRawPageSource() (string, error) {
	p.mu.RLock()
	if p.isFrozen && p.rawSource != "" {
		p.mu.RUnlock()
		return p.rawSource, nil
	}
	p.mu.RUnlock()

	xmlSource, rawSource, err := p.fetchDumpSources()
	if err != nil {
		return "", err
	}
	p.mu.Lock()
	p.source = xmlSource
	p.rawSource = rawSource
	p.mu.Unlock()
	return rawSource, nil
}

func buildPocoDumpXML(result interface{}) (string, error) {
	rootMap, ok := result.(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("poco dump result 类型错误")
	}
	doc := etree.NewDocument()
	hierarchy := doc.CreateElement("hierarchy")
	appendPocoNode(hierarchy, rootMap)
	doc.Indent(2)
	return doc.WriteToString()
}

func appendPocoNode(parent *etree.Element, nodeMap map[string]interface{}) {
	node := parent.CreateElement("node")
	payload, _ := nodeMap["payload"].(map[string]interface{})
	nodeName := toString(nodeMap["name"])
	if nodeName == "" {
		nodeName = toString(payload["name"])
	}
	setAttrIfNotEmpty(node, "name", nodeName)
	setAttrIfNotEmpty(node, "resource-id", nodeName)
	setAttrIfNotEmpty(node, "content-desc", toString(payload["tag"]))
	setAttrIfNotEmpty(node, "text", toString(payload["text"]))
	nodeType := toString(payload["type"])
	setAttrIfNotEmpty(node, "type", nodeType)
	setAttrIfNotEmpty(node, "class", nodeType)
	setAttrIfNotEmpty(node, "visible", strconv.FormatBool(toBool(payload["visible"])))
	setAttrIfNotEmpty(node, "enabled", strconv.FormatBool(toBool(payload["visible"])))
	clickable := toBool(payload["clickable"])
	setAttrIfNotEmpty(node, "clickable", strconv.FormatBool(clickable))
	setAttrIfNotEmpty(node, "long-clickable", "false")
	setAttrIfNotEmpty(node, "scrollable", strconv.FormatBool(hasAnyComponent(payload["components"], "ScrollRect", "Scrollbar", "Slider")))
	editable := strings.EqualFold(nodeType, "InputField")
	setAttrIfNotEmpty(node, "editable", strconv.FormatBool(editable))
	setAttrIfNotEmpty(node, "focusable", strconv.FormatBool(clickable || editable))
	setAttrIfNotEmpty(node, "focused", "false")
	setAttrIfNotEmpty(node, "checkable", "false")
	setAttrIfNotEmpty(node, "checked", "false")
	setAttrIfNotEmpty(node, "selected", "false")

	if bounds := buildBoundsFromPosSize(payload["pos"], payload["size"]); bounds != "" {
		setAttrIfNotEmpty(node, "bounds", bounds)
	}
	if pos := buildFloatArray(toFloatSlice(payload["pos"])); pos != "" {
		setAttrIfNotEmpty(node, "pos", pos)
	}
	if size := buildFloatArray(toFloatSlice(payload["size"])); size != "" {
		setAttrIfNotEmpty(node, "size", size)
	}
	if scale := buildFloatArray(toFloatSlice(payload["scale"])); scale != "" {
		setAttrIfNotEmpty(node, "scale", scale)
	}
	if anchor := buildFloatArray(toFloatSlice(payload["anchorPoint"])); anchor != "" {
		setAttrIfNotEmpty(node, "anchorPoint", anchor)
	}
	if components := strings.Join(toStringSlice(payload["components"]), ","); components != "" {
		setAttrIfNotEmpty(node, "components", components)
	}

	if children, ok := nodeMap["children"].([]interface{}); ok {
		for _, child := range children {
			childMap, ok := child.(map[string]interface{})
			if !ok {
				continue
			}
			appendPocoNode(node, childMap)
		}
	}
}

func setAttrIfNotEmpty(node *etree.Element, key string, value string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	node.CreateAttr(key, value)
}

func toString(v interface{}) string {
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x)
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", v))
	}
}

func toBool(v interface{}) bool {
	switch x := v.(type) {
	case bool:
		return x
	case string:
		return strings.EqualFold(strings.TrimSpace(x), "true")
	default:
		return false
	}
}

func toFloatSlice(v interface{}) []float64 {
	items, ok := v.([]interface{})
	if !ok {
		return nil
	}
	out := make([]float64, 0, len(items))
	for _, item := range items {
		switch f := item.(type) {
		case float64:
			out = append(out, f)
		case float32:
			out = append(out, float64(f))
		case int:
			out = append(out, float64(f))
		case int64:
			out = append(out, float64(f))
		}
	}
	return out
}

func toStringSlice(v interface{}) []string {
	items, ok := v.([]interface{})
	if !ok {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		text := toString(item)
		if text != "" {
			out = append(out, text)
		}
	}
	return out
}

func hasAnyComponent(v interface{}, names ...string) bool {
	components := toStringSlice(v)
	if len(components) == 0 {
		return false
	}
	for _, comp := range components {
		for _, name := range names {
			if strings.EqualFold(comp, name) {
				return true
			}
		}
	}
	return false
}

func buildBoundsFromPosSize(posV interface{}, sizeV interface{}) string {
	pos := toFloatSlice(posV)
	size := toFloatSlice(sizeV)
	if len(pos) < 2 || len(size) < 2 {
		return ""
	}
	left := pos[0] - size[0]/2
	top := pos[1] - size[1]/2
	right := pos[0] + size[0]/2
	bottom := pos[1] + size[1]/2
	return fmt.Sprintf("[%.6f,%.6f][%.6f,%.6f]", round6(left), round6(top), round6(right), round6(bottom))
}

func buildFloatArray(values []float64) string {
	if len(values) == 0 {
		return ""
	}
	parts := make([]string, 0, len(values))
	for _, v := range values {
		parts = append(parts, fmt.Sprintf("%.6f", round6(v)))
	}
	return strings.Join(parts, ",")
}

func round6(v float64) float64 {
	return math.Round(v*1e6) / 1e6
}
