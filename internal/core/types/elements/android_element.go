package elements

import (
	"Trek/internal/core/model"
	types2 "Trek/internal/core/types"
	"Trek/internal/tool"
	"Trek/log"
	"fmt"
	"strconv"
	"strings"

	"github.com/beevik/etree"
)

var _ types2.IElement = (*AndroidElement)(nil)

// NewElement 创建新的元素
func NewElement() *AndroidElement {
	return &AndroidElement{
		Children: make([]types2.IElement, 0),
		Parent:   nil,
	}
}

func init() {
	model.RegisterElementCreator(string(ANDROID_ELEMENT), CreateFromXml)
}

// CreateFromXml 从XML创建元素
func CreateFromXml(xmlContent string) (types2.IElement, error) {

	doc := etree.NewDocument()
	if err := doc.ReadFromString(xmlContent); err != nil {
		log.Errorf("parse xml error: %v", err)
		return nil, err
	}
	log.Debugf("The content of XML is: %s", xmlContent)
	elem, err := createFromXmlDoc(doc)
	if err != nil {
		return nil, err
	}

	// 如果整个界面，没有任何一个可点击项，则所有的元素都设置成可点击项
	var noSetAllElementClickable = false

	elem.RecursiveDoElements(func(element *AndroidElement) {
		if noSetAllElementClickable {
			return
		}
		noSetAllElementClickable = element.GetClickable()
	})

	if !noSetAllElementClickable {
		elem.RecursiveDoElements(func(element *AndroidElement) {
			element.SetClickable(true)
		})
	}

	return elem, nil
}

// createFromXmlDoc 从XML文档创建元素
func createFromXmlDoc(doc *etree.Document) (*AndroidElement, error) {
	root := doc.Root()
	if root == nil {
		return nil, fmt.Errorf("no root element found")
	}

	element := NewElement()
	element.fromXMLNode(root, nil)

	element.SetScrollable(true)

	return element, nil
}

// AndroidElement 元素结构
type AndroidElement struct {
	Children []types2.IElement
	Parent   *AndroidElement

	path string

	eNode *etree.Element
}

func (e *AndroidElement) SimpleIdentifier() uintptr {
	hashcode1 := tool.HashString(e.GetClassname())
	hashcode2 := tool.HashString(e.GetResourceID())
	return hashcode1 ^ (hashcode2 << 4)
}

func (e *AndroidElement) GetPath() string {
	return e.path
}

// GetClassname 获取类名
func (e *AndroidElement) GetClassname() string {
	return e.eNode.SelectAttrValue("class", "attribute class name get failed")
}

// GetResourceID 获取资源ID
func (e *AndroidElement) GetResourceID() string {
	return e.eNode.SelectAttrValue("resource-id", "attribute resource_id get failed")
}

// GetContentDesc 获取内容描述
func (e *AndroidElement) GetContentDesc() string {
	return e.eNode.SelectAttrValue("content-types", "attribute content description get failed")
}

// GetPackageName 获取包名
func (e *AndroidElement) GetPackageName() string {
	return e.eNode.SelectAttrValue("package", "attribute package name get failed")
}

// GetClickable 获取是否可点击
func (e *AndroidElement) GetClickable() bool {

	if val := e.eNode.SelectAttrValue("clickable", ""); val != "" {
		return val == "true"
	}
	return false
}

func (e *AndroidElement) SetClickable(clickable bool) {
	e.eNode.CreateAttr("clickable", fmt.Sprintf("%v", clickable))
}

func (e *AndroidElement) GetScrollable() bool {
	return e.eNode.SelectAttrValue("scrollable", "") == "true"
}

func (e *AndroidElement) SetScrollable(scrollable bool) {
	e.eNode.CreateAttr("scrollable", fmt.Sprintf("%v", scrollable))
}

