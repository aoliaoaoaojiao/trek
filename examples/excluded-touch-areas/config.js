/// <reference path="../../assets/goja/trek-plugin.d.ts" />

const config = {
  page_source: "uia",
  page_name_strategy: "structure_fingerprint",
  capture_screenshot: true,
  keep_step_records: true,

  /**
   * 屏蔽页面区域配置示例
   *
   * excluded_touch_areas 用于屏蔽不该触碰的区域，如：
   * - 系统状态栏
   * - 底部导航栏
   * - 广告位
   * - 悬浮按钮
   * - 弹窗关闭按钮等
   *
   * 当动作坐标落在排除矩形内时，该动作会被跳过。
   *
   * 坐标系自动判断：
   * - 如果 bounds 所有值都 < 1，则认为是归一化坐标（0~1）
   * - 如果 bounds 任何值 >= 1，则认为是像素坐标
   *
   * 配置说明：
   * - page_name: 页面名（与遍历日志中 page= 输出一致）
   *   - 使用 structure_fingerprint 策略时，格式为 "XMLPage:xxxxxx"
   *   - 使用 activity_only 策略时，格式为 Activity 名（如 "com.example/.MainActivity"）
   *   - 使用 image_fingerprint 策略时，格式为 "IMGPage:xxxxxx"
   * - bounds: 排除矩形 [left, top, right, bottom]
   *   - 像素坐标：如 [0, 0, 1080, 100]
   *   - 归一化坐标：如 [0, 0, 1.0, 0.052]（适用于不同分辨率设备）
   *
   * 提示：
   * 1. 可在 Web 配置界面的"界面截图"区域点击截图查看坐标
   * 2. 同一页面可配置多条排除规则
   * 3. 不同页面可配置不同的排除区域
   * 4. 推荐使用归一化坐标，方便跨设备复用
   */
  excluded_touch_areas: [
    // ===== 像素坐标示例（适合固定分辨率设备） =====

    // 示例1：屏蔽系统状态栏（顶部 100px）
    { page_name: "XMLPage:a1b2c3d4e5f6g7h8", bounds: [0, 0, 1080, 100] },

    // 示例2：屏蔽底部导航栏
    { page_name: "XMLPage:a1b2c3d4e5f6g7h8", bounds: [0, 1820, 1080, 1920] },

    // ===== 归一化坐标示例（适合不同分辨率设备） =====

    // 示例3：屏蔽系统状态栏（顶部约 5.2%）
    { page_name: "XMLPage:a1b2c3d4e5f6g7h8", bounds: [0, 0, 1.0, 0.052] },

    // 示例4：屏蔽底部导航栏（底部约 5.2%）
    { page_name: "XMLPage:a1b2c3d4e5f6g7h8", bounds: [0, 0.948, 1.0, 1.0] },

    // 示例5：屏蔽广告位（底部 10%~20% 区域）
    { page_name: "XMLPage:a1b2c3d4e5f6g7h8", bounds: [0, 0.8, 1.0, 0.9] },

    // 示例6：屏蔽悬浮按钮（右下角 83%~100% 区域）
    { page_name: "XMLPage:a1b2c3d4e5f6g7h8", bounds: [0.83, 0.78, 1.0, 0.89] },

    // 示例7：为不同页面配置不同的排除区域
    { page_name: "XMLPage:x9y8z7w6v5u4t3s2", bounds: [0.83, 0, 1.0, 0.05] },
  ],
}
