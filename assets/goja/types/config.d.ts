/**
 * Trek 配置相关类型定义
 */

declare namespace Trek {
  interface UCTBanditConfig {
    /**
     * 两状态循环惩罚系数。
     * 检测到 A→B→A 循环模式时，将此值作为负奖励加到累计回报上，
     * 使导致循环的动作更不容易被再次选中。默认 -3.0
     */
    two_state_loop_penalty?: number
    /**
     * 边重复惩罚系数。
     * 当同一条状态转移边（A→B）被重复经过超过 edge_repeat_threshold 次后，
     * 每多一次施加此惩罚：penalty * (重复次数 - 阈值)。默认 -1.0
     */
    edge_repeat_penalty?: number
    /**
     * 边重复惩罚触发阈值。
     * 状态转移边可重复经过此次数后才开始施加 edge_repeat_penalty。默认 2
     */
    edge_repeat_threshold?: number
    /**
     * 动作冷却惩罚系数。
     * 对同一页面上的同一动作，近期每重复选择一次就累加此惩罚，
     * 降低短期内重复执行同一动作的概率，鼓励探索不同控件。默认 0.8
     */
    action_cooldown_penalty?: number
    /**
     * 近期动作窗口大小。
     * 记录最近 N 次（页面, 动作）选择，用于计算 action_cooldown_penalty。
     * 窗口越大，"记忆"越长。默认 6
     */
    recent_action_window?: number
    /**
     * 循环逃逸探索加成系数。
     * 当检测到近期状态历史存在 A→B→A→B 循环模式时，
     * 将 epsilon-greedy 探索概率提升此值，使引擎更倾向于随机选择不同动作以逃出循环。
     * 探索概率上限为 0.9。默认 0.25
     */
    loop_escape_explore_boost?: number
  }

  interface PageExcludedArea {
    /** 页面名（与遍历日志中 page= 输出一致） */
    page_name: string
    /** 排除矩形 [left, top, right, bottom]，像素坐标 */
    bounds: Bounds
  }

