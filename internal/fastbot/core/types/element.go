package types

import (
	"Trek/internal/fastbot/tool"
	"Trek/log"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/beevik/etree"
)

// Xpath XPath结构
type Xpath struct {
	Clazz              string `json:"clazz"`
	ResourceID         string `json:"resource_id"`
	Text               string `json:"text"`
	ContentDescription string `json:"content_description"`
	Index              int    `json:"index"`
	OperationAND       bool   `json:"operation_and"`
	XpathStr           string `json:"-"`
}

// NewXpath 创建新的XPath
func NewXpath() *Xpath {
	return &Xpath{
		Index:        -1,
		OperationAND: true,
	}
}

// NewXpathFromString 从字符串创建XPath
func NewXpathFromString(xpathString string) *Xpath {
	return &Xpath{
		XpathStr:     xpathString,
		Index:        -1,
		OperationAND: true,
	}
}

// String 返回XPath字符串表示
func (x *Xpath) String() string {
	return x.XpathStr
}

// Element 元素结构
type Element struct {
	ResourceID    string
	Classname     string
	PackageName   string
	Text          string
	ContentDesc   string
	InputText     string
	PageName      string
	Enabled       bool
	Checked       bool
	Checkable     bool
	Clickable     bool
	Focusable     bool
	Scrollable    bool
	LongClickable bool
	ChildCount    int
	Focused       bool
	Index         int
	Password      bool
	Selected      bool
	IsEditable    bool
	Bounds        *Rect
	Children      []*Element
	Parent        *Element
	ValidText     string
}

// NewElement 创建新的元素
func NewElement() *Element {
	return &Element{
		Enabled:       false,
		Checked:       false,
		Checkable:     false,
		Clickable:     false,
		Focusable:     false,
		Scrollable:    false,
		LongClickable: false,
		ChildCount:    0,
		Focused:       false,
		Index:         -1,
		Password:      false,
		Selected:      false,
		IsEditable:    false,
		Bounds:        NewRect(0, 0, 0, 0),
		Children:      make([]*Element, 0),
		Parent:        nil,
	}
}

// GetClassname 获取类名
func (e *Element) GetClassname() string {
	return e.Classname
}

// GetResourceID 获取资源ID
func (e *Element) GetResourceID() string {
	return e.ResourceID
}

// GetText 获取文本
func (e *Element) GetText() string {
	return e.Text
}

// GetContentDesc 获取内容描述
func (e *Element) GetContentDesc() string {
	return e.ContentDesc
}

// GetPackageName 获取包名
func (e *Element) GetPackageName() string {
	return e.PackageName
}

// GetBounds 获取边界
func (e *Element) GetBounds() *Rect {
	return e.Bounds
}

// GetIndex 获取索引
func (e *Element) GetIndex() int {
	return e.Index
}

// GetClickable 获取是否可点击
func (e *Element) GetClickable() bool {
	return e.Clickable
}

// GetLongClickable 获取是否可长按
func (e *Element) GetLongClickable() bool {
	return e.LongClickable
}

// GetCheckable 获取是否可勾选
func (e *Element) GetCheckable() bool {
	return e.Checkable
}

// GetScrollable 获取是否可滚动
func (e *Element) GetScrollable() bool {
	return e.Scrollable
}

// GetEnable 获取是否启用
func (e *Element) GetEnable() bool {
	return e.Enabled
}

// GetChildren 获取子元素
func (e *Element) GetChildren() []*Element {
	return e.Children
}

// GetParent 获取父元素
func (e *Element) GetParent() *Element {
	return e.Parent
}

// GetScrollType 获取滚动类型
func (e *Element) GetScrollType() ScrollType {
	if !e.Scrollable {
		return NONE
	}

	// 根据类名精确判断滚动类型
	switch e.Classname {
	case "android.widget.ScrollView",
		"android.widget.ListView",
		"android.widget.ExpandableListView",
		"android.support.v17.leanback.widget.VerticalGridView",
		"android.support.v7.widget.RecyclerView",
		"androidx.recyclerview.widget.RecyclerView":
		return Vertical
	case "android.widget.HorizontalScrollView",
		"android.support.v17.leanback.widget.HorizontalGridView",
		"android.support.v4.view.ViewPager":
		return Horizontal
	}

	// 如果类名包含"ScrollView"，则支持所有方向滚动
	if strings.Contains(e.Classname, "ScrollView") {
		return ALL
	}

	// 默认情况下支持所有方向滚动（iOS兼容性）
	return ALL
}

