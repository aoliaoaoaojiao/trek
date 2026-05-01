package monkey

import (
	"crypto/sha1"
	"encoding/hex"
	"sort"
	"strings"
	"sync"

	"github.com/beevik/etree"
)

// canonicalCache 缓存 canonicalPageSource 结果，避免同一 XML 重复解析 etree。
var canonicalCache sync.Map // map[uint64]string

const pageFingerprintPrefix = "XMLPage"
const fnv64aOffset = 14695981039346656037
const fnv64aPrime = 1099511628211

// fnv64aString 内联计算 FNV-1a 哈希，避免分配 hash/fnv 对象。
func fnv64aString(s string) uint64 {
	h := uint64(fnv64aOffset)
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= fnv64aPrime
	}
	return h
}

var pageStructureRoleAttrs = []string{
	"class",
	"role",
	"type",
	"widget",
}

var pageStructureCapabilityAttrs = []string{
	"checkable",
	"clickable",
	"editable",
	"enabled",
	"focusable",
	"long-clickable",
	"scrollable",
	"touchable",
}

// defaultPageNameResolver 使用结构指纹生成页面名，不直接依赖 UIA/Poco 的业务字段值。
// 这样动态文案、控件名称变化不会让页面名抖动，后续也便于扩展图像策略。
func defaultPageNameResolver(xml string) string {
	return pageFingerprintName(xml)
}

func pageFingerprintName(xml string) string {
	canonical := canonicalPageSource(xml)
	if canonical == "" {
		return "UnknownPage"
	}
	sum := sha1.Sum([]byte(canonical))
	return pageFingerprintPrefix + ":" + hex.EncodeToString(sum[:])[:16]
}

func canonicalPageSource(xml string) string {
	trimmed := strings.TrimSpace(xml)
	if trimmed == "" {
		return ""
	}
	// 用内容哈希作缓存键，避免大 XML 字符串作 map key 的内存开销。
	key := fnv64aString(trimmed)
	if cached, ok := canonicalCache.Load(key); ok {
		return cached.(string)
	}
	doc := etree.NewDocument()
	if err := doc.ReadFromString(xml); err != nil {
		result := canonicalRawText(xml)
		canonicalCache.Store(key, result)
		return result
	}
	root := doc.Root()
	if root == nil {
		result := canonicalRawText(xml)
		canonicalCache.Store(key, result)
		return result
	}
	var b strings.Builder
	writeCanonicalElement(&b, root)
	result := b.String()
	canonicalCache.Store(key, result)
	return result
}

func canonicalRawText(text string) string {
	fields := strings.Fields(strings.TrimSpace(text))
	if len(fields) == 0 {
		return ""
	}
	return strings.Join(fields, " ")
}

func writeCanonicalElement(b *strings.Builder, elem *etree.Element) {
	b.WriteByte('(')
	b.WriteString(strings.ToLower(strings.TrimSpace(elem.Tag)))
	writeCanonicalStructureAttrs(b, elem)
	for _, child := range elem.ChildElements() {
		writeCanonicalElement(b, child)
	}
	b.WriteByte(')')
}

func writeCanonicalStructureAttrs(b *strings.Builder, elem *etree.Element) {
	attrs := make([]string, 0, len(pageStructureRoleAttrs)+len(pageStructureCapabilityAttrs))
	for _, key := range pageStructureRoleAttrs {
		if value, ok := canonicalElementAttr(elem, key); ok {
			attrs = append(attrs, key+"="+strings.ToLower(value))
		}
	}
	for _, key := range pageStructureCapabilityAttrs {
		if value, ok := canonicalElementAttr(elem, key); ok {
			attrs = append(attrs, key+"="+strings.ToLower(value))
		}
	}
	if len(attrs) == 0 {
		return
	}
	sort.Strings(attrs)
	b.WriteByte('[')
	b.WriteString(strings.Join(attrs, "|"))
	b.WriteByte(']')
}

func canonicalElementAttr(elem *etree.Element, key string) (string, bool) {
	for _, attr := range elem.Attr {
		if !strings.EqualFold(strings.TrimSpace(attr.Key), key) {
			continue
		}
		value := strings.Join(strings.Fields(strings.TrimSpace(attr.Value)), " ")
		if value == "" {
			return "", false
		}
		return value, true
	}
	return "", false
}
