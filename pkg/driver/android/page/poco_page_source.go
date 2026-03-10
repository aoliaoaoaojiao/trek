package page

import (
	"encoding/json"
	"fmt"
	"trek/pkg/driver"
	"trek/pkg/driver/android/gadb"
	"trek/pkg/driver/android/page/poco"

	"github.com/google/uuid"
)

var _ driver.IPageSource = (*PocoPageSource)(nil)

type PocoPageSource struct {
	device *gadb.Device
	engine poco.Engine
	conn   poco.PocoConnection
}

func NewPocoPageSource(device *gadb.Device, engine poco.Engine, port int) (*PocoPageSource, error) {
	p := &PocoPageSource{
		device: device,
		engine: engine,
	}

	var conn poco.PocoConnection
	if engine.IsWebSocket() {
		conn = poco.NewWebSocketClient(port)
	} else {
		conn = poco.NewSocketClient(port)
	}

	if err := conn.Connect(); err != nil {
		return nil, fmt.Errorf("连接 Poco 失败: %v", err)
	}

	p.conn = conn

	return p, nil
}

func (p *PocoPageSource) DumpPageSource() (string, error) {

	method := "Dump"
	if p.engine == poco.CocosCreator || p.engine == poco.Cocos2dxJs {
		method = "dump"
	}

	req := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      uuid.New().String(),
		"method":  method,
		"params":  []interface{}{true},
	}

	reqData, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("序列化请求失败: %v", err)
	}

	respData, err := p.conn.SendAndReceive(reqData)
	if err != nil {
		return "", fmt.Errorf("发送请求失败: %v", err)
	}

	var respMap map[string]interface{}
	if err := json.Unmarshal(respData, &respMap); err != nil {
		return "", fmt.Errorf("解析响应失败: %v", err)
	}

	result, ok := respMap["result"]
	if !ok {
		return "", fmt.Errorf("响应中未找到 result 字段")
	}

	resultBytes, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("序列化 result 失败: %v", err)
	}

	return string(resultBytes), nil
}

func (p *PocoPageSource) GetScreenSize() (int, int, error) {
	if p.engine != poco.Unity3d && p.engine != poco.Cocos2dxLua {
		return 0, 0, fmt.Errorf("当前引擎不支持获取屏幕尺寸")
	}

	method := "GetScreenSize"
	req := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      uuid.New().String(),
		"method":  method,
		"params":  []interface{}{true},
	}

	reqData, err := json.Marshal(req)
	if err != nil {
		return 0, 0, fmt.Errorf("序列化请求失败: %v", err)
	}

	respData, err := p.conn.SendAndReceive(reqData)
	if err != nil {
		return 0, 0, fmt.Errorf("发送请求失败: %v", err)
	}

	var respMap map[string]interface{}
	if err := json.Unmarshal(respData, &respMap); err != nil {
		return 0, 0, fmt.Errorf("解析响应失败: %v", err)
	}

	result, ok := respMap["result"]
	if !ok {
		return 0, 0, fmt.Errorf("响应中未找到 result 字段")
	}

	resultArray, ok := result.([]interface{})
	if !ok || len(resultArray) < 2 {
		return 0, 0, fmt.Errorf("result 格式错误")
	}

	width := int(resultArray[0].(float64))
	height := int(resultArray[1].(float64))

	return width, height, nil
}

func (p *PocoPageSource) Close() error {
	if p.conn != nil {
		p.conn.Disconnect()
	}
	return nil
}