// IsWebView 判断是否为WebView
func (e *Element) IsWebView() bool {
	return strings.Contains(strings.ToLower(e.Classname), "webview")
}

// IsEditText 判断是否为编辑文本
func (e *Element) IsEditText() bool {
	return strings.Contains(strings.ToLower(e.Classname), "edittext") || e.IsEditable
}

// ResetResourceID 重置资源ID
func (e *Element) ResetResourceID(resourceID string) {
	e.ResourceID = resourceID
}

// ResetContentDesc 重置内容描述
func (e *Element) ResetContentDesc(content string) {
	e.ContentDesc = content
}

// ResetText 重置文本
func (e *Element) ResetText(text string) {
	e.Text = text
}

// ResetIndex 重置索引
func (e *Element) ResetIndex(index int) {
	e.Index = index
}

// ResetClassname 重置类名
func (e *Element) ResetClassname(className string) {
	e.Classname = className
}

// ResetClickable 重置可点击状态
func (e *Element) ResetClickable(clickable bool) {
	e.Clickable = clickable
}

// ResetScrollable 重置可滚动状态
func (e *Element) ResetScrollable(scrollable bool) {
	e.Scrollable = scrollable
}

// ResetEnabled 重置启用状态
func (e *Element) ResetEnabled(enable bool) {
	e.Enabled = enable
}

// ResetBounds 重置边界
func (e *Element) ResetBounds(rect *Rect) {
	e.Bounds = rect
}

// ResetParent 重置父元素
func (e *Element) ResetParent(parent *Element) {
	e.Parent = parent
}

// AddChild 添加子元素
func (e *Element) AddChild(child *Element) {
	e.Children = append(e.Children, child)
	child.Parent = e
}

// MatchXpathSelector 匹配XPath选择器
func (e *Element) MatchXpathSelector(xpathSelector *Xpath) bool {
	if xpathSelector == nil {
		log.Debugf("xpath selector is null")
		return false
	}

	// 根据operationAND标志决定匹配逻辑
	isResourceIDEqual := xpathSelector.ResourceID != "" && e.ResourceID == xpathSelector.ResourceID
	isTextEqual := xpathSelector.Text != "" && e.Text == xpathSelector.Text
	isContentEqual := xpathSelector.ContentDescription != "" && e.ContentDesc == xpathSelector.ContentDescription
	isClassNameEqual := xpathSelector.Clazz != "" && e.Classname == xpathSelector.Clazz
	isIndexEqual := xpathSelector.Index > -1 && e.Index == xpathSelector.Index
	log.Debugf("begin find xpathSelector :\n XPathSelector:\n resourceID: %s text: %s contentDescription: %s clazz: %s index: %d \n UIPageElement:\n resourceID: %s text: %s contentDescription: %s clazz: %s index: %d \n equality: \n isResourceIDEqual:%t isTextEqual:%t isContentEqual:%t isClassNameEqual:%t isIndexEqual:%t",
		xpathSelector.ResourceID,
		xpathSelector.Text,
		xpathSelector.ContentDescription,
		xpathSelector.Clazz,
		xpathSelector.Index,
		e.ResourceID,
		e.Text,
		e.ContentDesc,
		e.Classname,
		e.Index,
		isResourceIDEqual,
		isTextEqual,
		isContentEqual,
		isClassNameEqual,
		isIndexEqual)

	if xpathSelector.OperationAND {
		// AND逻辑：所有非空条件都必须匹配
		match := true
		if xpathSelector.Clazz != "" {
			match = isClassNameEqual
		}
		if xpathSelector.ContentDescription != "" {
			match = match && isContentEqual
		}
		if xpathSelector.Text != "" {
			match = match && isTextEqual
		}
		if xpathSelector.ResourceID != "" {
			match = match && isResourceIDEqual
		}
		if xpathSelector.Index != -1 {
			match = match && isIndexEqual
		}
		return match
	} else {
		// OR逻辑：任一条件匹配即可
		return isResourceIDEqual || isTextEqual || isContentEqual || isClassNameEqual
	}
}

