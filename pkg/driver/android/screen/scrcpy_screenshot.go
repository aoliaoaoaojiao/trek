package screen

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
	"trek/logger"
	"trek/pkg/driver/android/adb"
)

// ScreenshotMode 控制截图获取策略。
type ScreenshotMode int

const (
	// ScreenshotModeADB 始终使用 ADB screencap。
	ScreenshotModeADB ScreenshotMode = iota
	// ScreenshotModeScrcpy 使用 scrcpy 流截图（如果可用）。
	ScreenshotModeScrcpy
	// ScreenshotModeAuto 优先 scrcpy，回退 ADB。
	ScreenshotModeAuto
)

// ScrcpyScreenshotConfig 配置 scrcpy 截图提供者。
type ScrcpyScreenshotConfig struct {
	Mode        ScreenshotMode   // 获取模式
	MaxSize     int              // scrcpy max_size（0 = 不缩放，使用原始分辨率）
	MaxFPS      int              // scrcpy max_fps（默认 10）
	CacheTTL    time.Duration    // 帧缓存 TTL（默认 200ms）
	IdleTimeout time.Duration    // 空闲断开超时（默认 30s）
	FFmpegPath  string           // ffmpeg 路径（空 = 在 PATH 中查找）
}

// DefaultScrcpyScreenshotConfig 返回默认配置。
// 优先读取环境变量 FFMPEG_PATH（或 TREK_FFMPEG_PATH），
// 如未设置则在 PATH 中查找 ffmpeg。
func DefaultScrcpyScreenshotConfig() ScrcpyScreenshotConfig {
	ffmpegPath := os.Getenv("FFMPEG_PATH")
	if ffmpegPath == "" {
		ffmpegPath = os.Getenv("TREK_FFMPEG_PATH")
	}
	return ScrcpyScreenshotConfig{
		Mode:        ScreenshotModeAuto,
		MaxSize:     0,
		MaxFPS:      10,
		CacheTTL:    500 * time.Millisecond,
		IdleTimeout: 30 * time.Second,
		FFmpegPath:  ffmpegPath,
	}
}

// ScrcpyScreenshotProvider 通过 scrcpy H264 流提供高速截图。
// 维护独立于录制的轻量级 scrcpy 连接，使用帧缓存避免重复 ADB 调用。
type ScrcpyScreenshotProvider struct {
	device *adb.Device
	config ScrcpyScreenshotConfig

	mu sync.Mutex

	// scrcpy 状态
	scrcpy     *Scrcpy
	cancelFunc context.CancelFunc

	// 配置和数据包缓存
	configPkt []byte // 最新的 H264 配置包（SPS/PPS）

	// 帧缓存
	cache struct {
		sync.RWMutex
		frame  []byte    // 解码后的 PNG 字节
		ts     time.Time // 缓存时间戳
		width  int       // 帧宽度
		height int       // 帧高度
	}

	// 空闲管理
	lastRequest time.Time
	stopIdle    chan struct{}
	idleWg      sync.WaitGroup

	// 新帧通知：onFrame 解码出新帧时发送，Screenshot 等待接收
	keyframeNotify chan struct{}

	// lava 状态
	started  bool
	fallback bool // scrcpy 永久不可用
}

// NewScrcpyScreenshotProvider 创建 scrcpy 截图提供者。
func NewScrcpyScreenshotProvider(device *adb.Device, config ScrcpyScreenshotConfig) *ScrcpyScreenshotProvider {
	return &ScrcpyScreenshotProvider{
		device:      device,
		config:      config,
		stopIdle:    make(chan struct{}),
		lastRequest: time.Now(),
	}
}

