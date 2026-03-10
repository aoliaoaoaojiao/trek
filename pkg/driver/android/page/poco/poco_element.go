package poco

type PocoElement struct {
	Data map[string]interface{}
}

func (e *PocoElement) GetName() string {
	if v, ok := e.Data["name"].(string); ok {
		return v
	}
	return ""
}

func (e *PocoElement) GetText() string {
	if v, ok := e.Data["text"].(string); ok {
		return v
	}
	return ""
}

func (e *PocoElement) GetType() string {
	if v, ok := e.Data["type"].(string); ok {
		return v
	}
	return ""
}

func (e *PocoElement) GetVisible() bool {
	if v, ok := e.Data["visible"].(bool); ok {
		return v
	}
	return false
}

func (e *PocoElement) GetClickable() bool {
	if v, ok := e.Data["clickable"].(bool); ok {
		return v
	}
	return false
}

func (e *PocoElement) GetX() float64 {
	if v, ok := e.Data["x"].(float64); ok {
		return v
	}
	return 0
}

func (e *PocoElement) GetY() float64 {
	if v, ok := e.Data["y"].(float64); ok {
		return v
	}
	return 0
}

func (e *PocoElement) GetWidth() float64 {
	if v, ok := e.Data["width"].(float64); ok {
		return v
	}
	return 0
}

func (e *PocoElement) GetHeight() float64 {
	if v, ok := e.Data["height"].(float64); ok {
		return v
	}
	return 0
}

func (e *PocoElement) GetCenterX() float64 {
	return e.GetX() + e.GetWidth()/2
}

func (e *PocoElement) GetCenterY() float64 {
	return e.GetY() + e.GetHeight()/2
}

func (e *PocoElement) GetChildren() []map[string]interface{} {
	if children, ok := e.Data["children"].([]interface{}); ok {
		result := make([]map[string]interface{}, len(children))
		for i, child := range children {
			if childMap, ok := child.(map[string]interface{}); ok {
				result[i] = childMap
			}
		}
		return result
	}
	return nil
}

func (e *PocoElement) GetAttr(name string) interface{} {
	return e.Data[name]
}
