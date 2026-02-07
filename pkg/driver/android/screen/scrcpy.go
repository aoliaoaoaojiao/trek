package screen

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"io"
	"math/rand"
	"net"
	"strings"
	"sync"
	"time"
	"trek/log"
	"trek/pkg/driver/android/gadb"
)

const (
	version          = "3.3.4"
	deviceServerPath = "/data/local/tmp/trek_scrcpy.jar"
)

//go:embed bin/scrcpy-server
var scrcpyBytes []byte

type Scrcpy struct {
	device            *gadb.Device
	scrcpyLn          net.Listener
	localPort         int
	videoSocket       net.Conn
	exitCallBackFunc  context.CancelFunc
	exitCtx           context.Context
	scid              int
	videoFrameHandler func([]byte)
}

func NewScrcpy(device *gadb.Device) *Scrcpy {
	ln, err := net.Listen("tcp", ":0") // 0表示随机端口
	if err != nil {
		return nil
	}

	tcpAddr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		return nil
	}

	ctx, exitFunc := context.WithCancel(context.Background())

	randInt := rand.New(rand.NewSource(time.Now().UnixNano()))

	randomMin := 2000
	randomMax := 5000

	randomNum := randomMin + randInt.Intn(randomMax-randomMin+1)

	return &Scrcpy{
		device:           device,
		scrcpyLn:         ln,
		localPort:        tcpAddr.Port,
		exitCtx:          ctx,
		exitCallBackFunc: exitFunc,
		scid:             randomNum,
	}
}

func (s *Scrcpy) SetVideoFrameHandler(handler func([]byte)) {
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

	go func() {
		<-s.exitCtx.Done()
	}()

	s.startServer()

	err = s.runBinary(maxsize)

	return err
}

func (s *Scrcpy) runBinary(maxSize int) error {
	var output io.Reader

	//output, err := s.device.RunShellLoopCommand(fmt.Sprintf("CLASSPATH=%s app_process / com.genymobile.scrcpy.Server v2.2  log_level=debug max_size=0 max_fps=60 control=false max_size=%d audio=false audio=false size_info=true",
	//	deviceServerPath,
	//	maxSize))

	output, err := s.device.RunShellLoopCommand(
		fmt.Sprintf("CLASSPATH=%s", deviceServerPath),
		"app_process",
		"/",
		"com.genymobile.scrcpy.Server",
		version,
		"log_level=debug",
		"max_size=0",
		"max_fps=60",
		"control=false",
		//fmt.Sprintf("scid=%d", s.scid),
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
						log.Debug("scrcpy output stream ended")
						return
					}
					if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
						// 超时错误，继续重试
						time.Sleep(100 * time.Millisecond)
						continue
					}
					// 其他错误，记录并退出
					log.Errorf("get scrcpy binary output error: %v", err)
					s.exitCallBackFunc()
					return
				}
				// 输出调试信息
				if n > 0 {
					//fmt.Println(string(byteDatas[:n]))
					log.Debug(string(byteDatas[:n]))
				}
			}
		}
	}()
	log.Debugf("start scrcpy server!")
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
			log.Errorf("get scrcpy video socket err: %v", err)
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
		log.Errorf("get scrcpy device info err: %v", err)
		s.exitCallBackFunc()
	}
	//buffer = make([]byte, 12)
	//_, err = s.videoSocket.Read(buffer)
	//if err != nil {
	//	log.Errorf("get scrcpy device width and height info err: %v", err)
	//	s.exitCallBackFunc()
	//}

	go func() {
		s.writeH264()
	}()
}

func (s *Scrcpy) writeH264() {
	// 包头缓冲区（12字节）
	headerBuf := make([]byte, 12)
	// 数据缓冲区
	dataBuf := make([]byte, 1024*1024*1024)

	for {
		select {
		case <-s.exitCtx.Done():
			return
		default:
			//log.Debug("read scrcpy video packet")
			// 读取包头（12字节）
			n, err := io.ReadFull(s.videoSocket, headerBuf)
			if err != nil {
				if err == io.EOF {
					log.Info("scrcpy video stream ended")
				} else {
					log.Errorf("read scrcpy packet header err: %v", err)
				}
				s.exitCallBackFunc()
				return
			}

			if n != 12 {
				log.Errorf("incomplete packet header, got %d bytes, expected 12", n)
				s.exitCallBackFunc()
				return
			}

			// 解析包头
			packetSize := s.parsePacketHeader(headerBuf)
			if packetSize <= 0 {
				log.Errorf("invalid packet size: %d", packetSize)
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
				log.Errorf("read scrcpy packet data err: %v", err)
				s.exitCallBackFunc()
				return
			}

			if n != packetSize {
				log.Errorf("incomplete packet data, got %d bytes, expected %d", n, packetSize)
				s.exitCallBackFunc()
				return
			}

			// 处理完整的数据包
			s.processVideoPacket(dataBuf[:packetSize])
		}
	}
}

// 解析包头，返回数据包大小
func (s *Scrcpy) parsePacketHeader(header []byte) int {
	// header结构：
	// 0-7字节：ptsAndFlags (8字节)
	// 8-11字节：packetSize (4字节，大端序)

	// 读取4字节的包大小（大端序）
	if len(header) < 12 {
		return -1
	}
	packetSize := int(header[8])<<24 | int(header[9])<<16 | int(header[10])<<8 | int(header[11])
	return packetSize
}

// 处理视频数据包
func (s *Scrcpy) processVideoPacket(data []byte) {
	if s.videoFrameHandler != nil {
		s.videoFrameHandler(data)
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