// Start 初始化 scrcpy 服务器并开始接收 H264 帧。
func (p *ScrcpyScreenshotProvider) Start() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.started || p.fallback {
		return nil
	}

	sc := NewScrcpy(p.device)
	if sc == nil {
		p.fallback = true
		return fmt.Errorf("创建 scrcpy 实例失败")
	}

	sc.SetVideoFrameHandler(func(data []byte, pts uint64, isKeyFrame bool, isConfig bool) {
		p.onFrame(data, pts, isKeyFrame, isConfig)
	})

	maxSize := p.config.MaxSize
	// maxSize=0 表示不缩放（scrcpy 协议原生支持）
	// 用户如需限制分辨率，在配置中设置 MaxSize>0

	if err := sc.Start(maxSize); err != nil {
		p.fallback = true
		logger.Warnf("scrcpy 截图模式启动失败，将回退 ADB: %v", err)
		return err
	}

	p.scrcpy = sc
	ctx, cancel := context.WithCancel(context.Background())
	p.cancelFunc = cancel
	p.started = true
	p.keyframeNotify = make(chan struct{}, 1)

	// 启动空闲监控
	p.idleWg.Add(1)
	go p.idleMonitor(ctx)

	logger.Infof("scrcpy 高速截图模式已启动 (max_size=%d, max_fps=%d)", maxSize, p.config.MaxFPS)
	return nil
}

// onFrame 处理从 scrcpy 流接收到的 H264 帧。
func (p *ScrcpyScreenshotProvider) onFrame(data []byte, pts uint64, isKeyFrame bool, isConfig bool) {
	// 配置包：仅在数据是 Annex B 格式（以 00 00 00 01 开头）时存储
	// scrcpy v3.3+ 的 config 包可能是 codec metadata（全零），不是 H.264 SPS/PPS
	if isConfig && len(data) > 4 && data[0] == 0x00 && data[1] == 0x00 && data[2] == 0x00 && data[3] == 0x01 {
		p.mu.Lock()
		p.configPkt = append([]byte{}, data...)
		p.mu.Unlock()
		return
	}

	// 关键帧中也检测 SPS/PPS（作为兜底 configPkt 来源）
	if isKeyFrame && len(data) > 4 && data[0] == 0x00 && data[1] == 0x00 && data[2] == 0x00 && data[3] == 0x01 {
		nalType := data[4] & 0x1F
		if nalType == 7 || nalType == 8 { // SPS=7, PPS=8
			// 提取从开头到第一个非 SPS/PPS NAL 之前的数据作为 configPkt
			configEnd := 0
			for i := 0; i+4 <= len(data); i++ {
				if data[i] == 0x00 && data[i+1] == 0x00 && data[i+2] == 0x00 && data[i+3] == 0x01 {
					nal := data[i+4] & 0x1F
					if nal != 7 && nal != 8 { // 非 SPS/PPS，找到了帧数据起点
						configEnd = i
						break
					}
				}
			}
			if configEnd > 0 {
				p.mu.Lock()
				p.configPkt = append([]byte{}, data[:configEnd]...)
				p.mu.Unlock()
			}
		}
	}

	if isKeyFrame {
		p.mu.Lock()
		decoded, err := p.decodeH264ToPNG(data)
		if err == nil && len(decoded) > 0 {
			// 获取尺寸信息
			img, _, err := image.Decode(bytes.NewReader(decoded))
			if err == nil {
				bounds := img.Bounds()
				p.cache.Lock()
				p.cache.frame = decoded
				p.cache.ts = time.Now()
				p.cache.width = bounds.Dx()
				p.cache.height = bounds.Dy()
				p.cache.Unlock()
				// 通知等待新帧的 Screenshot 调用
				select {
				case p.keyframeNotify <- struct{}{}:
				default:
				}
			}
		}
		p.mu.Unlock()
	}
}

