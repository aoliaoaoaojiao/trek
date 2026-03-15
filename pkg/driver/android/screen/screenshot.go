package screen

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"sync"
	"trek/pkg/driver/android/gadb"
	"trek/pkg/driver/common"

	"github.com/google/uuid"
	"github.com/yapingcat/gomedia/go-codec"
	"github.com/yapingcat/gomedia/go-mp4"
)

var _ common.IScreenCapture = (*ScreenCapture)(nil)

type ScreenCapture struct {
	device *gadb.Device

	mu          sync.Mutex
	scrcpy      *Scrcpy
	muxer       *mp4.Movmuxer
	file        *os.File
	cancelFunc  context.CancelFunc
	trackID     uint32
	initPts     uint64
	isInit      bool
	isRecording bool
}

func NewScreenCapture(device *gadb.Device) *ScreenCapture {
	return &ScreenCapture{
		device: device,
	}
}

func (s *ScreenCapture) Screenshot() ([]byte, error) {
	uuid := uuid.NewString()
	imgPath := fmt.Sprintf("/sdcard/%s.png", uuid)

	// 使用 screencap 命令
	_, err := s.device.RunShellCommand(fmt.Sprintf("screencap -p %s", imgPath))
	if err != nil {
		return nil, fmt.Errorf("截图失败: %v", err)
	}

	dest := bytes.Buffer{}
	err = s.device.Pull(imgPath, &dest)
	if err != nil {
		return nil, fmt.Errorf("拉取截图失败: %v", err)
	}

	// 清理设备上的临时文件
	_, _ = s.device.RunShellCommand(fmt.Sprintf("rm %s", imgPath))

	return dest.Bytes(), nil
}

func (s *ScreenCapture) SaveScreenshot(path string) error {
	data, err := s.Screenshot()
	if err != nil {
		return fmt.Errorf("截图失败: %v", err)
	}

	err = os.WriteFile(path, data, 0666)
	if err != nil {
		return fmt.Errorf("保存截图失败: %v", err)
	}

	return nil
}

func (s *ScreenCapture) Record(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.isRecording {
		return fmt.Errorf("已经正在录制中")
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		return fmt.Errorf("创建文件失败: %v", err)
	}

	muxer, err := mp4.CreateMp4Muxer(file)
	if err != nil {
		file.Close()
		return fmt.Errorf("创建 MP4 Muxer 失败: %v", err)
	}

	trackID := muxer.AddVideoTrack(mp4.MP4_CODEC_H264)

	ctx, cancel := context.WithCancel(context.Background())

	s.file = file
	s.muxer = muxer
	s.trackID = trackID
	s.cancelFunc = cancel
	s.initPts = 0
	s.isInit = false
	s.isRecording = true

	scrcpy := NewScrcpy(s.device)
	scrcpy.SetVideoFrameHandler(func(frameData []byte, oriPTS uint64, isKeyFrame bool) {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if !s.isInit {
			s.initPts = oriPTS
			s.isInit = true
		}

		codec.SplitFrameWithStartCode(frameData, func(nalu []byte) bool {
			pts := (oriPTS - s.initPts) / 1000
			s.muxer.Write(s.trackID, nalu, pts, pts)
			return true
		})
	})

	if err := scrcpy.Start(1000); err != nil {
		file.Close()
		s.isRecording = false
		return fmt.Errorf("启动 scrcpy 失败: %v", err)
	}

	s.scrcpy = scrcpy

	return nil
}

func (s *ScreenCapture) StopRecording() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isRecording {
		return fmt.Errorf("当前没有在录制")
	}

	if s.cancelFunc != nil {
		s.cancelFunc()
	}

	var errs []error

	if s.muxer != nil {
		if err := s.muxer.WriteTrailer(); err != nil {
			errs = append(errs, fmt.Errorf("写入 MP4 trailer 失败: %v", err))
		}
	}

	if s.file != nil {
		if err := s.file.Close(); err != nil {
			errs = append(errs, fmt.Errorf("关闭文件失败: %v", err))
		}
	}

	s.isRecording = false
	s.scrcpy = nil
	s.muxer = nil
	s.file = nil
	s.cancelFunc = nil
	s.initPts = 0
	s.isInit = false

	if len(errs) > 0 {
		return fmt.Errorf("停止录制时发生错误: %v", errs)
	}

	return nil
}

func (s *ScreenCapture) Close() error {
	if s.isRecording {
		return s.StopRecording()
	}
	return nil
}