  export interface StaticConfig {
    /**
     * 触摸排除区域列表，点击坐标落在矩形内的动作会被跳过。
     * 用于屏蔽广告位、系统导航栏、悬浮按钮等不该触碰的区域。
     * 同一页面可配置多条。
     *
     * @example
     * ```js
     * excluded_touch_areas: [
     *   { page_name: "MainActivity", bounds: [0, 0, 1080, 100] },   // 顶部状态栏
     *   { page_name: "WebView",      bounds: [0, 900, 1080, 1080] } // 底部广告
     * ]
     * ```
     */
    excluded_touch_areas?: PageExcludedArea[]
    /**
     * 是否跳过内置决策算法。
     * - false（默认）：引擎先调用插件 beforeDecide，若插件未拦截，则走内置算法（Reuse/UctBandit/Random）产出动作。
     * - true：内置算法完全不执行，引擎仅依赖插件 beforeDecide 提供动作；
     *         若插件也未返回动作，则引擎输出 NOP（空操作）。
     * 适用于插件全权控制决策的场景（如纯脚本策略、外部策略引擎驱动）。
     */
    skip_all_actions_from_model?: boolean
    /** 遍历算法：random / reuse / uct_bandit。默认 "reuse" */
    algorithm?: AlgorithmType
    /** 外部插件脚本路径列表 */
    plugins?: string[]
    /** 指定 monkey 运行使用的页面源类型。默认 "uia"，可选 "uia" / "poco" / "screenshot"。选择 "screenshot" 时会按截图驱动页面识别，并默认开启每步截图。 */
    page_source?: PageSourceType
    /** 混合模式。设为 true 时，同时获取结构化 XML（来自 uia/poco）+ 截图，适合配合 chain 策略使用。仅在 page_source="uia" 或 "poco" 时生效。 */
    mixed_mode?: boolean
    /** 指定 monkey 运行使用的触控模式。默认 "motion" */
    touch_mode?: TouchMode
    /** 指定页面名生成策略（不填时默认使用 structure_fingerprint） */
    page_name_strategy?: PageNameStrategy
    /**
     * 页面控件信息获取策略。
     * `raw` 直接使用 dump 原始 XML；
     * `ocr` 基于截图 OCR 提取控件区域并生成伪控件树；
     * `llm` 基于截图 LLM 推断控件区域并生成伪控件树；
     * `chain` 兜底模式：有 raw XML 直接用，否则查缓存，缓存未命中时 OCR+LLM 并行调用合并结果。
     * 当策略为 `ocr`、`llm` 或 `chain` 时，运行期会自动启用截图采集，
     * 并按截图图片指纹缓存伪控件树；相同图片优先复用缓存，仅首次出现时才调用 OCR/LLM。
     * 同一图片连续命中缓存达到阈值后会自动重新识别一次；阻塞恢复路径也会强制刷新，避免长期复用过期控件树。
     */
    page_control_strategy?: PageControlStrategy
    /** 是否采集截图给决策层。默认 false；当 page_source="screenshot" 或页面理解策略不是 raw 时会自动开启。 */
    capture_screenshot?: boolean
    /** 是否保留每步记录。默认 true */
    keep_step_records?: boolean
    /**
     * OCR 探索超时（毫秒）。
     * Explore 模式下，引擎可将截图发送给外部 OCR 服务（如 PaddleOCR）识别可点击文本区域作为候选动作。
     * 此字段控制该 HTTP 请求的超时时间。默认 10000
     */
    explore_ocr_timeout_ms?: number
    /**
     * LLM 超时（毫秒）。
     * 仅用于 `page_control_strategy="llm"` 时的页面控件检测请求超时控制。默认 15000
     */
    llm_timeout_ms?: number
    /**
     * LLM 响应缓存 TTL（秒）。
     * 缓存"页面签名+阻塞原因 → LLM 候选"的映射，避免同一页面重复调用 LLM。默认 300（5 分钟）
     */
    plan_cache_ttl_seconds?: number
    /**
     * 恢复冷却步数。
     * 恢复动作成功脱困后，状态机进入冷却模式持续此步数，期间不会再触发恢复，
     * 防止刚脱困就立即再次进入恢复。默认 2
     */
    recovery_cooldown_steps?: number
    /**
     * 两状态循环检测阈值。
     * 引擎跟踪页面签名跳转，当检测到连续 A→B→A→B 模式达到此次数时，
     * 判定为 two_state_ping_pong 卡死并触发恢复。默认 2
     */
    recovery_two_state_loop_threshold?: number
    /**
     * 高访问页面阈值。
     * 当某页面访问次数 >= 此值，且近期窗口内唯一页面数 <= 2 时，
     * 判定为 high_visit_low_reward 卡死并触发恢复。默认 8
     */
    recovery_high_visit_threshold?: number
    /**
     * 低奖励窗口大小。
     * 配合 recovery_high_visit_threshold 使用：检查最近 N 步的页面签名，
     * 若唯一页面数 <= 2 且当前页面访问次数达标，则判定卡死。默认 6
     */
    recovery_low_reward_window?: number
    /**
     * 候选歧义顶部间距阈值。
     * 当前仅用于保留探索阶段的歧义度判定配置，不会再触发内置 LLM 增强。默认 0.15
     */
    candidate_ambiguity_top_gap_threshold?: number
    /**
     * 高价值页面访问上限。
     * 当前仅用于保留探索阶段的页面价值判定配置，不会再触发内置 LLM 候选增强。默认 2
     */
    high_value_page_visit_limit?: number
    /**
     * 候选风险下降阈值。
     * 候选融合阶段，RiskScore >= 此值的候选动作会被直接丢弃。
     * 已知失败的动作风险分约 2.0，设为 2.1 可过滤掉它们。默认 2.1
     */
    candidate_risk_drop_threshold?: number
    /**
     * 候选最小融合分数。
     * 候选融合分数 = Confidence + EscapeScore + NoveltyScore*0.5 - RiskScore，
     * 低于此值的候选被过滤。默认 -0.3
     */
    candidate_min_fusion_score?: number
    /**
     * 滚动推断阈值。
     * 当 UI 元素的可点击子节点数 >= 此值时，自动推断为可滚动容器。
     * 默认 5，设为 0 表示关闭推断。
     * 值越小越激进，更容易把容器识别成可滚动；值越大越保守，误判更少但也更容易漏掉真实可滚动区域。
     * raw 模式下最有效，ocr 模式下仍可生效但精度较弱；llm 模式下当前已禁用此推断。
     * 主要用于解决 Poco/Unity 游戏 UI 未声明 ScrollRect 的场景。
     */
    scroll_infer_threshold?: number
    /** UIA 端口相关配置 */
    uia?: {
      /** 设备端 UIA server 端口。默认 6790 */
      server_port?: number
    }
    /** Poco 页面源配置 */
    poco?: {
      /** Poco 引擎类型（决定协议与默认端口） */
      engine?: PocoEngine
      /** Poco 远端端口；不填时可按引擎默认端口推导 */
      port?: number
    }
    log?: {
      /** 文件日志级别；控制台日志级别由命令行 -log-level 控制 */
      file_level?: LogLevel
    }
    /** 有效触控区域映射（坐标映射公式：x' = left + (right-left) * x）。range 默认 {left:0, top:0, right:1, bottom:1} */
    effective_touch_area?: {
      /** 命中设备序列号（可选，空表示不限制设备） */
      serial?: string
      /** 命中包名（可选，空表示不限制包名） */
      package_name?: string
      range: {
        left: number
        top: number
        right: number
        bottom: number
      }
    }
    /** UCT Bandit 算法参数 */
    uct_bandit?: UCTBanditConfig
    /**
     * 模型族，影响 VLM 坐标格式适配和 prompt 优化。
     * - "claude": 默认，[0,1000] 归一化坐标
     * - "gpt": [0,1000] 归一化坐标
     * - "gemini": [y1,x1,y2,x2] 轴交换，自动转换
     * - "qwen": 原始像素坐标，自动归一化
     */
    model_family?: "claude" | "gpt" | "gemini" | "qwen" | "doubao" | "glm" | "autoglm"

    /** DeepLocate 两阶段定位配置 */
    deep_locate?: {
      /** 启用/关闭 DeepLocate。默认 true */
      enabled?: boolean
      /** 区域扩展像素数。默认 100 */
      section_expand_px?: number
      /** 区域最小尺寸。默认 400 */
      section_min_size?: number
      /** 第二阶段放大倍数。默认 2 */
      zoom_factor?: number
    }

    /** VLM 截图编号标注配置 */
    vlm_annotation?: {
      /** 启用/关闭编号标注。默认 false */
      enabled?: boolean
      /** 编号字体缩放。默认 2 */
      font_scale?: number
    }
  }
}