// decodeH264ToPNG 使用 ffmpeg 子进程将 H264 数据解码为 PNG。
// 如果配置包可用，将其前置到关键帧数据前以提高解码成功率。
// ffmpeg 路径优先级：config.FFmpegPath > 环境变量 FFMPEG_PATH/TREK_FFMPEG_PATH > PATH 查找
func (p *ScrcpyScreenshotProvider) decodeH264ToPNG(h264Data []byte) ([]byte, error) {
	ffmpegPath := p.config.FFmpegPath
	if ffmpegPath == "" {
		ffmpegPath = os.Getenv("FFMPEG_PATH")
	}
	if ffmpegPath == "" {
		ffmpegPath = os.Getenv("TREK_FFMPEG_PATH")
	}
	if ffmpegPath == "" {
		var err error
		ffmpegPath, err = exec.LookPath("ffmpeg")
		if err != nil {
			return nil, fmt.Errorf("ffmpeg 未找到: %w", err)
		}
	}

	// 构建输入数据：如果可用则前置配置包
	var inputData []byte
	if len(p.configPkt) > 0 {
		inputData = append([]byte{}, p.configPkt...)
		inputData = append(inputData, h264Data...)
	} else {
		inputData = h264Data
	}

	// ffmpeg -f h264 -i pipe:0 -vframes 1 -f image2pipe -vcodec png -
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, ffmpegPath,
		"-f", "h264",
		"-i", "pipe:0",
		"-vframes", "1",
		"-f", "image2pipe",
		"-vcodec", "png",
		"-loglevel", "error",
		"pipe:1",
	)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("创建 ffmpeg stdin pipe 失败: %w", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("启动 ffmpeg 失败: %w", err)
	}

	// 写入 H264 数据并关闭 stdin
	_, _ = stdin.Write(inputData)
	stdin.Close()

	if err := cmd.Wait(); err != nil {
		if stderr.Len() > 0 {
			return nil, fmt.Errorf("ffmpeg 解码失败: %s", strings.TrimSpace(stderr.String()))
		}
		return nil, fmt.Errorf("ffmpeg 解码失败: %w", err)
	}

	if stdout.Len() == 0 {
		return nil, fmt.Errorf("ffmpeg 未输出任何图像数据")
	}

	return stdout.Bytes(), nil
}