// GetLongClickable 获取是否可长按
func (e *AndroidElement) GetLongClickable() bool {

	if val := e.eNode.SelectAttrValue("long-clickable", ""); val != "" {
		return val == "true"
	}
	return false
}

func (e *AndroidElement) SetLongClickable(longClickable bool) {
	e.eNode.CreateAttr("long-clickable", fmt.Sprintf("%v", longClickable))
}

// GetCheckBoxable 获取是否可勾选
func (e *AndroidElement) GetCheckBoxable() bool {
	if val := e.eNode.SelectAttrValue("checkable", ""); val != "" {
		return val == "true"
	}
	return false
}

func (e *AndroidElement) SetCheckBoxable(checked bool) {
	e.eNode.CreateAttr("checkable", fmt.Sprintf("%v", checked))
}

// GetEnable 获取是否启用
func (e *AndroidElement) GetEnable() bool {

	if val := e.eNode.SelectAttrValue("enabled", ""); val != "" {
		return val == "true"
	}
	return false
}

func (e *AndroidElement) SetEnable(enable bool) {
	e.eNode.CreateAttr("enabled", fmt.Sprintf("%v", enable))
}

// GetText 获取文本
func (e *AndroidElement) GetText() string {
	if val := e.eNode.SelectAttrValue("text", "get text attribute failed"); val != "" {
		return val
	}
	return ""
}

func (e *AndroidElement) SetText(text string) {
	e.eNode.CreateAttr("text", text)
}

// GetBounds 获取边界
func (e *AndroidElement) GetBounds() *types2.Rect {

	var bounds *types2.Rect

	if val := e.eNode.SelectAttrValue("bounds", "get bounds attribute failed"); val != "" {
		bounds = parseBounds(val)
	}

	return bounds
}

func (e *AndroidElement) SetBounds(rect *types2.Rect) {
	// [left,top][right,bottom]
	e.eNode.CreateAttr("bounds", fmt.Sprintf("[%d,%d][%d,%d]", rect.Left, rect.Top, rect.Right, rect.Bottom))
}

// GetChildren 获取子元素
func (e *AndroidElement) GetChildren() []types2.IElement {
	return e.Children
}

// GetParent 获取父元素
func (e *AndroidElement) GetParent() types2.IElement {
	if e.Parent == nil {
		element := NewElement()
		element.fromXMLNode(e.eNode.Parent(), nil)
		e.Parent = element
	}

	return e.Parent
}

// GetScrollType 获取滚动类型
func (e *AndroidElement) GetScrollType() types2.ScrollType {
	// 如果有值，肯定是被用户自己修改过了，所以优先级最高
	if val := e.eNode.SelectAttrValue("scrollType", ""); val != "" {
		return types2.StringToScrollType(val)
	}

	if !e.GetScrollable() {
		return types2.NONE
	}

	// 根据类名精确判断滚动类型
	switch e.GetClassname() {
	case "android.widget.ScrollView",
		"android.widget.ListView",
		"android.widget.ExpandableListView",
		"android.support.v17.leanback.widget.VerticalGridView",
		"android.support.v7.widget.RecyclerView",
		"androidx.recyclerview.widget.RecyclerView":
		return types2.Vertical
	case "android.widget.HorizontalScrollView",
		"android.support.v17.leanback.widget.HorizontalGridView",
		"android.support.v4.view.ViewPager":
		return types2.Horizontal
	}

	// 如果类名包含"ScrollView"，则支持所有方向滚动
	if strings.Contains(e.GetClassname(), "ScrollView") {
		return types2.ALL
	}

	// 默认情况下支持所有方向滚动
	return types2.ALL
}

// SetScrollType 设置滚动类型
func (e *AndroidElement) SetScrollType(ScrollType string) {
	e.eNode.CreateAttr("scrollType", ScrollType)
}

// IsWebView 判断是否为WebView
func (e *AndroidElement) IsWebView() bool {
	return strings.Contains(strings.ToLower(e.GetClassname()), "webview")
}

