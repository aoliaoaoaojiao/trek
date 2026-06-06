package screen

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"
	"trek/logger"
	"trek/pkg/driver/android/adb"
	"trek/pkg/driver/common"

	"github.com/yapingcat/gomedia/go-codec"
	"github.com/yapingcat/gomedia/go-mp4"
)

var _ common.IScreenCapture = (*ScreenCapture)(nil)

// ScreenCapture 提供 Android 设备的截图和录屏功能。
// 支持通过 scrcpy 流进行高速截图（避免 ADB 往返延迟）。
// 支持后台截图模式：启动后台线程持续截图，Screenshot() 直接返回最新帧。
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

	// scrcpy 高速截图提供者（用于快速截图）
	screenshotProvider *ScrcpyScreenshotProvider
	screenshotMode     ScreenshotMode

	// 后台截图模式
	bgLatest      atomic.Value // 存放 *bgFrame
	bgCancel      context.CancelFunc
	bgWg          sync.WaitGroup
	lastActionDone atomic.Int64 // 上一步动作完成的 UnixNano，用于确保截图在动作之后
}

type bgFrame struct {
	data []byte
	ts   time.Time
}

// NewScreenCapture 创建标准截图捕获器（仅 ADB screencap）。
func NewScreenCapture(device *adb.Device) *ScreenCapture {
	return &ScreenCapture{
		device:          device,
		screenshotMode:  ScreenshotModeADB,
	}
}

// NewScreenCaptureWithScrcpy 创建支持 scrcpy 高速截图的捕获器。
// config 为 nil 时使用默认配置。
func NewScreenCaptureWithScrcpy(device *adb.Device, config *ScrcpyScreenshotConfig) *ScreenCapture {
	cfg := DefaultScrcpyScreenshotConfig()
	if config != nil {
		cfg = *config
	}
	provider := NewScrcpyScreenshotProvider(device, cfg)
	return &ScreenCapture{
		device:             device,
		screenshotMode:     cfg.Mode,
		screenshotProvider: provider,
	}
}

// Screenshot 返回 PNG 格式的截图。
// 后台模式下直接返回最新帧（零等待），否则走原有的 scrcpy/ADB 路径。
// 如果帧时间戳早于上一步动作完成时间，会短暂等待新帧以确保截图反映动作后的状态。
func (s *ScreenCapture) Screenshot(ctx context.Context) ([]byte, error) {
	// 后台模式：返回最新帧，但确保是动作之后的
	if v := s.bgLatest.Load(); v != nil {
		f := v.(*bgFrame)
		actionDone := s.lastActionDone.Load()
		if actionDone > 0 && f.ts.UnixNano() < actionDone {
			// 帧比动作早，短暂等待新帧（最多 300ms）
			deadline := time.Now().Add(300 * time.Millisecond)
			for time.Now().Before(deadline) {
				time.Sleep(50 * time.Millisecond)
				if v2 := s.bgLatest.Load(); v2 != nil {
					f2 := v2.(*bgFrame)
					if f2.ts.UnixNano() >= actionDone {
						logger.Debugf("后台截图等待到动作后新帧 (%d 字节, 年龄=%v)", len(f2.data), time.Since(f2.ts))
						return f2.data, nil
					}
				}
			}
			logger.Debugf("后台截图等待超时，返回当前帧 (%d 字节, 年龄=%v)", len(f.data), time.Since(f.ts))
		} else {
			logger.Debugf("后台截图返回最新帧 (%d 字节, 年龄=%v)", len(f.data), time.Since(f.ts))
		}
		return f.data, nil
	}
	return s.screenshotDirect(ctx)
}

// MarkActionDone 记录动作完成时间，供 Screenshot() 判断帧是否在动作之后。
func (s *ScreenCapture) MarkActionDone() {
	s.lastActionDone.Store(time.Now().UnixNano())
}

// screenshotDirect 执行实际截图（不检查后台缓存），供后台线程和首帧使用。
func (s *ScreenCapture) screenshotDirect(ctx context.Context) ([]byte, error) {
	// 如果启用了 scrcpy 高速截图，优先使用
	if s.screenshotMode != ScreenshotModeADB && s.screenshotProvider != nil {
		data, err := s.screenshotProvider.Screenshot(ctx)
		if err == nil && len(data) > 0 {
			return data, nil
		}
		// scrcpy 失败且为 Auto 模式时回退 ADB
		if s.screenshotMode == ScreenshotModeAuto {
			logger.Warnf("scrcpy 截图失败，回退 ADB: %v", err)
			return s.device.Screenshot(ctx)
		}
		return nil, err
	}

	// 标准 ADB screencap
	logger.Debugf("Starting device screenshot, serial=%s", s.device.Serial())
	data, err := s.device.Screenshot(ctx)
	if err != nil {
		return nil, fmt.Errorf("截图失败: %v", err)
	}
	logger.Debugf("Device screenshot completed, serial=%s size=%d", s.device.Serial(), len(data))
	return data, nil
}

// SetScreenshotMode 设置截图获取模式。
func (s *ScreenCapture) SetScreenshotMode(mode ScreenshotMode) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.screenshotMode = mode
}

// SetScreenshotProvider 设置 scrcpy 截图提供者。
func (s *ScreenCapture) SetScreenshotProvider(provider *ScrcpyScreenshotProvider) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.screenshotProvider = provider
	if provider != nil {
		s.screenshotMode = provider.config.Mode
	}
}

// ScreenshotProvider 返回当前的 scrcpy 截图提供者（可能为 nil）。
func (s *ScreenCapture) ScreenshotProvider() *ScrcpyScreenshotProvider {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.screenshotProvider
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
	s.StopBackground()
	if s.screenshotProvider != nil {
		_ = s.screenshotProvider.Close()
	}
	if s.isRecording {
		return s.StopRecording()
	}
	return nil
}

// StartBackground 启动后台截图线程，interval 控制截图频率（建议 200-300ms）。
// 启动后 Screenshot() 直接返回最新帧，不再阻塞等待。
func (s *ScreenCapture) StartBackground(ctx context.Context, interval time.Duration) {
	if s.bgCancel != nil {
		return // 已启动
	}
	ctx, s.bgCancel = context.WithCancel(ctx)
	s.bgWg.Add(1)
	go s.bgLoop(ctx, interval)
	logger.Infof("后台截图已启动 (间隔=%v)", interval)
}

// StopBackground 停止后台截图线程。
func (s *ScreenCapture) StopBackground() {
	if s.bgCancel != nil {
		s.bgCancel()
		s.bgCancel = nil
	}
	s.bgWg.Wait()
}

func (s *ScreenCapture) bgLoop(ctx context.Context, interval time.Duration) {
	defer s.bgWg.Done()
	s.bgCaptureOnce()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.bgCaptureOnce()
		}
	}
}

func (s *ScreenCapture) bgCaptureOnce() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// 优先取 scrcpy 缓存帧（几乎零延迟），不阻塞
	if s.screenshotProvider != nil && s.screenshotProvider.IsActive() {
		if data := s.screenshotProvider.tryCachedFrameAnyAge(); data != nil {
			s.bgLatest.Store(&bgFrame{data: data, ts: time.Now()})
			return
		}
	}

	// scrcpy 无帧时降级 ADB（较慢，但兜底）
	data, err := s.screenshotDirect(ctx)
	if err != nil || len(data) == 0 {
		return
	}
	s.bgLatest.Store(&bgFrame{data: data, ts: time.Now()})
}
