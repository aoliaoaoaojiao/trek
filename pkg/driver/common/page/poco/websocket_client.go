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
	connected bool
	msgCh     chan []byte
}

func NewWebSocketClient(port int) *WebSocketClient {
	return &WebSocketClient{
		port:  port,
		msgCh: make(chan []byte, 1),
	}
}

func (w *WebSocketClient) Connect() error {
	dialer := websocket.Dialer{}

	conn, _, err := dialer.Dial(fmt.Sprintf("ws://localhost:%d", w.port), http.Header{})
	if err != nil {
		return fmt.Errorf("连接 Poco WebSocket 失败: %v", err)
	}

	w.mu.Lock()
	w.conn = conn
	w.connected = true
	w.mu.Unlock()

	go w.readLoop()

	// 等待 readLoop 启动确认
	for i := 0; i < 20; i++ {
		w.mu.Lock()
		c := w.connected
		w.mu.Unlock()
		if c {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("等待 WebSocket 连接超时")
}

func (w *WebSocketClient) readLoop() {
	defer func() {
		w.mu.Lock()
		w.connected = false
		w.mu.Unlock()
	}()

	for {
		_, message, err := w.conn.ReadMessage()
		if err != nil {
			break
		}

		select {
		case w.msgCh <- message:
		default:
			// 丢弃旧消息，保留最新
			select {
			case <-w.msgCh:
			default:
			}
			w.msgCh <- message
		}
	}
}

func (w *WebSocketClient) SendAndReceive(data []byte) ([]byte, error) {
	w.mu.Lock()
	if !w.connected || w.conn == nil {
		w.mu.Unlock()
		return nil, fmt.Errorf("未连接")
	}
	conn := w.conn
	w.mu.Unlock()

	// 清空残留消息
	select {
	case <-w.msgCh:
	default:
	}

	err := conn.WriteMessage(websocket.TextMessage, data)
	if err != nil {
		return nil, fmt.Errorf("发送数据失败: %v", err)
	}

	select {
	case result := <-w.msgCh:
		return result, nil
	case <-time.After(10 * time.Second):
		return nil, fmt.Errorf("等待响应超时")
	}
}

func (w *WebSocketClient) Disconnect() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.conn != nil {
		w.conn.Close()
		w.conn = nil
		w.connected = false
	}
}

func (w *WebSocketClient) IsConnected() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.connected && w.conn != nil
}
