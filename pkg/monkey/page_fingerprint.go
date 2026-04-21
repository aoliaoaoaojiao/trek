package monkey

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"github.com/beevik/etree"
)

const pageFingerprintPrefix = "XMLPage"

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
	return fmt.Sprintf("%s:%s", pageFingerprintPrefix, hex.EncodeToString(sum[:])[:16])
}

func canonicalPageSource(xml string) string {
	doc := etree.NewDocument()
	if err := doc.ReadFromString(xml); err != nil {
		return canonicalRawText(xml)
	}
	root := doc.Root()
	if root == nil {
		return canonicalRawText(xml)
	}
	var b strings.Builder
	writeCanonicalElement(&b, root)
	return b.String()
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