// Screenshot 返回 PNG 格式的截图。
// 参考 Midscene 的策略：先等新关键帧（300ms），超时再用缓存帧。
func (p *ScrcpyScreenshotProvider) Screenshot(ctx context.Context) ([]byte, error) {
	p.lastRequest = time.Now()

	// 如果未启动 scrcpy，尝试启动
	if !p.started && !p.fallback {
		if err := p.Start(); err != nil {
			p.fallback = true
			logger.Warnf("scrcpy 启动失败，降级到 ADB: %v", err)
		}
	}

	// 核心路径：等待新关键帧到达（参考 Midscene waitForNextKeyframe）
	if p.started && !p.fallback && p.keyframeNotify != nil {
		select {
		case <-p.keyframeNotify:
			// 收到新帧通知，返回最新缓存
			if data := p.tryCachedFrameAnyAge(); data != nil {
				logger.Debugf("scrcpy 等到新帧，返回最新帧 (%d 字节)", len(data))
				return data, nil
			}
		case <-time.After(300 * time.Millisecond):
			// 超时，屏幕可能静止，返回缓存帧
			logger.Debugf("scrcpy 等新帧超时(300ms)，返回缓存帧")
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	// 兜底：返回任意缓存帧
	if data := p.tryCachedFrameAnyAge(); data != nil {
		return data, nil
	}

	// 无缓存帧，降级 ADB
	logger.Debug("scrcpy 无缓存帧可用，降级到 ADB")
	return p.device.Screenshot(ctx)
}

// tryCachedFrame 尝试从帧缓存返回（Tier 1 最快路径）。
func (p *ScrcpyScreenshotProvider) tryCachedFrame() []byte {
	p.cache.RLock()
	defer p.cache.RUnlock()
	if p.cache.frame != nil && time.Since(p.cache.ts) < p.config.CacheTTL {
		frame := make([]byte, len(p.cache.frame))
		copy(frame, p.cache.frame)
		logger.Debugf("scrcpy 截图返回缓存帧 (%d 字节, 已缓存 %v)", len(frame), time.Since(p.cache.ts))
		return frame
	}
	return nil
}

// tryCachedFrameAnyAge 返回任意年龄的缓存帧（不检查 TTL），供后台线程快速取帧。
func (p *ScrcpyScreenshotProvider) tryCachedFrameAnyAge() []byte {
	p.lastRequest = time.Now() // 更新活跃时间，防止 idle 断开
	p.cache.RLock()
	defer p.cache.RUnlock()
	if p.cache.frame == nil {
		return nil
	}
	frame := make([]byte, len(p.cache.frame))
	copy(frame, p.cache.frame)
	return frame
}

// tryScrcpyFrameFast 快速等待 scrcpy 新帧（150ms 超时），用于有缓存帧时的快速检测。
func (p *ScrcpyScreenshotProvider) tryScrcpyFrameFast() []byte {
	p.mu.Lock()
	sc := p.scrcpy
	p.mu.Unlock()
	if sc == nil {
		return nil
	}

	deadline := time.Now().Add(150 * time.Millisecond)
	for time.Now().Before(deadline) {
		p.cache.RLock()
		fresh := p.cache.frame != nil && time.Since(p.cache.ts) < 100*time.Millisecond
		if fresh {
			frame := make([]byte, len(p.cache.frame))
			copy(frame, p.cache.frame)
			p.cache.RUnlock()
			logger.Debugf("scrcpy 快速检测到新帧 (%d 字节)", len(frame))
			return frame
		}
		p.cache.RUnlock()
		time.Sleep(20 * time.Millisecond)
	}
	return nil
}

// tryScrcpyFrame 等待 scrcpy 流中的新关键帧并解码（Tier 1 次快路径，500ms 超时）。
func (p *ScrcpyScreenshotProvider) tryScrcpyFrame() []byte {
	p.mu.Lock()
	sc := p.scrcpy
	p.mu.Unlock()
	if sc == nil {
		return nil
	}

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		p.cache.RLock()
		fresh := p.cache.frame != nil && time.Since(p.cache.ts) < 100*time.Millisecond
		if fresh {
			frame := make([]byte, len(p.cache.frame))
			copy(frame, p.cache.frame)
			p.cache.RUnlock()
			logger.Debugf("scrcpy 截图获取新帧 (%d 字节)", len(frame))
			return frame
		}
		p.cache.RUnlock()
		time.Sleep(20 * time.Millisecond)
	}
	return nil
}

// tryFastADB 使用较短超时的 ADB 截图（Tier 2）。
// 相当于 Midscene 的 appium-adb takeScreenshot() 层。
func (p *ScrcpyScreenshotProvider) tryFastADB(ctx context.Context) []byte {
	fastCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	data, err := p.device.Screenshot(fastCtx)
	if err == nil && len(data) > 0 {
		logger.Debugf("ADB 快速截图成功 (%d 字节)", len(data))
		return data
	}
	logger.Debugf("ADB 快速截图失败: %v", err)
	return nil
}

// FrameSize 返回缓存的帧尺寸（可能为 0,0 如果无缓存）。
func (p *ScrcpyScreenshotProvider) FrameSize() (width, height int) {
	p.cache.RLock()
	defer p.cache.RUnlock()
	return p.cache.width, p.cache.height
}

// LastFrameAge 返回缓存帧的年龄。
func (p *ScrcpyScreenshotProvider) LastFrameAge() time.Duration {
	p.cache.RLock()
	defer p.cache.RUnlock()
	if p.cache.frame == nil {
		return -1
	}
	return time.Since(p.cache.ts)
}

// idleMonitor 在空闲超过 IdleTimeout 后断开 scrcpy 连接以节省资源。
func (p *ScrcpyScreenshotProvider) idleMonitor(ctx context.Context) {
	defer p.idleWg.Done()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-p.stopIdle:
			return
		case <-ticker.C:
			if time.Since(p.lastRequest) > p.config.IdleTimeout {
				p.mu.Lock()
				if p.scrcpy != nil {
					p.scrcpy.exitCallBackFunc()
					p.scrcpy = nil
					p.started = false
					logger.Debug("scrcpy 截图连接因空闲已断开")
				}
				p.mu.Unlock()
			}
		}
	}
}

// Close 关闭 scrcpy 连接并释放资源。
func (p *ScrcpyScreenshotProvider) Close() error {
	close(p.stopIdle)
	p.idleWg.Wait()

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cancelFunc != nil {
		p.cancelFunc()
	}
	if p.scrcpy != nil {
		p.scrcpy.exitCallBackFunc()
	}
	p.started = false
	return nil
}

// IsActive 返回 scrcpy 截图提供者是否可用。
func (p *ScrcpyScreenshotProvider) IsActive() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.started && !p.fallback
}

// IsFallback 返回是否已回退 ADB 模式。
func (p *ScrcpyScreenshotProvider) IsFallback() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.fallback
}
