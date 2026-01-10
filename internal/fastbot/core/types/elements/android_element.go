package elements

import (
	"Trek/internal/fastbot/core/model"
	"Trek/internal/fastbot/core/types"
	"Trek/internal/fastbot/tool"
	"Trek/log"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/beevik/etree"
)

var _ types.IElement = (*AndroidElement)(nil)

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

// AndroidElement 元素结构
type AndroidElement struct {
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
	Bounds        *types.Rect
	Children      []types.IElement
	Parent        *AndroidElement
	ValidText     string
	xpath         string
}

func (e *AndroidElement) SimpleIdentifier() uintptr {
	hashcode1 := tool.HashString(e.Classname)
	hashcode2 := tool.HashString(e.ResourceID)
	return hashcode1 ^ (hashcode2 << 4)
}

func (e *AndroidElement) GetPath() string {
	return e.xpath
}

// GetClassname 获取类名
func (e *AndroidElement) GetClassname() string {
	return e.Classname
}

// GetResourceID 获取资源ID
func (e *AndroidElement) GetResourceID() string {
	return e.ResourceID
}

// GetText 获取文本
func (e *AndroidElement) GetText() string {
	return e.Text
}

// GetContentDesc 获取内容描述
func (e *AndroidElement) GetContentDesc() string {
	return e.ContentDesc
}

// GetPackageName 获取包名
func (e *AndroidElement) GetPackageName() string {
	return e.PackageName
}

// GetBounds 获取边界
func (e *AndroidElement) GetBounds() *types.Rect {
	return e.Bounds
}

// GetIndex 获取索引
func (e *AndroidElement) GetIndex() int {
	return e.Index
}

// GetClickable 获取是否可点击
func (e *AndroidElement) GetClickable() bool {
	return e.Clickable
}

// GetLongClickable 获取是否可长按
func (e *AndroidElement) GetLongClickable() bool {
	return e.LongClickable
}

// GetCheckBoxable 获取是否可勾选
func (e *AndroidElement) GetCheckBoxable() bool {
	return e.Checkable
}

// GetScrollable 获取是否可滚动
func (e *AndroidElement) GetScrollable() bool {
	return e.Scrollable
}

// GetEnable 获取是否启用
func (e *AndroidElement) GetEnable() bool {
	return e.Enabled
}

// GetChildren 获取子元素
func (e *AndroidElement) GetChildren() []types.IElement {
	return e.Children
}

// GetParent 获取父元素
func (e *AndroidElement) GetParent() types.IElement {
	return e.Parent
}

// GetScrollType 获取滚动类型
func (e *AndroidElement) GetScrollType() types.ScrollType {
	if !e.Scrollable {
		return types.NONE
	}

	// 根据类名精确判断滚动类型
	switch e.Classname {
	case "android.widget.ScrollView",
		"android.widget.ListView",
		"android.widget.ExpandableListView",
		"android.support.v17.leanback.widget.VerticalGridView",
		"android.support.v7.widget.RecyclerView",
		"androidx.recyclerview.widget.RecyclerView":
		return types.Vertical
	case "android.widget.HorizontalScrollView",
		"android.support.v17.leanback.widget.HorizontalGridView",
		"android.support.v4.view.ViewPager":
		return types.Horizontal
	}

	// 如果类名包含"ScrollView"，则支持所有方向滚动
	if strings.Contains(e.Classname, "ScrollView") {
		return types.ALL
	}

	// 默认情况下支持所有方向滚动（iOS兼容性）
	return types.ALL
}

// IsWebView 判断是否为WebView
func (e *AndroidElement) IsWebView() bool {
	return strings.Contains(strings.ToLower(e.Classname), "webview")
}

// IsEditText 判断是否为编辑文本
func (e *AndroidElement) IsEditText() bool {
	return strings.Contains(strings.ToLower(e.Classname), "edittext") || e.IsEditable
}

// ResetResourceID 重置资源ID
func (e *AndroidElement) ResetResourceID(resourceID string) {
	e.ResourceID = resourceID
}

// ResetContentDesc 重置内容描述
func (e *AndroidElement) ResetContentDesc(content string) {
	e.ContentDesc = content
}

// ResetText 重置文本
func (e *AndroidElement) ResetText(text string) {
	e.Text = text
}

// ResetIndex 重置索引
func (e *AndroidElement) ResetIndex(index int) {
	e.Index = index
}

// ResetClassname 重置类名
func (e *AndroidElement) ResetClassname(className string) {
	e.Classname = className
}

// ResetClickable 重置可点击状态
func (e *AndroidElement) ResetClickable(clickable bool) {
	e.Clickable = clickable
}

// ResetScrollable 重置可滚动状态
func (e *AndroidElement) ResetScrollable(scrollable bool) {
	e.Scrollable = scrollable
}

// ResetEnabled 重置启用状态
func (e *AndroidElement) ResetEnabled(enable bool) {
	e.Enabled = enable
}

