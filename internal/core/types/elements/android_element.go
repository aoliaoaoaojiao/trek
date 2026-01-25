package elements

import (
	"Trek/internal/core/model"
	types2 "Trek/internal/core/types"
	"Trek/internal/tool"
	"Trek/log"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/tidwall/gjson"
	"strconv"
	"strings"

	"github.com/beevik/etree"
)

var _ types2.IElement = (*AndroidElement)(nil)

func init() {
	model.RegisterElementCreator(string(ANDROID_ELEMENT), CreateAndroidElementFromXml)
}

func CreateAndroidElement(tag string) (types2.IElement, error) {
	doc := etree.NewDocument()
	tagElem := doc.CreateElement(tag)

	var noParent *AndroidElement = nil

	element := NewAndroidElement()
	element.fromXMLNode(tagElem, noParent)

	return element, nil

}

// CreateAndroidElementFromXml 从XML创建元素
func CreateAndroidElementFromXml(xmlContent string) (types2.IElement, error) {

	doc := etree.NewDocument()

	if err := doc.ReadFromString(xmlContent); err != nil {
		log.Errorf("parse xml error: %v", err)
		return nil, err
	}
	log.Debugf("The content of XML is: %s", xmlContent)

	elem, err := createAndroidFromXmlDoc(doc)
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

// createAndroidFromXmlDoc 从XML文档创建元素
func createAndroidFromXmlDoc(doc *etree.Document) (*AndroidElement, error) {
	root := doc.Root()
	if root == nil {
		return nil, fmt.Errorf("no root element found")
	}

	element := NewAndroidElement()

	var noParent *AndroidElement = nil

	element.fromXMLNode(root, noParent)

	element.SetScrollable(true)

	return element, nil
}

// NewAndroidElement 创建新的元素
func NewAndroidElement() *AndroidElement {
	return &AndroidElement{
		children: make([]types2.IElement, 0),
		parent:   nil,
	}
}

// AndroidElement 元素结构
type AndroidElement struct {
	children []types2.IElement
	// todo 不推荐这么写，这应该给接口类型，但是目前代码就这样了，后面有空优化
	parent types2.IElement

	path string

	eNode *etree.Element
}

func (e *AndroidElement) GetAttr(key string) interface{} {
	if e != nil && e.eNode != nil {
		if val := e.eNode.SelectAttrValue(key, ""); val != "" {
			return gjson.Parse(val).Value()
		}
	}
	return nil
}

func (e *AndroidElement) SetAttr(key string, value interface{}) {
	if e != nil && e.eNode != nil {
		data, err := json.Marshal(value)
		if err != nil {
			return
		}
		e.eNode.CreateAttr(key, string(data))
	}
}

func (e *AndroidElement) GetIdentifierHash() uintptr {
	hashcode1 := tool.HashString(e.GetClassname())
	hashcode2 := tool.HashString(e.GetResourceID())
	return hashcode1 ^ (hashcode2 << 4)
}

func (e *AndroidElement) GetPath() string {
	if e == nil || e.eNode == nil {
		return ""
	}
	return e.path
}

// GetClassname 获取类名
func (e *AndroidElement) GetClassname() string {
	if e == nil || e.eNode == nil {
		return ""
	}
	return e.eNode.SelectAttrValue("class", "attribute class name get failed")
}

// GetResourceID 获取资源ID
func (e *AndroidElement) GetResourceID() string {
	if e == nil || e.eNode == nil {
		return ""
	}
	return e.eNode.SelectAttrValue("resource-id", "attribute resource_id get failed")
}

// GetContentDesc 获取内容描述
func (e *AndroidElement) GetContentDesc() string {
	if e == nil || e.eNode == nil {
		return ""
	}
	return e.eNode.SelectAttrValue("content-desc", "attribute content description get failed")
}

// GetPackageName 获取包名
func (e *AndroidElement) GetPackageName() string {
	if e == nil || e.eNode == nil {
		return ""
	}
	return e.eNode.SelectAttrValue("package", "attribute package name get failed")
}

// GetClickable 获取是否可点击
func (e *AndroidElement) GetClickable() bool {
	if e == nil || e.eNode == nil {
		return false
	}
	if val := e.eNode.SelectAttrValue("clickable", ""); val != "" {
		return val == "true"
	}
	return false
}

func (e *AndroidElement) SetClickable(clickable bool) {
	if e == nil || e.eNode == nil {
		return
	}
	e.eNode.CreateAttr("clickable", fmt.Sprintf("%v", clickable))
}

func (e *AndroidElement) GetScrollable() bool {
	if e == nil || e.eNode == nil {
		return false
	}
	return e.eNode.SelectAttrValue("scrollable", "") == "true"
}

func (e *AndroidElement) SetScrollable(scrollable bool) {
	if e == nil || e.eNode == nil {
		return
	}
	e.eNode.CreateAttr("scrollable", fmt.Sprintf("%v", scrollable))
}

// GetLongClickable 获取是否可长按
func (e *AndroidElement) GetLongClickable() bool {
	if e == nil || e.eNode == nil {
		return false
	}
	if val := e.eNode.SelectAttrValue("long-clickable", ""); val != "" {
		return val == "true"
	}
	return false
}

func (e *AndroidElement) SetLongClickable(longClickable bool) {
	if e == nil || e.eNode == nil {
		return
	}
	e.eNode.CreateAttr("long-clickable", fmt.Sprintf("%v", longClickable))
}

// GetCheckBoxable 获取是否可勾选
func (e *AndroidElement) GetCheckBoxable() bool {
	if e == nil || e.eNode == nil {
		return false
	}
	if val := e.eNode.SelectAttrValue("checkable", ""); val != "" {
		return val == "true"
	}
	return false
}

func (e *AndroidElement) SetCheckBoxable(checked bool) {
	if e == nil || e.eNode == nil {
		return
	}
	e.eNode.CreateAttr("checkable", fmt.Sprintf("%v", checked))
}

// GetEnable 获取是否启用
func (e *AndroidElement) GetEnable() bool {
	if e == nil || e.eNode == nil {
		return false
	}
	if val := e.eNode.SelectAttrValue("enabled", ""); val != "" {
		return val == "true"
	}
	return false
}

func (e *AndroidElement) SetEnable(enable bool) {
	if e == nil || e.eNode == nil {
		return
	}
	e.eNode.CreateAttr("enabled", fmt.Sprintf("%v", enable))
}

// GetText 获取文本
func (e *AndroidElement) GetText() string {
	if e == nil || e.eNode == nil {
		return ""
	}
	if val := e.eNode.SelectAttrValue("text", "get text attribute failed"); val != "" {
		return val
	}
	return ""
}

func (e *AndroidElement) SetText(text string) {
	if e == nil || e.eNode == nil {
		return
	}
	e.eNode.CreateAttr("text", text)
}

// GetBounds 获取边界
func (e *AndroidElement) GetBounds() *types2.Rect {
	if e == nil || e.eNode == nil {
		return nil
	}
	var bounds *types2.Rect

	if val := e.eNode.SelectAttrValue("bounds", "get bounds attribute failed"); val != "" {
		bounds = parseBounds(val)
	}

	return bounds
}

func (e *AndroidElement) SetBounds(rect *types2.Rect) {
	if e == nil || e.eNode == nil {
		return
	}
	// [left,top][right,bottom]
	e.eNode.CreateAttr("bounds", fmt.Sprintf("[%d,%d][%d,%d]", rect.Left, rect.Top, rect.Right, rect.Bottom))
}

// GetChildren 获取子元素
func (e *AndroidElement) GetChildren() []types2.IElement {
	if e == nil || e.eNode == nil {
		return nil
	}
	return e.children
}

func (e *AndroidElement) SetChildren(childList []types2.IElement) {
	if e == nil || e.eNode == nil {
		return
	}
	// 只考虑为第一层赋值parent，child下的子项，应该在调用前就做好设置父节点的准备
	for _, child := range childList {
		if androidChild, ok := child.(*AndroidElement); ok {
			androidChild.parent = e
		}
	}
	e.children = childList
}

// GetParent 获取父元素
func (e *AndroidElement) GetParent() types2.IElement {
	// 1. 防护：如果 e 本身是空指针，避免 e.parent 触发 Panic
	// 2. 转换：如果 e.parent 存储的具体指针是 nil，显式返回 nil 接口
	if e == nil || e.parent == nil {
		return nil
	}

	return e.parent
}

// GetScrollType 获取滚动类型
func (e *AndroidElement) GetScrollType() types2.ScrollType {
	if e == nil || e.eNode == nil {
		return types2.NONE
	}
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
	if e == nil || e.eNode == nil {
		return
	}
	e.eNode.CreateAttr("scrollType", ScrollType)
}

// IsWebView 判断是否为WebView
func (e *AndroidElement) IsWebView() bool {
	return strings.Contains(strings.ToLower(e.GetClassname()), "webview")
}

// IsEditText 判断是否为编辑文本
func (e *AndroidElement) GetEditable() bool {
	if e == nil || e.eNode == nil {
		return false
	}
	// 肯定被用户修改过了，优先级最高
	if val := e.eNode.SelectAttrValue("editable", ""); val != "" {
		return val == "true"
	}

	return false
}

func (e *AndroidElement) SetEditable(editable bool) {
	if e == nil || e.eNode == nil {
		return
	}
	e.eNode.CreateAttr("editable", fmt.Sprintf("%v", editable))
}

// addChild 添加子元素
func (e *AndroidElement) addChild(child *AndroidElement) {
	e.children = append(e.children, child)
	child.parent = e
}

//// DeleteElement 删除元素
//func (e *AndroidElement) DeleteElement() {
//	if e.GetParent() != nil {
//		for i, child := range e.GetParent().GetChildren() {
//			if child == e {
//				childList := append(e.parent.children[:i], e.parent.children[i+1:]...)
//				e.GetParent().SetChildren(childList)
//				break
//			}
//		}
//	} else {
//		log.Errorf("element is a root elements")
//	}
//}

// RecursiveDoElements 递归执行操作
// RecursiveDoElements 递归执行操作
func (e *AndroidElement) RecursiveDoElements(doFunc func(*AndroidElement)) {
	// 修复点：自身判空
	if e == nil || doFunc == nil {
		return
	}

	doFunc(e)
	for _, child := range e.children {
		if androidChild, ok := child.(*AndroidElement); ok {
			// 修复点：显式检查子节点指针
			if androidChild != nil {
				androidChild.RecursiveDoElements(doFunc)
			}
		}
	}
}

// String 实现Stringer接口
func (e *AndroidElement) String() string {
	if e != nil && e.eNode != nil {
		var buf bytes.Buffer
		e.eNode.WriteTo(&buf, &etree.WriteSettings{})
		return buf.String()
	}
	//return fmt.Sprintf("AndroidElement{class:%s, text:%s, resource-id:%s, bounds:%s}",
	//	e.GetClassname(), e.GetText(), e.GetResourceID(), e.GetBounds().String())
	return ""
}

// fromXMLNode 从XML节点创建元素
func (e *AndroidElement) fromXMLNode(node *etree.Element, parent types2.IElement) {

	if node == nil {
		return
	}
	e.eNode = node
	e.path = e.eNode.GetPath()

	if p, isAndroid := parent.(*AndroidElement); isAndroid {
		e.parent = p
	} else {
		e.parent = nil
	}

	e.SetEditable(e.GetClassname() == "android.widget.EditText")

	if types2.FORCE_EDITTEXT_CLICK_TRUE && e.GetEditable() {
		e.SetLongClickable(true)
		e.SetClickable(true)
	}

	if types2.PARENT_CLICK_CHANGE_CHILDREN && e.GetParent() != nil && e.GetParent().GetLongClickable() {
		e.SetLongClickable(true)
	}

	if types2.PARENT_CLICK_CHANGE_CHILDREN && e.GetParent() != nil && e.GetParent().GetClickable() {
		e.SetClickable(true)
	}

	if e.GetClickable() || e.GetLongClickable() {
		e.SetEnable(true)
	}

	// 递归处理子节点
	for _, childNode := range e.eNode.ChildElements() {
		child := NewAndroidElement()
		child.fromXMLNode(childNode, e)
		e.addChild(child)
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

// Query 根据 XPath 表达式查询元素
// 返回 []*AndroidElement，对这些对象的修改会直接同步到原始树中
func (e *AndroidElement) Query(xpath string) []types2.IElement {
	if e == nil || e.eNode == nil || xpath == "" {
		return nil
	}

	// 1. 获取所有匹配的底层 XML 节点
	matchedNodes := e.eNode.FindElements(xpath)
	if len(matchedNodes) == 0 {
		return nil
	}

	// 2. 为了高效比对，将匹配到的 XML 节点指针存入 map
	targetMap := make(map[*etree.Element]bool)
	for _, node := range matchedNodes {
		targetMap[node] = true
	}

	// 3. 递归寻找持有这些 eNode 的 AndroidElement 包装对象
	var results []types2.IElement
	e.findWrappersRecursive(targetMap, &results)

	return results
}

// findWrappersRecursive 内部递归查找匹配包装器的辅助函数
func (e *AndroidElement) findWrappersRecursive(targetMap map[*etree.Element]bool, results *[]types2.IElement) {
	if e == nil {
		return
	}

	// 如果当前节点的 eNode 在匹配名单中，将其加入结果集
	if targetMap[e.eNode] {
		*results = append(*results, e)
	}

	// 继续向下递归
	for _, child := range e.children {
		if androidChild, ok := child.(*AndroidElement); ok && androidChild != nil {
			androidChild.findWrappersRecursive(targetMap, results)
		}
	}
}

// DeleteElement 根据用户输入的 XPath 表达式删除匹配的所有元素
func (e *AndroidElement) DeleteElement(xpath string) bool {
	if e == nil || e.eNode == nil || xpath == "" {
		return false
	}

	// 1. 利用 etree 的 XPath 引擎找到所有匹配的底层 XML 节点
	matchedNodes := e.eNode.FindElements(xpath)
	if len(matchedNodes) == 0 {
		return false
	}

	// 2. 将匹配到的节点指针存入 map，方便后续 O(1) 效率的比对
	targetMap := make(map[*etree.Element]bool)
	for _, node := range matchedNodes {
		targetMap[node] = true
	}

	// 3. 调用递归辅助函数进行删除
	return e.deleteRecursiveByMatchedNodes(targetMap)
}

// deleteRecursiveByMatchedNodes 递归辅助函数
func (e *AndroidElement) deleteRecursiveByMatchedNodes(targetMap map[*etree.Element]bool) bool {
	hasDeleted := false

	// 注意：删除切片元素时，必须逆序遍历，否则索引会发生错乱
	for i := len(e.children) - 1; i >= 0; i-- {
		child, ok := e.children[i].(*AndroidElement)
		if !ok || child == nil {
			continue
		}

		// 如果当前子节点的底层 eNode 在匹配名单中
		if targetMap[child.eNode] {
			// A. 从底层的 XML 树中删除节点
			if child.eNode != nil {
				e.eNode.RemoveChild(child.eNode)
			}

			// B. 从当前包装对象的 children 切片中删除
			e.children = append(e.children[:i], e.children[i+1:]...)

			// C. 清空父引用
			child.parent = nil

			hasDeleted = true
			continue // 继续检查同级的其他子节点
		}

		// 如果当前子节点没命中，则递归进入子节点的子节点寻找
		if child.deleteRecursiveByMatchedNodes(targetMap) {
			hasDeleted = true
		}
	}

	return hasDeleted
}