// DeleteElement 删除元素
func (e *Element) DeleteElement() {
	if e.Parent != nil {
		for i, child := range e.Parent.Children {
			if child == e {
				e.Parent.Children = append(e.Parent.Children[:i], e.Parent.Children[i+1:]...)
				break
			}
		}
	} else {
		log.Errorf("element is a root elements")
	}
}

// RecursiveElements 递归获取元素
func (e *Element) RecursiveElements(filter func(*Element) bool, result []*Element) []*Element {
	if filter(e) {
		result = append(result, e)
	}
	for _, child := range e.Children {
		result = child.RecursiveElements(filter, result)
	}
	return result
}

// RecursiveDoElements 递归执行操作
func (e *Element) RecursiveDoElements(doFunc func(*Element)) {
	doFunc(e)
	for _, child := range e.Children {
		child.RecursiveDoElements(doFunc)
	}
}

// ToJSON 转换为JSON
func (e *Element) ToJSON() string {
	data := map[string]interface{}{
		"resource_id":    e.ResourceID,
		"class":          e.Classname,
		"package":        e.PackageName,
		"text":           e.Text,
		"content_desc":   e.ContentDesc,
		"enabled":        e.Enabled,
		"checked":        e.Checked,
		"checkable":      e.Checkable,
		"clickable":      e.Clickable,
		"focusable":      e.Focusable,
		"scrollable":     e.Scrollable,
		"long_clickable": e.LongClickable,
		"child_count":    e.ChildCount,
		"focused":        e.Focused,
		"index":          e.Index,
		"password":       e.Password,
		"selected":       e.Selected,
		"editable":       e.IsEditable,
		"bounds":         e.Bounds,
		"valid_text":     e.ValidText,
	}

	jsonBytes, _ := json.Marshal(data)
	return string(jsonBytes)
}

// ToXML 转换为XML
func (e *Element) ToXML() string {
	doc := etree.NewDocument()
	root := e.toXMLNode(doc)
	doc.SetRoot(root)
	xmlStr, err := doc.WriteToString()
	if err != nil {
		return ""
	}
	return xmlStr
}

// toXMLNode 转换为XML节点
func (e *Element) toXMLNode(doc *etree.Document) *etree.Element {
	node := doc.CreateElement("node")

	node.CreateAttr("bounds", e.Bounds.String())

	node.CreateAttr("class", e.Classname)
	node.CreateAttr("resource-id", e.ResourceID)
	node.CreateAttr("package", e.PackageName)
	node.CreateAttr("text", e.Text)
	node.CreateAttr("content-types", e.ContentDesc)
	node.CreateAttr("enabled", strconv.FormatBool(e.Enabled))
	node.CreateAttr("checked", strconv.FormatBool(e.Checked))
	node.CreateAttr("checkable", strconv.FormatBool(e.Checkable))
	node.CreateAttr("clickable", strconv.FormatBool(e.Clickable))
	node.CreateAttr("focusable", strconv.FormatBool(e.Focusable))
	node.CreateAttr("scrollable", strconv.FormatBool(e.Scrollable))
	node.CreateAttr("long-clickable", strconv.FormatBool(e.LongClickable))
	node.CreateAttr("child-count", strconv.Itoa(e.ChildCount))
	node.CreateAttr("focused", strconv.FormatBool(e.Focused))
	node.CreateAttr("index", strconv.Itoa(e.Index))
	node.CreateAttr("password", strconv.FormatBool(e.Password))
	node.CreateAttr("selected", strconv.FormatBool(e.Selected))
	node.CreateAttr("editable", strconv.FormatBool(e.IsEditable))

	for _, child := range e.Children {
		childNode := child.toXMLNode(doc)
		node.AddChild(childNode)
	}

	return node
}

