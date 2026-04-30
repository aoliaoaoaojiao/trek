package screen

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"
	"trek/logger"
	"trek/pkg/driver/android/adb"
)

const (
	version          = "3.3.4"
	deviceServerPath = "/data/local/tmp/trek_scrcpy.jar"
)

//go:embed bin/scrcpy-server
var scrcpyBytes []byte

type Scrcpy struct {
	device            *adb.Device
	scrcpyLn          net.Listener
	localPort         int
	videoSocket       net.Conn
	exitCallBackFunc  context.CancelFunc
	exitCtx           context.Context
	videoFrameHandler func([]byte, uint64, bool)
}

func NewScrcpy(device *adb.Device) *Scrcpy {
	ln, err := net.Listen("tcp", ":0") // 0表示随机端口
	if err != nil {
		return nil
	}

	tcpAddr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		return nil
	}

	ctx, exitFunc := context.WithCancel(context.Background())

	return &Scrcpy{
		device:           device,
		scrcpyLn:         ln,
		localPort:        tcpAddr.Port,
		exitCtx:          ctx,
		exitCallBackFunc: exitFunc,
	}
}

// SetVideoFrameHandler 设置视频帧处理函数，参数为视频数据、pts(us)和是否为关键帧
func (s *Scrcpy) SetVideoFrameHandler(handler func(frameData []byte, oriPTS uint64, isKeyFrame bool)) {
	s.videoFrameHandler = handler
}

func (s *Scrcpy) Start(maxsize int) error {
	// todo
	var err error
	err = s.device.Push(bytes.NewReader(scrcpyBytes), deviceServerPath, time.Now())
	if err != nil {
		return err
	}

	err = s.device.ReverseLocalAbstract(getSocketName(-1), s.localPort)
	if err != nil {
		return err
	}

	s.startServer()

	err = s.runBinary(maxsize)

	return err
}

func (s *Scrcpy) runBinary(maxSize int) error {
	var output net.Conn

	output, err := s.device.RunShellLoopCommandSock(
		fmt.Sprintf("CLASSPATH=%s", deviceServerPath),
		"app_process",
		"/",
		"com.genymobile.scrcpy.Server",
		version,
		"log_level=debug",
		"max_size=0",
		"max_fps=60",
		"control=false",
		fmt.Sprintf("max_size=%d", maxSize),
		"audio=false",
		"send_codec_meta=false",
		"size_info=true",
	)

	if err != nil {
		s.exitCallBackFunc()
		return fmt.Errorf("execute scrcpy err: %v", err)
	}
	var isRelease sync.WaitGroup

	isRelease.Add(1)

	var err2 error

	// 运行scrcpy
	go func() {
		var byteDatas = make([]byte, 1024)

		n, err := output.Read(byteDatas)
		if err != nil {
			s.exitCallBackFunc()
			err2 = fmt.Errorf("start scrcpy err: %v", err)
			isRelease.Done()
			return
		}
		if !strings.Contains(string(byteDatas[:n]), "Device") {
			err2 = fmt.Errorf("not start scrcpy: %v", string(byteDatas[:n]))
			s.exitCallBackFunc()
			isRelease.Done()
			return
		}
		isRelease.Done()

		for {
			select {
			case <-s.exitCtx.Done():
				return
			default:
				n, err = output.Read(byteDatas)
				if err != nil {
					// 处理读取错误
					if err == io.EOF {
						// 正常结束
						logger.Debug("scrcpy output stream ended")
						return
					}
					if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
						// 超时错误，继续重试
						time.Sleep(100 * time.Millisecond)
						continue
					}
					// 其他错误，记录并退出
					logger.Errorf("get scrcpy binary output error: %v", err)
					s.exitCallBackFunc()
					return
				}
				// 输出调试信息
				if n > 0 {
					logger.Debug(string(byteDatas[:n]))
				}
			}
		}
	}()
	logger.Debugf("start scrcpy server!")
	isRelease.Wait()
	if err2 != nil {
		return err2
	}
	return nil
}

func (s *Scrcpy) startServer() {
	go func() {
		var err error
		s.videoSocket, err = s.scrcpyLn.Accept()
		if err != nil {
			logger.Errorf("get scrcpy video socket err: %v", err)
			return
		}
		// 解析和转发video socket
		go func() {
			s.videoParse()
		}()
	}()

}

func (s *Scrcpy) videoParse() {
	buffer := make([]byte, 64)
	_, err := s.videoSocket.Read(buffer)
	if err != nil {
		logger.Errorf("get scrcpy device info err: %v", err)
		s.exitCallBackFunc()
	}

	go func() {
		s.writeH264()
	}()
}

