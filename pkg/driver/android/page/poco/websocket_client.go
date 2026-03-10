package poco

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type WebSocketClient struct {
	conn      *websocket.Conn
	port      int
	mu        sync.Mutex
	result    string
	connected bool
	muResult  sync.RWMutex
}

func NewWebSocketClient(port int) *WebSocketClient {
	return &WebSocketClient{
		port: port,
	}
}

func (w *WebSocketClient) Connect() error {
	dialer := websocket.Dialer{}

	conn, _, err := dialer.Dial(fmt.Sprintf("ws://localhost:%d", w.port), http.Header{})
	if err != nil {
		return fmt.Errorf("连接 Poco WebSocket 失败: %v", err)
	}

	w.conn = conn
	w.connected = true

	go w.readLoop()

	for i := 0; i < 20; i++ {
		if w.connected {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("等待 WebSocket 连接超时")
}

func (w *WebSocketClient) readLoop() {
	defer func() {
		w.connected = false
	}()

	for {
		_, message, err := w.conn.ReadMessage()
		if err != nil {
			break
		}

		w.muResult.Lock()
		w.result = string(message)
		w.muResult.Unlock()
	}
}

func (w *WebSocketClient) SendAndReceive(data []byte) ([]byte, error) {
	if !w.connected || w.conn == nil {
		return nil, fmt.Errorf("未连接")
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	err := w.conn.WriteMessage(websocket.TextMessage, data)
	if err != nil {
		return nil, fmt.Errorf("发送数据失败: %v", err)
	}

	for i := 0; i < 20; i++ {
		time.Sleep(500 * time.Millisecond)

		w.muResult.RLock()
		result := w.result
		w.muResult.RUnlock()

		if result != "" {
			w.muResult.Lock()
			w.result = ""
			w.muResult.Unlock()
			return []byte(result), nil
		}
	}

	return nil, fmt.Errorf("等待响应超时")
}

func (w *WebSocketClient) Disconnect() {
	if w.conn != nil {
		w.conn.Close()
		w.conn = nil
		w.connected = false
	}
}

func (w *WebSocketClient) IsConnected() bool {
	return w.connected && w.conn != nil
}
