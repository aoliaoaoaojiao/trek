package elements

import (
	"testing"
	types2 "trek/internal/core/types"

	"github.com/beevik/etree"

	"github.com/stretchr/testify/assert"
)

// TestCreateAndroidElement_Basic 测试基本的创建逻辑
func TestCreateAndroidElement_Basic(t *testing.T) {
	// 1. 测试通过 Tag 创建
	elem, err := CreateAndroidElement("uia.widget.Button")

	assert.NoError(t, err)
	assert.NotNil(t, elem)

	// 这里不再 Panic，因为我们修复了 GetParent
	assert.Nil(t, elem.GetParent(), "新创建的根元素 Parent 应该为 nil")

	// 假设我们需要手动设置 class 以进行测试：
	if androidElem, ok := elem.(*AndroidElement); ok {
		androidElem.eNode.CreateAttr("class", "uia.widget.Button")
		assert.Equal(t, "uia.widget.Button", androidElem.GetClassname())
	}

}

// TestCreateAndroidElementFromXml_Complex 测试复杂的嵌套 XML 解析
func TestCreateAndroidElementFromXml_Complex(t *testing.T) {
	xmlContent := `
	<node index="0" text="" resource-id="root_id" class="uia.widget.FrameLayout" package="com.test" content-desc="" checkable="false" checked="false" clickable="false" enabled="true" focusable="false" focused="false" scrollable="false" long-clickable="false" password="false" selected="false" bounds="[0,0][1080,2400]">
		<node index="0" text="Login" resource-id="btn_login" class="uia.widget.Button" package="com.test" content-desc="login_desc" checkable="false" checked="false" clickable="true" enabled="true" focusable="true" focused="false" scrollable="false" long-clickable="false" password="false" selected="false" bounds="[100,200][300,400]" />
		<node index="1" text="Username" resource-id="input_user" class="uia.widget.EditText" package="com.test" content-desc="" checkable="false" checked="false" clickable="true" enabled="true" focusable="true" focused="false" scrollable="false" long-clickable="true" password="false" selected="false" bounds="[100,500][800,600]" />
	</node>
	`

	elem, err := CreateAndroidElementFromXml(xmlContent)
	assert.NoError(t, err)
	assert.NotNil(t, elem)

	androidElem, _ := elem.(*AndroidElement)

	// 1. 验证根节点
	assert.Equal(t, "uia.widget.FrameLayout", androidElem.GetClassname())
	assert.Equal(t, "root_id", androidElem.GetResourceID())
	assert.Equal(t, 2, len(androidElem.GetChildren()))

	rect := androidElem.GetBounds()
	assert.Equal(t, float64(0), rect.Left)
	assert.Equal(t, float64(2400), rect.Bottom)

	// 2. 验证第一个子节点 (Button)
	btn := androidElem.GetChildren()[0].(*AndroidElement)
	assert.Equal(t, "uia.widget.Button", btn.GetClassname())
	assert.Equal(t, "Login", btn.GetText())
	assert.Equal(t, "login_desc", btn.GetContentDesc())
	assert.True(t, btn.GetClickable())

	// 验证父子关系
	assert.Equal(t, elem, btn.GetParent())

	// 3. 验证第二个子节点 (EditText)
	input, _ := elem.GetChildren()[1].(*AndroidElement)
	assert.Equal(t, "uia.widget.EditText", input.GetClassname())

	assert.True(t, input.GetEditable(), "EditText 应该自动被标记为可编辑")

}