func (s *Scrcpy) writeH264() {
	// 包头缓冲区（12字节）
	headerBuf := make([]byte, 12)
	// 数据缓冲区（初始 4MB，按需扩容）
	dataBuf := make([]byte, 4*1024*1024)

	for {
		select {
		case <-s.exitCtx.Done():
			return
		default:
			// 读取包头（12字节）
			n, err := io.ReadFull(s.videoSocket, headerBuf)
			if err != nil {
				if err == io.EOF {
					logger.Info("scrcpy video stream ended")
				} else {
					logger.Errorf("read scrcpy packet header err: %v", err)
				}
				s.exitCallBackFunc()
				return
			}

			if n != 12 {
				logger.Errorf("incomplete packet header, got %d bytes, expected 12", n)
				s.exitCallBackFunc()
				return
			}

			// 解析包头
			packetSize, pts, isKeyFrame, _, err := s.parsePacketHeader(headerBuf)

			if err != nil {
				logger.Errorf("parse scrcpy packet header err: %v", err)
				s.exitCallBackFunc()
				return
			}

			if packetSize <= 0 {
				logger.Errorf("invalid packet size: %d", packetSize)
				s.exitCallBackFunc()
				return
			}

			// 读取实际数据
			if packetSize > len(dataBuf) {
				// 如果数据包太大，重新分配缓冲区
				dataBuf = make([]byte, packetSize)
			}

			n, err = io.ReadFull(s.videoSocket, dataBuf[:packetSize])
			if err != nil {
				logger.Errorf("read scrcpy packet data err: %v", err)
				s.exitCallBackFunc()
				return
			}

			if n != packetSize {
				logger.Errorf("incomplete packet data, got %d bytes, expected %d", n, packetSize)
				s.exitCallBackFunc()
				return
			}

			// 处理完整的数据包
			s.processVideoPacket(dataBuf[:packetSize], pts, isKeyFrame)
		}
	}
}

// 定义和Java端一致的标志位常量（Scrcpy标准定义）
const (
	PACKET_FLAG_CONFIG    = uint64(1 << 63) // 配置包标志（最高位）
	PACKET_FLAG_KEY_FRAME = uint64(1 << 62) // 关键帧标志
)

// 解析包头，返回：包大小、pts、是否关键帧、是否配置包、错误
func (s *Scrcpy) parsePacketHeader(header []byte) (packetSize int, pts uint64, isKeyFrame bool, isConfig bool, err error) {
	// header结构：
	// 0-7字节：ptsAndFlags (8字节，大端序)
	// 8-11字节：packetSize (4字节，大端序)

	// 校验header长度（必须至少12字节）
	if len(header) < 12 {
		return -1, 0, false, false, fmt.Errorf("无效的header长度：%d（需至少12字节）", len(header))
	}

	// 1. 解析8字节的ptsAndFlags（大端序，Java ByteBuffer默认大端序）
	ptsAndFlags := binary.BigEndian.Uint64(header[0:8])

	// 2. 判断是否为配置包
	isConfig = (ptsAndFlags & PACKET_FLAG_CONFIG) != 0

	if isConfig {
		// 配置包无媒体数据，pts和关键帧无意义
		pts = 0
		isKeyFrame = false
	} else {
		// 3. 提取原始pts（清除标志位）
		pts = ptsAndFlags & ^(PACKET_FLAG_CONFIG | PACKET_FLAG_KEY_FRAME)
		// 4. 判断是否为关键帧
		isKeyFrame = (ptsAndFlags & PACKET_FLAG_KEY_FRAME) != 0
	}

	// 5. 解析4字节的包大小（大端序）
	packetSizeBytes := header[8:12]
	packetSize = int(binary.BigEndian.Uint32(packetSizeBytes))

	return packetSize, pts, isKeyFrame, isConfig, nil
}

// 处理视频数据包
func (s *Scrcpy) processVideoPacket(data []byte, pts uint64, isKeyFrame bool) {
	if s.videoFrameHandler != nil {
		s.videoFrameHandler(data, pts, isKeyFrame)
	}
}

// 定义Socket名称前缀（对应Java中的SOCKET_NAME_PREFIX常量）
const socketNamePrefix = "scrcpy"

func getSocketName(scid int) string {
	if scid == -1 {
		// scid为-1时，直接返回前缀（简化scrcpy-server单独使用的场景）
		return socketNamePrefix
	}
	// 拼接前缀 + 下划线 + 8位十六进制格式的scid（%08x表示补零到8位的小写十六进制）
	return fmt.Sprintf("%s_%08x", socketNamePrefix, scid)
}