// FromJSON 从JSON创建元素
func (e *Element) FromJSON(jsonData string) error {
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(jsonData), &data); err != nil {
		return err
	}

	if val, ok := data["resource_id"].(string); ok {
		e.ResourceID = val
	}
	if val, ok := data["class"].(string); ok {
		e.Classname = val
	}
	if val, ok := data["package"].(string); ok {
		e.PackageName = val
	}
	if val, ok := data["text"].(string); ok {
		e.Text = val
	}
	if val, ok := data["content_desc"].(string); ok {
		e.ContentDesc = val
	}
	if val, ok := data["enabled"].(bool); ok {
		e.Enabled = val
	}
	if val, ok := data["checked"].(bool); ok {
		e.Checked = val
	}
	if val, ok := data["checkable"].(bool); ok {
		e.Checkable = val
	}
	if val, ok := data["clickable"].(bool); ok {
		e.Clickable = val
	}
	if val, ok := data["focusable"].(bool); ok {
		e.Focusable = val
	}
	if val, ok := data["scrollable"].(bool); ok {
		e.Scrollable = val
	}
	if val, ok := data["long_clickable"].(bool); ok {
		e.LongClickable = val
	}
	if val, ok := data["child_count"].(float64); ok {
		e.ChildCount = int(val)
	}
	if val, ok := data["focused"].(bool); ok {
		e.Focused = val
	}
	if val, ok := data["index"].(float64); ok {
		e.Index = int(val)
	}
	if val, ok := data["password"].(bool); ok {
		e.Password = val
	}
	if val, ok := data["selected"].(bool); ok {
		e.Selected = val
	}
	if val, ok := data["editable"].(bool); ok {
		e.IsEditable = val
	}
	if val, ok := data["valid_text"].(string); ok {
		e.ValidText = val
	}

	return nil
}

// String 实现Stringer接口
func (e *Element) String() string {
	return fmt.Sprintf("Element{class:%s, text:%s, resource-id:%s, bounds:%s}",
		e.Classname, e.Text, e.ResourceID, e.Bounds.String())
}

// GetValidText 获取有效文本
func (e *Element) GetValidText() string {
	return e.ValidText
}

// CreateFromXml 从XML创建元素
func CreateFromXml(xmlContent string) (*Element, error) {

	doc := etree.NewDocument()
	if err := doc.ReadFromString(xmlContent); err != nil {
		log.Errorf("parse xml error: %v", err)
		return nil, err
	}
	log.Debugf("The content of XML is: %s", xmlContent)
	return createFromXmlDoc(doc)
}

// createFromXmlDoc 从XML文档创建元素
func createFromXmlDoc(doc *etree.Document) (*Element, error) {
	root := doc.Root()
	if root == nil {
		return nil, fmt.Errorf("no root element found")
	}

	element := NewElement()
	element.fromXMLNode(root, nil)

	element.Scrollable = true

	return element, nil
}

