package vision

import (
	"context"
	"fmt"

	"trek/internal/engine/decision"
	"trek/internal/vision/coord"
	"trek/internal/vision/imageproc"
)

// PerceptorConfig 配置视觉感知器的行为。
type PerceptorConfig struct {
	// VLM 图像优化配置
	VLMConfig imageproc.VLMConfig

	// 截图标注
	EnableAnnotation bool // 是否在发送给 VLM 前标注元素编号
	FontScale        int  // 标注字体缩放（默认 2）

	// 设备逻辑尺寸（用于 DPR 计算，0 = 不计算）
	LogicalWidth  int
	LogicalHeight int

	// 设备物理尺寸（用于 DPR 计算，0 = 使用截图尺寸）
	DeviceWidth  int
	DeviceHeight int
}

// DefaultPerceptorConfig 返回默认配置。
func DefaultPerceptorConfig() PerceptorConfig {
	return PerceptorConfig{
		VLMConfig:   imageproc.DefaultVLMConfig(),
		FontScale:   2,
		LogicalWidth:  0,
		LogicalHeight: 0,
	}
}

// Perceptor 是增强后的视觉感知模块。
// 编排完整截图处理管线：格式检测 → VLM 优化 → 坐标变换。
type Perceptor struct {
	config PerceptorConfig
}

// NewPerceptor 创建视觉感知器。
func NewPerceptor(cfg PerceptorConfig) *Perceptor {
	return &Perceptor{config: cfg}
}

// Observe 实现 decision.Perceptor 接口。
// 处理流程：
//  1. 验证截图非空
//  2. 使用 imageproc.OptimizeForVLM 优化截图（缩放 + 格式转换）
//  3. 计算 DPR（如果提供了逻辑尺寸）
//  4. 返回包含优化后截图的 Observation
func (p *Perceptor) Observe(ctx context.Context, input decision.PerceptionInput) (*decision.Observation, error) {
	_ = ctx
	if len(input.Screenshot) == 0 {
		return nil, fmt.Errorf("vision perception requires screenshot")
	}

	// 处理截图
	processed, err := p.ProcessScreenshot(input.Screenshot)
	if err != nil {
		// 处理失败时返回原始截图
		return &decision.Observation{
			PageName:   input.PageName,
			XMLDesc:    input.XMLDesc,
			Screenshot: input.Screenshot,
			Element:    nil,
		}, nil
	}

	return &decision.Observation{
		PageName:   input.PageName,
		XMLDesc:    input.XMLDesc,
		Screenshot: processed.Data,
		Element:    nil,
	}, nil
}

// ProcessedScreenshot 保存截图处理管线的输出。
type ProcessedScreenshot struct {
	Data       []byte         // 最终图像字节（已优化/VLM 就绪）
	MediaType  string         // MIME 类型
	OrigWidth  int            // 原始截图宽度
	OrigHeight int            // 原始截图高度
	ShotWidth  int            // 处理后的图像宽度（缩放后）
	ShotHeight int            // 处理后的图像高度（缩放后）
	DPR        coord.DPRInfo  // 设备像素比信息
}

// ProcessScreenshot 执行完整的截图处理管线：
//   - DetectFormat → OptimizeForVLM → DPR 计算
func (p *Perceptor) ProcessScreenshot(screenshot []byte) (*ProcessedScreenshot, error) {
	if len(screenshot) == 0 {
		return nil, fmt.Errorf("empty screenshot")
	}

	// 检测格式
	imgFmt := imageproc.DetectFormat(screenshot)

	// VLM 优化
	optimized, origW, origH, shotW, shotH, err := imageproc.OptimizeForVLM(screenshot, p.config.VLMConfig)
	if err != nil {
		return nil, fmt.Errorf("VLM optimization failed: %w", err)
	}

	// 确定媒体类型
	mediaType := imgFmt.MediaType()
	// 如果被缩放或转码了，更新媒体类型
	if origW != shotW || origH != shotH || imgFmt != p.config.VLMConfig.Format {
		mediaType = p.config.VLMConfig.Format.MediaType()
	}
	if mediaType == "" {
		mediaType = p.config.VLMConfig.Format.MediaType()
	}

	// DPR 计算
	dprInfo := coord.DPRInfo{
		ScreenshotWidth:  shotW,
		ScreenshotHeight: shotH,
		LogicalWidth:     p.config.LogicalWidth,
		LogicalHeight:    p.config.LogicalHeight,
		DeviceWidth:      p.config.DeviceWidth,
		DeviceHeight:     p.config.DeviceHeight,
	}
	if dprInfo.DeviceWidth <= 0 {
		dprInfo.DeviceWidth = origW
	}
	if dprInfo.DeviceHeight <= 0 {
		dprInfo.DeviceHeight = origH
	}

	return &ProcessedScreenshot{
		Data:       optimized,
		MediaType:  mediaType,
		OrigWidth:  origW,
		OrigHeight: origH,
		ShotWidth:  shotW,
		ShotHeight: shotH,
		DPR:        dprInfo,
	}, nil
}