// ResetBounds 重置边界
func (e *AndroidElement) ResetBounds(rect *types.Rect) {
	e.Bounds = rect
}

// ResetParent 重置父元素
func (e *AndroidElement) ResetParent(parent *AndroidElement) {
	e.Parent = parent
}

// AddChild 添加子元素
func (e *AndroidElement) AddChild(child *AndroidElement) {
	e.Children = append(e.Children, child)
	child.Parent = e
}

// MatchXpathSelector 匹配XPath选择器
func (e *AndroidElement) MatchXpathSelector(xpathSelector *Xpath) bool {
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
func (e *AndroidElement) DeleteElement() {
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
func (e *AndroidElement) RecursiveElements(filter func(*AndroidElement) bool, result []*AndroidElement) []*AndroidElement {
	if filter(e) {
		result = append(result, e)
	}
	for _, child := range e.Children {
		if androidChild, ok := child.(*AndroidElement); ok {
			result = androidChild.RecursiveElements(filter, result)
		}

	}
	return result
}

// RecursiveDoElements 递归执行操作
func (e *AndroidElement) RecursiveDoElements(doFunc func(*AndroidElement)) {
	doFunc(e)
	for _, child := range e.Children {
		if androidChild, ok := child.(*AndroidElement); ok {
			androidChild.RecursiveDoElements(doFunc)
		}
	}
}

// ToJSON 转换为JSON
func (e *AndroidElement) ToJSON() string {
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
func (e *AndroidElement) ToXML() string {
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
func (e *AndroidElement) toXMLNode(doc *etree.Document) *etree.Element {
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
		if androidChild, ok := child.(*AndroidElement); ok {
			childNode := androidChild.toXMLNode(doc)
			node.AddChild(childNode)
		}
	}

	return node
}

// FromJSON 从JSON创建元素
func (e *AndroidElement) FromJSON(jsonData string) error {
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

	if e.Classname == "android.widget.EditText" ||
		e.Classname == "android.inputmethodservice.ExtractEditText" ||
		e.Classname == "android.widget.AutoCompleteTextView" ||
		e.Classname == "android.widget.MultiAutoCompleteTextView" {
		e.IsEditable = true
	}

	return nil
}

// String 实现Stringer接口
func (e *AndroidElement) String() string {
	return fmt.Sprintf("AndroidElement{class:%s, text:%s, resource-id:%s, bounds:%s}",
		e.Classname, e.Text, e.ResourceID, e.Bounds.String())
}

// GetValidText 获取有效文本
func (e *AndroidElement) GetValidText() string {
	return e.ValidText
}

// NewElement 创建新的元素
func NewElement() *AndroidElement {
	return &AndroidElement{
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
		Bounds:        types.NewRect(0, 0, 0, 0),
		Children:      make([]types.IElement, 0),
		Parent:        nil,
	}
}

func init() {
	model.RegisterElementCreator(string(ANDROID_ELEMENT), CreateFromXml)
}

// CreateFromXml 从XML创建元素
func CreateFromXml(xmlContent string) (types.IElement, error) {

	doc := etree.NewDocument()
	if err := doc.ReadFromString(xmlContent); err != nil {
		log.Errorf("parse xml error: %v", err)
		return nil, err
	}
	log.Debugf("The content of XML is: %s", xmlContent)
	return createFromXmlDoc(doc)
}

// createFromXmlDoc 从XML文档创建元素
func createFromXmlDoc(doc *etree.Document) (*AndroidElement, error) {
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
func (e *AndroidElement) fromXMLNode(xmlNode *etree.Element, parent *AndroidElement) {

	e.xpath = xmlNode.GetPath()

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
	if types.FORCE_EDITTEXT_CLICK_TRUE && e.IsEditText() {
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
func parseBounds(boundsStr string) *types.Rect {
	// 边界格式通常是 "[left,top][right,bottom]"
	parts := strings.Split(boundsStr, "][")
	if len(parts) != 2 {
		return types.NewRect(0, 0, 0, 0)
	}

	leftTop := strings.Trim(parts[0], "[]")
	rightBottom := strings.Trim(parts[1], "[]")

	lt := strings.Split(leftTop, ",")
	rb := strings.Split(rightBottom, ",")

	if len(lt) != 2 || len(rb) != 2 {
		return types.NewRect(0, 0, 0, 0)
	}

	left, _ := strconv.Atoi(lt[0])
	top, _ := strconv.Atoi(lt[1])
	right, _ := strconv.Atoi(rb[0])
	bottom, _ := strconv.Atoi(rb[1])

	return types.NewRect(left, top, right, bottom)
}

func (e *AndroidElement) Hash(recursive bool) uintptr {
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
			if child, ok := e.Children[i].(*AndroidElement); ok {
				childHash := child.Hash(false) << 2
				hashcode ^= childHash
				hashcode ^= uintptr(0x7398c + (tool.HashInt(i) << 8))
			}
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