// TestAndroidElement_Attributes 测试基本的 Set/Get 属性操作
func TestAndroidElement_Attributes(t *testing.T) {
	elem := NewAndroidElement()
	// 手动初始化 eNode，因为 NewAndroidElement 没有做这个
	doc := etree.NewDocument()
	elem.eNode = doc.CreateElement("node")

	// 1. Text
	elem.SetText("Hello World")
	assert.Equal(t, "Hello World", elem.GetText())

	// 2. Clickable
	elem.SetClickable(true)
	assert.True(t, elem.GetClickable())
	elem.SetClickable(false)
	assert.False(t, elem.GetClickable())

	// 3. Scrollable
	elem.SetScrollable(true)
	assert.True(t, elem.GetScrollable())

	// 4. Enabled
	elem.SetEnable(true)
	assert.True(t, elem.GetEnable())

	// 5. Checkable
	elem.SetCheckBoxable(true)
	assert.True(t, elem.GetCheckBoxable()) // 注意：方法名是 GetCheckBoxable, 属性是 checkable
}

// TestAndroidElement_Bounds 测试坐标解析逻辑
func TestAndroidElement_Bounds(t *testing.T) {
	elem := NewAndroidElement()
	doc := etree.NewDocument()
	elem.eNode = doc.CreateElement("node")

	// 1. 测试标准坐标设置与获取
	originalRect := types2.NewRect(50, 60, 200, 300)
	elem.SetBounds(originalRect)

	parsedRect := elem.GetBounds()
	assert.Equal(t, float64(50), parsedRect.Left)
	assert.Equal(t, float64(60), parsedRect.Top)
	assert.Equal(t, float64(200), parsedRect.Right)
	assert.Equal(t, float64(300), parsedRect.Bottom)

	// 2. 测试从字符串属性直接解析 (模拟 XML 读取)
	// 覆盖 parseBounds 函数
	elem.eNode.CreateAttr("bounds", "[0,0][1080,1920]")
	parsedRect2 := elem.GetBounds()
	assert.Equal(t, float64(1080), parsedRect2.Right)
	assert.Equal(t, float64(1920), parsedRect2.Bottom)

	// 3. 测试非法格式 (健壮性测试)
	elem.eNode.CreateAttr("bounds", "invalid-format")
	zeroRect := elem.GetBounds()
	// parseBounds 遇到错误通常返回 [0,0][0,0]
	assert.Equal(t, float64(0), zeroRect.Left)
	assert.Equal(t, float64(0), zeroRect.Right)
}

// TestAndroidElement_ScrollType 测试滚动类型判定逻辑
func TestAndroidElement_ScrollType(t *testing.T) {
	tests := []struct {
		className      string
		scrollableAttr string // "true" or "false" or ""
		expectedType   types2.ScrollType
	}{
		// Case 1: 显式 Scrollable=false，应该返回 NONE
		{"uia.widget.ListView", "false", types2.NONE},

		// Case 2: 垂直列表
		{"uia.widget.ListView", "true", types2.Vertical},
		{"androidx.recyclerview.widget.RecyclerView", "true", types2.Vertical},
		{"uia.widget.ScrollView", "true", types2.Vertical},

		// Case 3: 水平列表
		{"uia.widget.HorizontalScrollView", "true", types2.Horizontal},
		{"uia.support.v4.view.ViewPager", "true", types2.Horizontal},

		// Case 4: 未知类名但包含 ScrollView -> ALL
		{"com.custom.MyScrollView", "true", types2.ALL},

		// Case 5: 普通元素 -> ALL (代码默认逻辑)
		{"uia.widget.Button", "true", types2.ALL},
	}

	for _, tt := range tests {
		t.Run(tt.className, func(t *testing.T) {
			elem := NewAndroidElement()
			doc := etree.NewDocument()
			elem.eNode = doc.CreateElement("node")

			elem.eNode.CreateAttr("class", tt.className)
			if tt.scrollableAttr != "" {
				elem.eNode.CreateAttr("scrollable", tt.scrollableAttr)
			} else {
				// 默认 createAndroidFromXmlDoc 会设置 scrollable=true，这里模拟一下
				elem.SetScrollable(true)
			}

			assert.Equal(t, tt.expectedType, elem.GetScrollType())
		})
	}
}

