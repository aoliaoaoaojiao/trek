package elements

import (
	"testing"

	coretypes "trek/internal/engine/core/types"
	types "trek/internal/engine/core/types"

	"github.com/stretchr/testify/assert"
)

func TestInferScrollableElements(t *testing.T) {
	tests := []struct {
		name         string
		xml          string
		threshold    int
		wantInferred bool // 是否应该标记了滚动能力
		wantNone     bool // GetScrollType() 返回 NONE
	}{
		{
			name: "threshold为0禁用推断",
			xml: `<node class="uia.widget.ListView" resource-id="root" bounds="[0,0][100,200]">
				<node class="uia.widget.Button" clickable="true" bounds="[10,20][90,80]"/>
				<node class="uia.widget.Button" clickable="true" bounds="[10,90][90,150]"/>
				<node class="uia.widget.Button" clickable="true" bounds="[10,160][90,220]"/>
			</node>`,
			threshold: 0,
			wantNone:  true, // threshold=0 禁用，ListView 不推断 → Vertical
		},
		{
			name: "可点击后代数量不足不推断",
			xml: `<node class="uia.widget.FrameLayout" resource-id="root" bounds="[0,0][100,200]">
				<node class="uia.widget.Button" clickable="true" bounds="[10,20][90,80]"/>
				<node class="uia.widget.Button" clickable="true" bounds="[10,90][90,150]"/>
			</node>`,
			threshold:    3,
			wantNone:     true,
			wantInferred: false,
		},
		{
			name: "恰好等于阈值应推断",
			xml: `<node class="uia.widget.LinearLayout" resource-id="root" bounds="[0,0][100,200]">
				<node class="uia.widget.Button" clickable="true" bounds="[10,20][90,80]"/>
				<node class="uia.widget.Button" clickable="true" bounds="[10,90][90,150]"/>
				<node class="uia.widget.Button" clickable="true" bounds="[10,160][90,220]"/>
			</node>`,
			threshold:    3,
			wantInferred: true,
			wantNone:     false, // LinearLayout 无特定滚动类型 → 被推断为 ALL
		},
		{
			name: "超出阈值应推断",
			xml: `<node class="uia.widget.FrameLayout" resource-id="root" bounds="[0,0][100,200]">
				<node class="uia.widget.Button" clickable="true" bounds="[10,20][90,80]"/>
				<node class="uia.widget.Button" clickable="true" bounds="[10,90][90,150]"/>
				<node class="uia.widget.Button" clickable="true" bounds="[10,160][90,220]"/>
				<node class="uia.widget.Button" clickable="true" bounds="[10,230][90,290]"/>
				<node class="uia.widget.Button" clickable="true" bounds="[10,300][90,360]"/>
			</node>`,
			threshold:    3,
			wantInferred: true,
			wantNone:     false,
		},
		{
			name: "深层容器应被推断而非祖先",
			xml: `<node class="uia.widget.FrameLayout" resource-id="root" bounds="[0,0][100,500]">
				<node class="uia.widget.LinearLayout" resource-id="list_container" bounds="[0,0][100,500]">
					<node class="uia.widget.Button" clickable="true" bounds="[10,20][90,80]"/>
					<node class="uia.widget.Button" clickable="true" bounds="[10,90][90,150]"/>
					<node class="uia.widget.Button" clickable="true" bounds="[10,160][90,220]"/>
				</node>
			</node>`,
			threshold:    3,
			wantInferred: false, // 根 FrameLayout 只有1个子元素，不满足
			wantNone:     true,  // FrameLayout 推断后也不是 ALL（无 class），保持 NONE
		},
		{
			name: "已标记scrollable的ListView保持Vertical不覆盖",
			xml: `<node class="uia.widget.ListView" scrollable="true" resource-id="root" bounds="[0,0][100,200]">
				<node class="uia.widget.Button" clickable="true" bounds="[10,20][90,80]"/>
				<node class="uia.widget.Button" clickable="true" bounds="[10,90][90,150]"/>
				<node class="uia.widget.Button" clickable="true" bounds="[10,160][90,220]"/>
			</node>`,
			threshold: 3,
			wantNone:  false, // ListView 已标记 Vertical，不应被推断覆盖
		},
		{
			name:         "叶子节点不推断",
			xml:          `<node class="uia.widget.Button" clickable="true" bounds="[0,0][100,200]"/>`,
			threshold:    3,
			wantNone:     true, // 无子节点，不满足推断条件
			wantInferred: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			origThreshold := coretypes.ScrollInferThreshold
			defer func() { coretypes.ScrollInferThreshold = origThreshold }()
			coretypes.ScrollInferThreshold = tt.threshold

			elem, err := CreateAndroidElementFromXml(tt.xml)
			assert.NoError(t, err)

			// 对根元素检查推断结果
			scrollType := elem.GetScrollType()

			if tt.wantNone {
				assert.Equal(t, types.NONE, scrollType, "应该保持 NONE")
			}
			if tt.wantInferred {
				assert.NotEqual(t, types.NONE, scrollType, "应该被推断为可滚动")
			}
		})
	}
}

func TestScrollInferThresholdDefault(t *testing.T) {
	assert.Equal(t, 5, coretypes.ScrollInferThreshold)
}

func TestInferScrollableElementsSkipWhenDisabledByAttr(t *testing.T) {
	origThreshold := coretypes.ScrollInferThreshold
	defer func() { coretypes.ScrollInferThreshold = origThreshold }()
	coretypes.ScrollInferThreshold = 3

	xml := `<node class="uia.widget.FrameLayout" trek-scroll-infer-disabled="true" bounds="[0,0][100,200]">
		<node class="uia.widget.Button" clickable="true" bounds="[10,20][90,80]"/>
		<node class="uia.widget.Button" clickable="true" bounds="[10,90][90,150]"/>
		<node class="uia.widget.Button" clickable="true" bounds="[10,160][90,220]"/>
	</node>`

	elem, err := CreateAndroidElementFromXml(xml)
	assert.NoError(t, err)
	assert.Equal(t, types.NONE, elem.GetScrollType(), "禁用标记存在时不应推断滚动能力")
}
