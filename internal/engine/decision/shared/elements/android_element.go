package elements

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"trek/internal/engine/decision/shared/tool"
	types2 "trek/internal/engine/decision/shared/types"
	"trek/logger"

	"github.com/tidwall/gjson"

	"github.com/beevik/etree"
)

var _ types2.IElement = (*AndroidElement)(nil)

func CreateAndroidElement(tag string) (types2.IElement, error) {
	doc := etree.NewDocument()
	tagElem := doc.CreateElement(tag)

	var noParent *AndroidElement = nil

	element := NewAndroidElement()
	element.fromXMLNode(tagElem, noParent)

	return element, nil

}

func CreateAndroidElementFromXml(xmlContent string) (types2.IElement, error) {

	doc := etree.NewDocument()

	if err := doc.ReadFromString(xmlContent); err != nil {
		logger.Errorf("parse xml error: %v", err)
		return nil, err
	}
	logger.Debugf("The content of XML is: %s", xmlContent)

	elem, err := createAndroidFromXmlDoc(doc)
	if err != nil {
		return nil, err
	}

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

func NewAndroidElement() *AndroidElement {
	return &AndroidElement{
		children: make([]types2.IElement, 0),
		parent:   nil,
	}
}

type AndroidElement struct {
	children []types2.IElement
	parent   types2.IElement

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

func (e *AndroidElement) GetClassname() string {
	if e == nil || e.eNode == nil {
		return ""
	}
	return e.eNode.SelectAttrValue("class", "attribute class name get failed")
}

func (e *AndroidElement) GetResourceID() string {
	if e == nil || e.eNode == nil {
		return ""
	}
	return e.eNode.SelectAttrValue("resource-id", "attribute resource_id get failed")
}

func (e *AndroidElement) GetContentDesc() string {
	if e == nil || e.eNode == nil {
		return ""
	}
	return e.eNode.SelectAttrValue("content-desc", "attribute content description get failed")
}

func (e *AndroidElement) GetPackageName() string {
	if e == nil || e.eNode == nil {
		return ""
	}
	return e.eNode.SelectAttrValue("package", "attribute package name get failed")
}

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
	e.eNode.CreateAttr("bounds", fmt.Sprintf("[%.0f,%.0f][%.0f,%.0f]", rect.Left, rect.Top, rect.Right, rect.Bottom))
}

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
	for _, child := range childList {
		if androidChild, ok := child.(*AndroidElement); ok {
			androidChild.parent = e
		}
	}
	e.children = childList
}

func (e *AndroidElement) GetParent() types2.IElement {
	if e == nil || e.parent == nil {
		return nil
	}

	return e.parent
}

func (e *AndroidElement) GetScrollType() types2.ScrollType {
	if e == nil || e.eNode == nil {
		return types2.NONE
	}
	if val := e.eNode.SelectAttrValue("scrollType", ""); val != "" {
		return types2.StringToScrollType(val)
	}

	if !e.GetScrollable() {
		return types2.NONE
	}

	switch e.GetClassname() {
	case "uia.widget.ScrollView",
		"uia.widget.ListView",
		"uia.widget.ExpandableListView",
		"uia.support.v17.leanback.widget.VerticalGridView",
		"uia.support.v7.widget.RecyclerView",
		"androidx.recyclerview.widget.RecyclerView":
		return types2.Vertical
	case "uia.widget.HorizontalScrollView",
		"uia.support.v17.leanback.widget.HorizontalGridView",
		"uia.support.v4.view.ViewPager":
		return types2.Horizontal
	}

	if strings.Contains(e.GetClassname(), "ScrollView") {
		return types2.ALL
	}

	return types2.ALL
}

func (e *AndroidElement) SetScrollType(ScrollType string) {
	if e == nil || e.eNode == nil {
		return
	}
	e.eNode.CreateAttr("scrollType", ScrollType)
}

func (e *AndroidElement) IsWebView() bool {
	return strings.Contains(strings.ToLower(e.GetClassname()), "webview")
}

func (e *AndroidElement) GetEditable() bool {
	if e == nil || e.eNode == nil {
		return false
	}
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

func (e *AndroidElement) addChild(child *AndroidElement) {
	e.children = append(e.children, child)
	child.parent = e
}

func (e *AndroidElement) RecursiveDoElements(doFunc func(*AndroidElement)) {
	if e == nil || doFunc == nil {
		return
	}

	doFunc(e)
	for _, child := range e.children {
		if androidChild, ok := child.(*AndroidElement); ok {
			if androidChild != nil {
				androidChild.RecursiveDoElements(doFunc)
			}
		}
	}
}

func (e *AndroidElement) String() string {
	if e != nil && e.eNode != nil {
		var buf bytes.Buffer
		e.eNode.WriteTo(&buf, &etree.WriteSettings{})
		return buf.String()
	}
	return ""
}

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

	e.SetEditable(e.GetClassname() == "uia.widget.EditText")

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

	for _, childNode := range e.eNode.ChildElements() {
		child := NewAndroidElement()
		child.fromXMLNode(childNode, e)
		e.addChild(child)
	}
}

func parseBounds(boundsStr string) *types2.Rect {
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

	left, _ := strconv.ParseFloat(lt[0], 64)
	top, _ := strconv.ParseFloat(lt[1], 64)
	right, _ := strconv.ParseFloat(rb[0], 64)
	bottom, _ := strconv.ParseFloat(rb[1], 64)

	return types2.NewRect(left, top, right, bottom)
}

func (e *AndroidElement) Query(xpath string) []types2.IElement {
	if e == nil || e.eNode == nil || xpath == "" {
		return nil
	}

	matchedNodes := e.eNode.FindElements(xpath)
	if len(matchedNodes) == 0 {
		return nil
	}

	targetMap := make(map[*etree.Element]bool)
	for _, node := range matchedNodes {
		targetMap[node] = true
	}

	var results []types2.IElement
	e.findWrappersRecursive(targetMap, &results)

	return results
}

func (e *AndroidElement) findWrappersRecursive(targetMap map[*etree.Element]bool, results *[]types2.IElement) {
	if e == nil {
		return
	}

	if targetMap[e.eNode] {
		*results = append(*results, e)
	}

	for _, child := range e.children {
		if androidChild, ok := child.(*AndroidElement); ok && androidChild != nil {
			androidChild.findWrappersRecursive(targetMap, results)
		}
	}
}

func (e *AndroidElement) DeleteElement(xpath string) bool {
	if e == nil || e.eNode == nil || xpath == "" {
		return false
	}

	matchedNodes := e.eNode.FindElements(xpath)
	if len(matchedNodes) == 0 {
		return false
	}

	targetMap := make(map[*etree.Element]bool)
	for _, node := range matchedNodes {
		targetMap[node] = true
	}

	return e.deleteRecursiveByMatchedNodes(targetMap)
}

func (e *AndroidElement) deleteRecursiveByMatchedNodes(targetMap map[*etree.Element]bool) bool {
	hasDeleted := false

	for i := len(e.children) - 1; i >= 0; i-- {
		child, ok := e.children[i].(*AndroidElement)
		if !ok || child == nil {
			continue
		}

		if targetMap[child.eNode] {
			if child.eNode != nil {
				e.eNode.RemoveChild(child.eNode)
			}

			e.children = append(e.children[:i], e.children[i+1:]...)

			child.parent = nil

			hasDeleted = true
			continue
		}

		if child.deleteRecursiveByMatchedNodes(targetMap) {
			hasDeleted = true
		}
	}

	return hasDeleted
}
