package elements

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"trek/internal/engine/core/model"
	"trek/internal/engine/core/tool"
	"trek/internal/engine/core/types"
	"trek/logger"

	"github.com/tidwall/gjson"

	"github.com/beevik/etree"
)

var _ types.IElement = (*AndroidElement)(nil)

func init() {
	model.RegisterElementCreator(string(ANDROID_ELEMENT), CreateAndroidElementFromXml)
}

func CreateAndroidElement(tag string) (types.IElement, error) {
	doc := etree.NewDocument()
	tagElem := doc.CreateElement(tag)

	var noParent *AndroidElement = nil

	element := NewAndroidElement()
	element.fromXMLNode(tagElem, noParent)

	return element, nil

}

// CreateAndroidElementFromXml 浠嶺ML鍒涘缓鍏冪礌
func CreateAndroidElementFromXml(xmlContent string) (types.IElement, error) {

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

	// 濡傛灉鏁翠釜鐣岄潰锛屾病鏈変换浣曚竴涓彲鐐瑰嚮椤癸紝鍒欐墍鏈夌殑鍏冪礌閮借缃垚鍙偣鍑婚」
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

// createAndroidFromXmlDoc 浠嶺ML鏂囨。鍒涘缓鍏冪礌
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

// NewAndroidElement 鍒涘缓鏂扮殑鍏冪礌
func NewAndroidElement() *AndroidElement {
	return &AndroidElement{
		children: make([]types.IElement, 0),
		parent:   nil,
	}
}

// AndroidElement 鍏冪礌缁撴瀯
type AndroidElement struct {
	children []types.IElement
	// todo 涓嶆帹鑽愯繖涔堝啓锛岃繖搴旇缁欐帴鍙ｇ被鍨嬶紝浣嗘槸鐩墠浠ｇ爜灏辫繖鏍蜂簡锛屽悗闈㈡湁绌轰紭鍖?
	parent types.IElement

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

// GetClassname 鑾峰彇绫诲悕
func (e *AndroidElement) GetClassname() string {
	if e == nil || e.eNode == nil {
		return ""
	}
	return e.eNode.SelectAttrValue("class", "attribute class name get failed")
}

// GetResourceID 鑾峰彇璧勬簮ID
func (e *AndroidElement) GetResourceID() string {
	if e == nil || e.eNode == nil {
		return ""
	}
	return e.eNode.SelectAttrValue("resource-id", "attribute resource_id get failed")
}

// GetContentDesc 鑾峰彇鍐呭鎻忚堪
func (e *AndroidElement) GetContentDesc() string {
	if e == nil || e.eNode == nil {
		return ""
	}
	return e.eNode.SelectAttrValue("content-desc", "attribute content description get failed")
}

// GetPackageName 鑾峰彇鍖呭悕
func (e *AndroidElement) GetPackageName() string {
	if e == nil || e.eNode == nil {
		return ""
	}
	return e.eNode.SelectAttrValue("package", "attribute package name get failed")
}

// GetClickable 鑾峰彇鏄惁鍙偣鍑?

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

// GetLongClickable 鑾峰彇鏄惁鍙暱鎸?

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

// GetCheckBoxable 鑾峰彇鏄惁鍙嬀閫?

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

// GetEnable 鑾峰彇鏄惁鍚敤
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

// GetText 鑾峰彇鏂囨湰
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

// GetBounds 鑾峰彇杈圭晫
func (e *AndroidElement) GetBounds() *types.Rect {
	if e == nil || e.eNode == nil {
		return nil
	}
	var bounds *types.Rect

	if val := e.eNode.SelectAttrValue("bounds", "get bounds attribute failed"); val != "" {
		bounds = parseBounds(val)
	}

	return bounds
}

func (e *AndroidElement) SetBounds(rect *types.Rect) {
	if e == nil || e.eNode == nil {
		return
	}
	// [left,top][right,bottom]
	e.eNode.CreateAttr("bounds", fmt.Sprintf("[%.0f,%.0f][%.0f,%.0f]", rect.Left, rect.Top, rect.Right, rect.Bottom))
}

// GetChildren 鑾峰彇瀛愬厓绱?

func (e *AndroidElement) GetChildren() []types.IElement {
	if e == nil || e.eNode == nil {
		return nil
	}
	return e.children
}

func (e *AndroidElement) SetChildren(childList []types.IElement) {
	if e == nil || e.eNode == nil {
		return
	}
	// 鍙€冭檻涓虹涓€灞傝祴鍊紁arent锛宑hild涓嬬殑瀛愰」锛屽簲璇ュ湪璋冪敤鍓嶅氨鍋氬ソ璁剧疆鐖惰妭鐐圭殑鍑嗗
	for _, child := range childList {
		if androidChild, ok := child.(*AndroidElement); ok {
			androidChild.parent = e
		}
	}
	e.children = childList
}

// GetParent 鑾峰彇鐖跺厓绱?

func (e *AndroidElement) GetParent() types.IElement {
	// 1. 闃叉姢锛氬鏋?e 鏈韩鏄┖鎸囬拡锛岄伩鍏?e.parent 瑙﹀彂 Panic
	// 2. 杞崲锛氬鏋?e.parent 瀛樺偍鐨勫叿浣撴寚閽堟槸 nil锛屾樉寮忚繑鍥?nil 鎺ュ彛
	if e == nil || e.parent == nil {
		return nil
	}

	return e.parent
}

// GetScrollType 鑾峰彇婊氬姩绫诲瀷
func (e *AndroidElement) GetScrollType() types.ScrollType {
	if e == nil || e.eNode == nil {
		return types.NONE
	}
	// 濡傛灉鏈夊€硷紝鑲畾鏄鐢ㄦ埛鑷繁淇敼杩囦簡锛屾墍浠ヤ紭鍏堢骇鏈€楂?
	if val := e.eNode.SelectAttrValue("scrollType", ""); val != "" {
		return types.StringToScrollType(val)
	}

	if !e.GetScrollable() {
		return types.NONE
	}

	// 鏍规嵁绫诲悕绮剧‘鍒ゆ柇婊氬姩绫诲瀷
	switch e.GetClassname() {
	case "uia.widget.ScrollView",
		"uia.widget.ListView",
		"uia.widget.ExpandableListView",
		"uia.support.v17.leanback.widget.VerticalGridView",
		"uia.support.v7.widget.RecyclerView",
		"androidx.recyclerview.widget.RecyclerView":
		return types.Vertical
	case "uia.widget.HorizontalScrollView",
		"uia.support.v17.leanback.widget.HorizontalGridView",
		"uia.support.v4.view.ViewPager":
		return types.Horizontal
	}

	// 濡傛灉绫诲悕鍖呭惈"ScrollView"锛屽垯鏀寔鎵€鏈夋柟鍚戞粴鍔?
	if strings.Contains(e.GetClassname(), "ScrollView") {
		return types.ALL
	}

	// 榛樿鎯呭喌涓嬫敮鎸佹墍鏈夋柟鍚戞粴鍔?
	return types.ALL
}

// SetScrollType 璁剧疆婊氬姩绫诲瀷
func (e *AndroidElement) SetScrollType(ScrollType string) {
	if e == nil || e.eNode == nil {
		return
	}
	e.eNode.CreateAttr("scrollType", ScrollType)
}

// IsWebView 鍒ゆ柇鏄惁涓篧ebView
func (e *AndroidElement) IsWebView() bool {
	return strings.Contains(strings.ToLower(e.GetClassname()), "webview")
}

// IsEditText 鍒ゆ柇鏄惁涓虹紪杈戞枃鏈?

func (e *AndroidElement) GetEditable() bool {
	if e == nil || e.eNode == nil {
		return false
	}
	// 鑲畾琚敤鎴蜂慨鏀硅繃浜嗭紝浼樺厛绾ф渶楂?
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

// addChild 娣诲姞瀛愬厓绱?

func (e *AndroidElement) addChild(child *AndroidElement) {
	e.children = append(e.children, child)
	child.parent = e
}

// RecursiveDoElements 閫掑綊鎵ц鎿嶄綔
// RecursiveDoElements 閫掑綊鎵ц鎿嶄綔
func (e *AndroidElement) RecursiveDoElements(doFunc func(*AndroidElement)) {
	// 淇鐐癸細鑷韩鍒ょ┖
	if e == nil || doFunc == nil {
		return
	}

	doFunc(e)
	for _, child := range e.children {
		if androidChild, ok := child.(*AndroidElement); ok {
			// 淇鐐癸細鏄惧紡妫€鏌ュ瓙鑺傜偣鎸囬拡
			if androidChild != nil {
				androidChild.RecursiveDoElements(doFunc)
			}
		}
	}
}

// String 瀹炵幇Stringer鎺ュ彛
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

// fromXMLNode 浠嶺ML鑺傜偣鍒涘缓鍏冪礌
func (e *AndroidElement) fromXMLNode(node *etree.Element, parent types.IElement) {

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

	if types.FORCE_EDITTEXT_CLICK_TRUE && e.GetEditable() {
		e.SetLongClickable(true)
		e.SetClickable(true)
	}

	if types.PARENT_CLICK_CHANGE_CHILDREN && e.GetParent() != nil && e.GetParent().GetLongClickable() {
		e.SetLongClickable(true)
	}

	if types.PARENT_CLICK_CHANGE_CHILDREN && e.GetParent() != nil && e.GetParent().GetClickable() {
		e.SetClickable(true)
	}

	if e.GetClickable() || e.GetLongClickable() {
		e.SetEnable(true)
	}

	// 閫掑綊澶勭悊瀛愯妭鐐?
	for _, childNode := range e.eNode.ChildElements() {
		child := NewAndroidElement()
		child.fromXMLNode(childNode, e)
		e.addChild(child)
	}
}

// parseBounds 瑙ｆ瀽杈圭晫瀛楃涓?

func parseBounds(boundsStr string) *types.Rect {
	// 杈圭晫鏍煎紡閫氬父鏄?"[left,top][right,bottom]"
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

	left, _ := strconv.ParseFloat(lt[0], 64)
	top, _ := strconv.ParseFloat(lt[1], 64)
	right, _ := strconv.ParseFloat(rb[0], 64)
	bottom, _ := strconv.ParseFloat(rb[1], 64)

	return types.NewRect(left, top, right, bottom)
}

// Query 鏍规嵁 XPath 琛ㄨ揪寮忔煡璇㈠厓绱?// 杩斿洖 []*AndroidElement锛屽杩欎簺瀵硅薄鐨勪慨鏀逛細鐩存帴鍚屾鍒板師濮嬫爲涓?

func (e *AndroidElement) Query(xpath string) []types.IElement {
	if e == nil || e.eNode == nil || xpath == "" {
		return nil
	}

	// 1. 鑾峰彇鎵€鏈夊尮閰嶇殑搴曞眰 XML 鑺傜偣
	matchedNodes := e.eNode.FindElements(xpath)
	if len(matchedNodes) == 0 {
		return nil
	}

	// 2. 涓轰簡楂樻晥姣斿锛屽皢鍖归厤鍒扮殑 XML 鑺傜偣鎸囬拡瀛樺叆 map
	targetMap := make(map[*etree.Element]bool)
	for _, node := range matchedNodes {
		targetMap[node] = true
	}

	// 3. 閫掑綊瀵绘壘鎸佹湁杩欎簺 eNode 鐨?AndroidElement 鍖呰瀵硅薄
	var results []types.IElement
	e.findWrappersRecursive(targetMap, &results)

	return results
}

// findWrappersRecursive 鍐呴儴閫掑綊鏌ユ壘鍖归厤鍖呰鍣ㄧ殑杈呭姪鍑芥暟
func (e *AndroidElement) findWrappersRecursive(targetMap map[*etree.Element]bool, results *[]types.IElement) {
	if e == nil {
		return
	}

	// 濡傛灉褰撳墠鑺傜偣鐨?eNode 鍦ㄥ尮閰嶅悕鍗曚腑锛屽皢鍏跺姞鍏ョ粨鏋滈泦
	if targetMap[e.eNode] {
		*results = append(*results, e)
	}

	// 缁х画鍚戜笅閫掑綊
	for _, child := range e.children {
		if androidChild, ok := child.(*AndroidElement); ok && androidChild != nil {
			androidChild.findWrappersRecursive(targetMap, results)
		}
	}
}

// DeleteElement 鏍规嵁鐢ㄦ埛杈撳叆鐨?XPath 琛ㄨ揪寮忓垹闄ゅ尮閰嶇殑鎵€鏈夊厓绱?

func (e *AndroidElement) DeleteElement(xpath string) bool {
	if e == nil || e.eNode == nil || xpath == "" {
		return false
	}

	// 1. 鍒╃敤 etree 鐨?XPath 寮曟搸鎵惧埌鎵€鏈夊尮閰嶇殑搴曞眰 XML 鑺傜偣
	matchedNodes := e.eNode.FindElements(xpath)
	if len(matchedNodes) == 0 {
		return false
	}

	// 2. 灏嗗尮閰嶅埌鐨勮妭鐐规寚閽堝瓨鍏?map锛屾柟渚垮悗缁?O(1) 鏁堢巼鐨勬瘮瀵?
	targetMap := make(map[*etree.Element]bool)
	for _, node := range matchedNodes {
		targetMap[node] = true
	}

	// 3. 璋冪敤閫掑綊杈呭姪鍑芥暟杩涜鍒犻櫎
	return e.deleteRecursiveByMatchedNodes(targetMap)
}

// deleteRecursiveByMatchedNodes 閫掑綊杈呭姪鍑芥暟
func (e *AndroidElement) deleteRecursiveByMatchedNodes(targetMap map[*etree.Element]bool) bool {
	hasDeleted := false

	// 娉ㄦ剰锛氬垹闄ゅ垏鐗囧厓绱犳椂锛屽繀椤婚€嗗簭閬嶅巻锛屽惁鍒欑储寮曚細鍙戠敓閿欎贡
	for i := len(e.children) - 1; i >= 0; i-- {
		child, ok := e.children[i].(*AndroidElement)
		if !ok || child == nil {
			continue
		}

		// 濡傛灉褰撳墠瀛愯妭鐐圭殑搴曞眰 eNode 鍦ㄥ尮閰嶅悕鍗曚腑
		if targetMap[child.eNode] {
			// A. 浠庡簳灞傜殑 XML 鏍戜腑鍒犻櫎鑺傜偣
			if child.eNode != nil {
				e.eNode.RemoveChild(child.eNode)
			}

			// B. 浠庡綋鍓嶅寘瑁呭璞＄殑 children 鍒囩墖涓垹闄?
			e.children = append(e.children[:i], e.children[i+1:]...)

			// C. 娓呯┖鐖跺紩鐢?
			child.parent = nil

			hasDeleted = true
			continue // 缁х画妫€鏌ュ悓绾х殑鍏朵粬瀛愯妭鐐?
		}

		// 濡傛灉褰撳墠瀛愯妭鐐规病鍛戒腑锛屽垯閫掑綊杩涘叆瀛愯妭鐐圭殑瀛愯妭鐐瑰鎵?
		if child.deleteRecursiveByMatchedNodes(targetMap) {
			hasDeleted = true
		}
	}

	return hasDeleted
}
