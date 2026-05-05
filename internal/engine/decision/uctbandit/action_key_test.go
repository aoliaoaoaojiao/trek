package uctbandit

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"trek/internal/engine/core/types"
)

func TestBuildActionKeyClickWithResourceID(t *testing.T) {
	key := BuildActionKey(types.CLICK, "android.widget.Button", "com.example:id/login_btn", "登录", 0.3, 0.5)
	assert.Contains(t, key, "CLICK")
	assert.Contains(t, key, "android.widget.Button")
	assert.Contains(t, key, "com.example:id/login_btn")
	assert.Contains(t, key, "short_text")
}

func TestBuildActionKeyClickNoResourceID(t *testing.T) {
	key := BuildActionKey(types.CLICK, "android.widget.TextView", "", "这是一段超长的文本内容用于测试长文本模式归类", 0.5, 0.7)
	assert.Contains(t, key, "CLICK")
	assert.Contains(t, key, "android.widget.TextView")
	assert.Contains(t, key, "long_text")
}

func TestBuildActionKeyBack(t *testing.T) {
	key := BuildActionKey(types.BACK, "", "", "", 0, 0)
	assert.Contains(t, key, "BACK")
}

func TestBuildActionKeyStability(t *testing.T) {
	// 相同输入应产生相同 key
	key1 := BuildActionKey(types.CLICK, "android.widget.Button", "id/btn", "确定", 0.5, 0.5)
	key2 := BuildActionKey(types.CLICK, "android.widget.Button", "id/btn", "确定", 0.5, 0.5)
	assert.Equal(t, key1, key2)
}

func TestBuildActionKeyDynamicText(t *testing.T) {
	// 动态文案不应导致 key 完全不同（关键靠 resourceID 和 positionBucket）
	key1 := BuildActionKey(types.CLICK, "android.widget.TextView", "id/item_title", "商品A", 0.3, 0.5)
	key2 := BuildActionKey(types.CLICK, "android.widget.TextView", "id/item_title", "商品B", 0.3, 0.5)
	// resourceID 相同，positionBucket 相同 → key 应相同（textPattern 归一化后）
	assert.Equal(t, key1, key2, "dynamic text with same resID and position should produce same key")
}

func TestBuildActionKeyDifferentPosition(t *testing.T) {
	key1 := BuildActionKey(types.CLICK, "android.widget.Button", "id/btn", "OK", 0.1, 0.1)
	key2 := BuildActionKey(types.CLICK, "android.widget.Button", "id/btn", "OK", 0.9, 0.9)
	// 不同位置应产生不同 key
	assert.NotEqual(t, key1, key2, "different positions should produce different keys")
}

func TestClassifyTextPattern(t *testing.T) {
	assert.Equal(t, "empty", classifyTextPattern(""))
	assert.Equal(t, "short_text", classifyTextPattern("hi"))
	assert.Equal(t, "long_text", classifyTextPattern("这是一段很长的文本内容用于测试长文本模式"))
	assert.Equal(t, "number_like", classifyTextPattern("12345"))
	assert.Equal(t, "number_like", classifyTextPattern("3.14"))
	assert.Equal(t, "more_like", classifyTextPattern("更多"))
	assert.Equal(t, "detail_like", classifyTextPattern("查看详情"))
}

func TestClassifyPosition(t *testing.T) {
	// 中心区域
	assert.Equal(t, "middle_center", classifyPosition(0.5, 0.5))
	// 左上角
	assert.Equal(t, "top_left", classifyPosition(0.1, 0.1))
	// 右下角
	assert.Equal(t, "bottom_right", classifyPosition(0.9, 0.9))
	// 上中间
	assert.Equal(t, "top_center", classifyPosition(0.5, 0.1))
}

func TestBuildArmKey(t *testing.T) {
	armKey := BuildArmKey("Home|many_clickables|list|no_input|title_short", "android.widget.TextView", types.CLICK, "middle")
	assert.Contains(t, armKey, "Home")
	assert.Contains(t, armKey, "android.widget.TextView")
	assert.Contains(t, armKey, "CLICK")
	assert.Contains(t, armKey, "middle")
}

func TestBuildPageCluster(t *testing.T) {
	cluster := BuildPageCluster("Home", 15, true, false, "首页")
	assert.Contains(t, cluster, "Home")
	assert.Contains(t, cluster, "many_clickables")
	assert.Contains(t, cluster, "list")
	assert.Contains(t, cluster, "no_input")
}

func TestBuildPageClusterFewClickables(t *testing.T) {
	cluster := BuildPageCluster("Detail", 3, false, false, "商品详情")
	assert.Contains(t, cluster, "few_clickables")
	assert.Contains(t, cluster, "no_list")
}

func TestBuildPageClusterWithInput(t *testing.T) {
	cluster := BuildPageCluster("Search", 8, false, true, "搜索")
	assert.Contains(t, cluster, "has_input")
}

func TestClassifyClickableCount(t *testing.T) {
	assert.Equal(t, "few_clickables", classifyClickableCount(3))
	assert.Equal(t, "few_clickables", classifyClickableCount(5))
	assert.Equal(t, "many_clickables", classifyClickableCount(6))
	assert.Equal(t, "many_clickables", classifyClickableCount(20))
}

func TestClassifyTitle(t *testing.T) {
	assert.Equal(t, "title_short", classifyTitle("OK"))
	assert.Equal(t, "title_long", classifyTitle("这是一个非常长的页面标题超过了限制长度"))
	assert.Equal(t, "title_empty", classifyTitle(""))
}

func TestPageClusterConsistency(t *testing.T) {
	// 相似页面应产生相同聚类
	cluster1 := BuildPageCluster("Home", 12, true, false, "首页")
	cluster2 := BuildPageCluster("Home", 11, true, false, "首页")
	assert.Equal(t, cluster1, cluster2, "similar page should produce same cluster")
}

func TestDifferentPagesDifferentClusters(t *testing.T) {
	home := BuildPageCluster("Home", 15, true, false, "")
	detail := BuildPageCluster("Detail", 3, false, false, "详情")
	assert.NotEqual(t, home, detail)
}

func TestArmKeyStability(t *testing.T) {
	cluster := BuildPageCluster("Home", 15, true, false, "首页")
	arm1 := BuildArmKey(cluster, "android.widget.Button", types.CLICK, "middle_left")
	arm2 := BuildArmKey(cluster, "android.widget.Button", types.CLICK, "middle_left")
	assert.Equal(t, arm1, arm2, "same arm key components should produce same arm key")
}

func TestIsNumberLike(t *testing.T) {
	assert.True(t, isNumberLike("12345"))
	assert.True(t, isNumberLike("3.14"))
	assert.True(t, isNumberLike("1,000"))
	assert.True(t, isNumberLike("99%"))
	assert.False(t, isNumberLike("hello"))
	assert.False(t, isNumberLike(""))
	assert.False(t, isNumberLike("abc123"))
}
