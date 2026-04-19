package screen

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"sync"
	"trek/logger"
	"trek/pkg/driver/android/adb"
	"trek/pkg/driver/common"

	"github.com/google/uuid"
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

func (s *ScreenCapture) Screenshot() ([]byte, error) {
	uuid := uuid.NewString()
	imgPath := fmt.Sprintf("/sdcard/%s.png", uuid)
	logger.Debugf("Starting device screenshot, serial=%s remotePath=%s", s.device.Serial(), imgPath)

	_, err := s.device.RunShellCommand(fmt.Sprintf("screencap -p %s", imgPath))
	if err != nil {
		return nil, fmt.Errorf("鎴浘澶辫触: %v", err)
	}

	dest := bytes.Buffer{}
	err = s.device.Pull(imgPath, &dest)
	if err != nil {
		return nil, fmt.Errorf("鎷夊彇鎴浘澶辫触: %v", err)
	}

	_, _ = s.device.RunShellCommand(fmt.Sprintf("rm %s", imgPath))
	logger.Debugf("Device screenshot completed, serial=%s size=%d", s.device.Serial(), dest.Len())
	return dest.Bytes(), nil
}

func (s *ScreenCapture) SaveScreenshot(path string) error {
	logger.Debugf("Starting screenshot save, serial=%s path=%s", s.device.Serial(), path)
	data, err := s.Screenshot()
	if err != nil {
		return fmt.Errorf("鎴浘澶辫触: %v", err)
	}

	err = os.WriteFile(path, data, 0666)
	if err != nil {
		return fmt.Errorf("淇濆瓨鎴浘澶辫触: %v", err)
	}

	logger.Debugf("Screenshot save completed, serial=%s path=%s size=%d", s.device.Serial(), path, len(data))
	return nil
}

func (s *ScreenCapture) Record(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	logger.Debugf("Starting screen recording initialization, serial=%s path=%s", s.device.Serial(), path)

	if s.isRecording {
		return fmt.Errorf("宸茬粡姝ｅ湪褰曞埗涓?")
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		return fmt.Errorf("鍒涘缓鏂囦欢澶辫触: %v", err)
	}

	muxer, err := mp4.CreateMp4Muxer(file)
	if err != nil {
		file.Close()
		return fmt.Errorf("鍒涘缓 MP4 Muxer 澶辫触: %v", err)
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
		return fmt.Errorf("鍚姩 scrcpy 澶辫触: %v", err)
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
		return fmt.Errorf("褰撳墠娌℃湁鍦ㄥ綍鍒?")
	}

	if s.cancelFunc != nil {
		s.cancelFunc()
	}

	var errs []error
	if s.muxer != nil {
		if err := s.muxer.WriteTrailer(); err != nil {
			errs = append(errs, fmt.Errorf("鍐欏叆 MP4 trailer 澶辫触: %v", err))
		}
	}
	if s.file != nil {
		if err := s.file.Close(); err != nil {
			errs = append(errs, fmt.Errorf("鍏抽棴鏂囦欢澶辫触: %v", err))
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
		return fmt.Errorf("鍋滄褰曞埗鏃跺彂鐢熼敊璇? %v", errs)
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
