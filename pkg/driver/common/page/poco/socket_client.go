package poco

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

type SocketClient struct {
	conn      net.Conn
	port      int
	mu        sync.Mutex
	connected bool
}

func NewSocketClient(port int) *SocketClient {
	return &SocketClient{
		port: port,
	}
}

func (s *SocketClient) Connect() error {
	var conn net.Conn
	var err error

	backoff := 200 * time.Millisecond
	maxBackoff := 5 * time.Second

	for i := 0; i < 20; i++ {
		conn, err = net.Dial("tcp", fmt.Sprintf("localhost:%d", s.port))
		if err == nil {
			s.conn = conn
			s.connected = true
			return nil
		}
		time.Sleep(backoff)
		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}

	return fmt.Errorf("连接 Poco Socket 失败: %v", err)
}

func (s *SocketClient) SendAndReceive(data []byte) ([]byte, error) {
	if !s.connected || s.conn == nil {
		return nil, fmt.Errorf("未连接")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	header := make([]byte, 4)
	binary.LittleEndian.PutUint32(header, uint32(len(data)))

	_, err := s.conn.Write(header)
	if err != nil {
		return nil, fmt.Errorf("发送数据失败: %v", err)
	}

	_, err = s.conn.Write(data)
	if err != nil {
		return nil, fmt.Errorf("发送数据失败: %v", err)
	}

	respHeader := make([]byte, 4)
	_, err = io.ReadFull(s.conn, respHeader)
	if err != nil {
		return nil, fmt.Errorf("读取响应头失败: %v", err)
	}

	respLen := binary.LittleEndian.Uint32(respHeader)
	respData := make([]byte, respLen)

	_, err = io.ReadFull(s.conn, respData)
	if err != nil {
		return nil, fmt.Errorf("读取响应数据失败: %v", err)
	}

	return respData, nil
}

func (s *SocketClient) Disconnect() {
	if s.conn != nil {
		s.conn.Close()
		s.conn = nil
		s.connected = false
	}
}

func (s *SocketClient) IsConnected() bool {
	return s.connected && s.conn != nil
}