// fromXMLNode 从XML节点创建元素
func (e *Element) fromXMLNode(xmlNode *etree.Element, parent *Element) {
	e.Parent = parent

	// 解析属性
	if val := xmlNode.SelectAttrValue("class", ""); val != "" {
		e.Classname = val
	}
	if val := xmlNode.SelectAttrValue("resource-id", ""); val != "" {
		e.ResourceID = val
	}
	if val := xmlNode.SelectAttrValue("package", ""); val != "" {
		e.PackageName = val
	}
	if val := xmlNode.SelectAttrValue("text", ""); val != "" {
		e.Text = val
	}
	if val := xmlNode.SelectAttrValue("content-types", ""); val != "" {
		e.ContentDesc = val
	}
	if val := xmlNode.SelectAttrValue("enabled", ""); val != "" {
		e.Enabled = val == "true"
	}
	if val := xmlNode.SelectAttrValue("checked", ""); val != "" {
		e.Checked = val == "true"
	}
	if val := xmlNode.SelectAttrValue("checkable", ""); val != "" {
		e.Checkable = val == "true"
	}
	if val := xmlNode.SelectAttrValue("clickable", ""); val != "" {
		e.Clickable = val == "true"
	}
	if val := xmlNode.SelectAttrValue("focusable", ""); val != "" {
		e.Focusable = val == "true"
	}
	if val := xmlNode.SelectAttrValue("scrollable", ""); val != "" {
		e.Scrollable = val == "true"
	}
	if val := xmlNode.SelectAttrValue("long-clickable", ""); val != "" {
		e.LongClickable = val == "true"
	}
	if val := xmlNode.SelectAttrValue("child-count", ""); val != "" {
		if num, err := strconv.Atoi(val); err == nil {
			e.ChildCount = num
		}
	}
	if val := xmlNode.SelectAttrValue("focused", ""); val != "" {
		e.Focused = val == "true"
	}
	if val := xmlNode.SelectAttrValue("index", ""); val != "" {
		if num, err := strconv.Atoi(val); err == nil {
			e.Index = num
		}
	}
	if val := xmlNode.SelectAttrValue("password", ""); val != "" {
		e.Password = val == "true"
	}
	if val := xmlNode.SelectAttrValue("selected", ""); val != "" {
		e.Selected = val == "true"
	}
	if val := xmlNode.SelectAttrValue("editable", ""); val != "" {
		e.IsEditable = val == "true"
	}
	if val := xmlNode.SelectAttrValue("bounds", ""); val != "" {
		e.Bounds = parseBounds(val)
	}

	// C++版本中的逻辑：如果元素可点击或可长按，则启用它
	if e.Clickable || e.LongClickable {
		e.Enabled = true
	}

	// C++版本中的逻辑：强制为EditText元素添加点击和启用功能
	if FORCE_EDITTEXT_CLICK_TRUE && e.IsEditText() {
		e.LongClickable = true
		e.Clickable = true
		e.Enabled = true
	}

	// 递归处理子节点
	for _, childNode := range xmlNode.ChildElements() {
		child := NewElement()
		child.fromXMLNode(childNode, e)
		e.AddChild(child)
	}
}

// parseBounds 解析边界字符串
func parseBounds(boundsStr string) *Rect {
	// 边界格式通常是 "[left,top][right,bottom]"
	parts := strings.Split(boundsStr, "][")
	if len(parts) != 2 {
		return NewRect(0, 0, 0, 0)
	}

	leftTop := strings.Trim(parts[0], "[]")
	rightBottom := strings.Trim(parts[1], "[]")

	lt := strings.Split(leftTop, ",")
	rb := strings.Split(rightBottom, ",")

	if len(lt) != 2 || len(rb) != 2 {
		return NewRect(0, 0, 0, 0)
	}

	left, _ := strconv.Atoi(lt[0])
	top, _ := strconv.Atoi(lt[1])
	right, _ := strconv.Atoi(rb[0])
	bottom, _ := strconv.Atoi(rb[1])

	return NewRect(left, top, right, bottom)
}

func (e *Element) Hash(recursive bool) uintptr {
	hashcode := uintptr(0x1)
	hashcode1 := uintptr(127) * tool.HashString(e.ResourceID) << 1
	hashcode2 := tool.HashString(e.Classname) << 2
	hashcode3 := tool.HashString(e.PackageName) << 3
	hashcode4 := uintptr(256) * tool.HashString(e.Text) << 4
	hashcode5 := tool.HashString(e.ContentDesc) << 5
	hashcode6 := tool.HashString(e.PageName) << 2
	hashcode7 := uintptr(64) * tool.HashInt(boolToInt(e.Clickable)) << 6

	hashcode = hashcode1 ^ hashcode2 ^ hashcode3 ^ hashcode4 ^ hashcode5 ^ hashcode6 ^ hashcode7

	if recursive {
		for i := 0; i < len(e.Children); i++ {
			childHash := e.Children[i].Hash(false) << 2
			hashcode ^= childHash
			hashcode ^= uintptr(0x7398c + (tool.HashInt(i) << 8))
		}
	}

	return hashcode
}

// boolToInt 将bool转换为int
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