// IsEditText 判断是否为编辑文本
func (e *AndroidElement) GetEditable() bool {
	// 肯定被用户修改过了，优先级最高
	if val := e.eNode.SelectAttrValue("editable", ""); val != "" {
		return val == "true"
	}

	return false
}

func (e *AndroidElement) SetEditable(editable bool) {
	e.eNode.CreateAttr("editable", fmt.Sprintf("%v", editable))
}

// AddChild 添加子元素
func (e *AndroidElement) AddChild(child *AndroidElement) {
	e.Children = append(e.Children, child)
	child.Parent = e
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

// String 实现Stringer接口
func (e *AndroidElement) String() string {
	return fmt.Sprintf("AndroidElement{class:%s, text:%s, resource-id:%s, bounds:%s}",
		e.GetClassname(), e.GetText(), e.GetResourceID(), e.GetBounds().String())
}

// fromXMLNode 从XML节点创建元素
func (e *AndroidElement) fromXMLNode(node *etree.Element, parent *AndroidElement) {
	e.eNode = node

	e.path = e.eNode.GetPath()

	e.Parent = parent

	e.SetEditable(e.GetClassname() == "android.widget.EditText")

	if types2.FORCE_EDITTEXT_CLICK_TRUE && e.GetEditable() {
		e.SetLongClickable(true)
		e.SetClickable(true)
	}

	if types2.PARENT_CLICK_CHANGE_CHILDREN && e.Parent != nil && e.Parent.GetLongClickable() {
		e.SetLongClickable(true)
	}

	if types2.PARENT_CLICK_CHANGE_CHILDREN && e.Parent != nil && e.Parent.GetClickable() {
		e.SetClickable(true)
	}

	if e.GetClickable() || e.GetLongClickable() {
		e.SetEnable(true)
	}

	// 递归处理子节点
	for _, childNode := range e.eNode.ChildElements() {
		child := NewElement()
		child.fromXMLNode(childNode, e)
		e.AddChild(child)
	}
}

// parseBounds 解析边界字符串
func parseBounds(boundsStr string) *types2.Rect {
	// 边界格式通常是 "[left,top][right,bottom]"
	parts := strings.Split(boundsStr, "][")
	if len(parts) != 2 {
		return types2.NewRect(0, 0, 0, 0)
	}

	leftTop := strings.Trim(parts[0], "[]")
	rightBottom := strings.Trim(parts[1], "[]")

	lt := strings.Split(leftTop, ",")
	rb := strings.Split(rightBottom, ",")

	if len(lt) != 2 || len(rb) != 2 {
		return types2.NewRect(0, 0, 0, 0)
	}

	left, _ := strconv.Atoi(lt[0])
	top, _ := strconv.Atoi(lt[1])
	right, _ := strconv.Atoi(rb[0])
	bottom, _ := strconv.Atoi(rb[1])

	return types2.NewRect(left, top, right, bottom)
}

func (e *AndroidElement) Hash(recursive bool) uintptr {
	hashcode := uintptr(0x1)
	hashcode1 := uintptr(127) * tool.HashString(e.GetResourceID()) << 1
	hashcode2 := tool.HashString(e.GetClassname()) << 2
	hashcode3 := tool.HashString(e.GetPackageName()) << 3
	hashcode4 := uintptr(256) * tool.HashString(e.GetText()) << 4
	hashcode5 := tool.HashString(e.GetContentDesc()) << 5
	//hashcode6 := tool.HashString(e.PageName) << 2
	hashcode7 := uintptr(64) * tool.HashInt(boolToInt(e.GetClickable())) << 6

	//hashcode = hashcode1 ^ hashcode2 ^ hashcode3 ^ hashcode4 ^ hashcode5 ^ hashcode6 ^ hashcode7
	hashcode = hashcode1 ^ hashcode2 ^ hashcode3 ^ hashcode4 ^ hashcode5 ^ hashcode7

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
