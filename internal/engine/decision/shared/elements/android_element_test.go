package elements

import (
	"testing"
	"trek/internal/engine/core/types"

	"github.com/stretchr/testify/assert"
)

func TestCreateAndroidElementFromXMLBasic(t *testing.T) {
	xml := `<node class="uia.widget.FrameLayout" resource-id="root" bounds="[0,0][100,200]">
		<node class="uia.widget.Button" text="Login" clickable="true" bounds="[10,20][90,80]"/>
	</node>`

	elem, err := CreateAndroidElementFromXml(xml)
	assert.NoError(t, err)
	assert.NotNil(t, elem)

	root := elem.(*AndroidElement)
	assert.Equal(t, "uia.widget.FrameLayout", root.GetClassname())
	assert.Equal(t, "root", root.GetResourceID())
	assert.Len(t, root.GetChildren(), 1)
}

func TestAndroidElementBoundsAndEditable(t *testing.T) {
	xml := `<node class="uia.widget.EditText" editable="true" bounds="[1,2][3,4]"/>`
	elem, err := CreateAndroidElementFromXml(xml)
	assert.NoError(t, err)

	ae := elem.(*AndroidElement)
	rect := ae.GetBounds()
	assert.Equal(t, float64(1), rect.Left)
	assert.Equal(t, float64(2), rect.Top)
	assert.Equal(t, float64(3), rect.Right)
	assert.Equal(t, float64(4), rect.Bottom)
	assert.True(t, ae.GetEditable())
}

func TestAndroidElementQueryAndDelete(t *testing.T) {
	xml := `<hierarchy>
	<node class="uia.widget.FrameLayout" resource-id="root">
		<node class="uia.widget.Button" resource-id="btn_ok" clickable="true"/>
		<node class="uia.widget.TextView" resource-id="txt_footer"/>
	</node>
</hierarchy>`

	elem, err := CreateAndroidElementFromXml(xml)
	assert.NoError(t, err)
	root := elem.(*AndroidElement)

	nodes := root.Query("//*[@resource-id='btn_ok']")
	assert.Len(t, nodes, 1)
	assert.Equal(t, "btn_ok", nodes[0].(*AndroidElement).GetResourceID())

	ok := root.DeleteElement("//*[@resource-id='txt_footer']")
	assert.True(t, ok)
	assert.Nil(t, root.Query("//*[@resource-id='txt_footer']"))
}

func TestAndroidElementScrollType(t *testing.T) {
	xml := `<node class="uia.widget.ListView" scrollable="true" bounds="[0,0][10,10]"/>`
	elem, err := CreateAndroidElementFromXml(xml)
	assert.NoError(t, err)
	assert.Equal(t, types.Vertical, elem.(*AndroidElement).GetScrollType())
}
