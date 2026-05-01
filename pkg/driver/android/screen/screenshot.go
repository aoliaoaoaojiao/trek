package screen

import (
	"context"
	"fmt"
	"os"
	"sync"
	"trek/logger"
	"trek/pkg/driver/android/adb"
	"trek/pkg/driver/common"

	"github.com/yapingcat/gomedia/go-codec"
	"github.com/yapingcat/gomedia/go-mp4"
)

var _ common.IScreenCapture = (*ScreenCapture)(nil)

type ScreenCapture struct {
	device *adb.Device

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

func NewScreenCapture(device *adb.Device) *ScreenCapture {
	return &ScreenCapture{device: device}
}

func (s *ScreenCapture) Screenshot(ctx context.Context) ([]byte, error) {
	logger.Debugf("Starting device screenshot, serial=%s", s.device.Serial())
	data, err := s.device.Screenshot(ctx)
	if err != nil {
		return nil, fmt.Errorf("截图失败: %v", err)
	}
	logger.Debugf("Device screenshot completed, serial=%s size=%d", s.device.Serial(), len(data))
	return data, nil
}

func (s *ScreenCapture) SaveScreenshot(path string) error {
	logger.Debugf("Starting screenshot save, serial=%s path=%s", s.device.Serial(), path)
	data, err := s.Screenshot(context.Background())
	if err != nil {
		return fmt.Errorf("截图失败: %v", err)
	}

	err = os.WriteFile(path, data, 0666)
	if err != nil {
		return fmt.Errorf("保存截图失败: %v", err)
	}

	logger.Debugf("Screenshot save completed, serial=%s path=%s size=%d", s.device.Serial(), path, len(data))
	return nil
}

func (s *ScreenCapture) Record(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	logger.Debugf("Starting screen recording initialization, serial=%s path=%s", s.device.Serial(), path)

	if s.isRecording {
		return fmt.Errorf("已经在录制中")
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
	logger.Debugf("Screen recording started, serial=%s path=%s trackID=%d", s.device.Serial(), path, s.trackID)
	return nil
}

func (s *ScreenCapture) StopRecording() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	logger.Debugf("Starting screen recording stop, serial=%s", s.device.Serial())

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

	logger.Debugf("Screen recording stopped, serial=%s", s.device.Serial())
	return nil
}

func (s *ScreenCapture) Close() error {
	if s.isRecording {
		return s.StopRecording()
	}
	return nil
}
