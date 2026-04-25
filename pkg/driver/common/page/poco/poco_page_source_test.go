package poco

import (
	"strings"
	"testing"
)

func TestBuildPocoDumpXML(t *testing.T) {
	result := map[string]interface{}{
		"name": "<Root>",
		"payload": map[string]interface{}{
			"name":      "<Root>",
			"type":      "Root",
			"visible":   true,
			"clickable": false,
			"pos":       []interface{}{0.5, 0.5},
			"size":      []interface{}{1.0, 1.0},
		},
		"children": []interface{}{
			map[string]interface{}{
				"name": "btn_start",
				"payload": map[string]interface{}{
					"name":      "btn_start",
					"type":      "Button",
					"text":      "Start",
					"visible":   true,
					"clickable": true,
					"pos":       []interface{}{0.5, 0.35},
					"size":      []interface{}{0.08, 0.10},
				},
			},
		},
	}

	xmlText, err := buildPocoDumpXML(result)
	if err != nil {
		t.Fatalf("构建 XML 失败: %v", err)
	}

	mustContain(t, xmlText, "<hierarchy>")
	mustContain(t, xmlText, "resource-id=\"btn_start\"")
	mustContain(t, xmlText, "class=\"Button\"")
	mustContain(t, xmlText, "clickable=\"true\"")
	mustContain(t, xmlText, "bounds=\"[0.460000,0.300000][0.540000,0.400000]\"")
}

func TestBuildPocoDumpXMLEditableByInputField(t *testing.T) {
	result := map[string]interface{}{
		"name": "root",
		"payload": map[string]interface{}{
			"name":      "root",
			"type":      "Root",
			"visible":   true,
			"clickable": false,
			"pos":       []interface{}{0.5, 0.5},
			"size":      []interface{}{1.0, 1.0},
		},
		"children": []interface{}{
			map[string]interface{}{
				"name": "input_name",
				"payload": map[string]interface{}{
					"name":      "input_name",
					"type":      "InputField",
					"visible":   true,
					"clickable": true,
					"pos":       []interface{}{0.5, 0.6},
					"size":      []interface{}{0.4, 0.1},
				},
			},
		},
	}

	xmlText, err := buildPocoDumpXML(result)
	if err != nil {
		t.Fatalf("构建 XML 失败: %v", err)
	}

	mustContain(t, xmlText, "class=\"InputField\"")
	mustContain(t, xmlText, "editable=\"true\"")
}

func mustContain(t *testing.T, text string, expected string) {
	t.Helper()
	if !strings.Contains(text, expected) {
		t.Fatalf("预期包含 %q，实际: %s", expected, text)
	}
}