// TestAndroidElement_Operations 测试树形结构操作
func TestAndroidElement_Operations(t *testing.T) {
	// 构造父子结构
	parent, _ := CreateAndroidElement("parent")
	child1, _ := CreateAndroidElement("child1")
	child2, _ := CreateAndroidElement("child2")

	// 1. 测试 SetChildren / GetChildren
	children := []types2.IElement{child1, child2}
	parent.SetChildren(children)

	assert.Equal(t, 2, len(parent.GetChildren()))
	assert.Equal(t, parent, child1.GetParent())
	assert.Equal(t, parent, child2.GetParent())

	// 2. 测试 RecursiveDoElements (遍历)
	count := 0
	if p, ok := parent.(*AndroidElement); ok {
		p.RecursiveDoElements(func(e *AndroidElement) {
			count++
		})
	}
	// parent + child1 + child2 = 3
	assert.Equal(t, 3, count)
}

const testXml = `
<hierarchy rotation="0">
  <node index="0" class="uia.widget.FrameLayout" resource-id="root_id">
    <node index="0" class="uia.widget.LinearLayout">
      <node index="0" class="uia.widget.Button" text="Confirm" resource-id="btn_ok" clickable="true" />
      <node index="1" class="uia.widget.EditText" text="Initial Value" resource-id="input_field" editable="true" />
    </node>
    <node index="1" class="uia.widget.TextView" text="Footer" resource-id="footer_text" />
  </node>
</hierarchy>
`

// 测试 QueryByXPath 的全局同步修改能力
func TestAndroidElement_QueryByXPath_GlobalEffect(t *testing.T) {
	// 1. 初始化树
	root, err := CreateAndroidElementFromXml(testXml)
	assert.NoError(t, err)

	// 2. 使用 XPath 查询 EditText 节点
	xpath := "//*[@resource-id='input_field']"
	results := root.(*AndroidElement).Query(xpath)

	assert.Equal(t, 1, len(results), "应该查询到一个结果")
	target := results[0]

	// 3. 修改查询到的对象
	newText := "Modified by Test"
	target.SetText(newText)
	target.SetClickable(true)

	// 4. 验证全局生效：直接从 root 再次查询
	reQueryResult := root.(*AndroidElement).Query(xpath)
	assert.Equal(t, newText, reQueryResult[0].GetText(), "通过 root 再次获取的文本应该是修改后的")
	assert.True(t, reQueryResult[0].GetClickable(), "点击属性也应该被同步修改")

	// 5. 验证底层 XML 也被修改
	assert.Contains(t, root.String(), newText, "根节点的字符串导出中应包含修改后的文本")
}

// 测试 DeleteElement 的删除功能
func TestAndroidElement_DeleteElementByXPath(t *testing.T) {
	root, _ := CreateAndroidElementFromXml(testXml)
	androidRoot := root.(*AndroidElement)

	// 1. 删除前确认节点存在
	xpath := "//*[@resource-id='footer_text']"
	nodesBefore := androidRoot.Query(xpath)
	assert.Equal(t, 1, len(nodesBefore))

	// 2. 执行删除
	success := androidRoot.DeleteElement(xpath)
	assert.True(t, success, "删除操作应该成功")

	// 3. 验证节点已从树中消失
	nodesAfter := androidRoot.Query(xpath)
	assert.Equal(t, 0, len(nodesAfter), "删除后不应再查询到该节点")

	// 4. 验证父节点的子节点数量减少
	// 在 testXml 中，root_id 下有两个直接子 node，删掉 footer 应该剩 1 个
	assert.Equal(t, 1, len(root.GetChildren()), "根节点的直接子元素数量应减少")
}

// 测试安全防护：当 XPath 未命中时
func TestAndroidElement_Query_Safety(t *testing.T) {
	root, _ := CreateAndroidElementFromXml(testXml)
	androidRoot := root.(*AndroidElement)

	// 查询不存在的路径不应 Panic
	results := androidRoot.Query("//non_existent_tag")
	assert.Nil(t, results)

	// 删除不存在的路径不应报错
	success := androidRoot.DeleteElement("//non_existent_tag")
	assert.False(t, success)
}
