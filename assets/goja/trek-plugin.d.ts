/**
 * Trek Goja 策略插件类型声明
 * 用法：在插件脚本顶部添加
 * /// <reference path="./trek-plugin.d.ts" />
 */

declare namespace Trek {
  // ── 动作相关 ──────────────────────────────────────────────────

  type ActionType =
    | "NOP"
    | "BACK"
    | "CLICK"
    | "LONG_CLICK"
    | "SCROLL_TOP_DOWN"
    | "SCROLL_BOTTOM_UP"
    | "SCROLL_LEFT_RIGHT"
    | "SCROLL_RIGHT_LEFT"
    | "SCROLL_BOTTOM_UP_N"
    | "START"
    | "RESTART"
    | "CLEAN_RESTART"
    | "ACTIVATE"

  type ScrollDirection =
    | "top_down"
    | "bottom_up"
    | "left_right"
    | "right_left"
    | "bottom_up_n"

  type Bounds = [number, number, number, number]

  interface Action {
    /** 动作类型 */
    type: ActionType
    /** 动作作用区域（点击/滑动等） */
    bounds?: Bounds
    /** 输入文本（点击输入场景） */
    text?: string
    /** 输入前是否清空 */
    clear?: boolean
    /** 是否走 adb 输入模式 */
    adb_input?: boolean
    /** 是否允许 fuzzing */
    allow_fuzzing?: boolean
    /** 动作节流（毫秒） */
    throttle?: number
    /** 动作后等待时长（毫秒） */
    wait_time?: number
  }

  // ── 页面相关 ──────────────────────────────────────────────────

  interface Screenshot {
    /** PNG/JPEG 原始字节 */
    bytes: Uint8Array
    /** MIME 类型 */
    mime: "image/png" | "image/jpeg"
    /** 字节数 */
    size: number
    /** 图片宽度 */
    width?: number
    /** 图片高度 */
    height?: number
  }

  interface PageNode {
    /** 节点文本（text） */
    text: string
    /** 资源 ID（resource-id） */
    resource_id: string
    /** 无障碍描述（content-desc） */
    content_desc: string
    /** 节点类名（class） */
    class_name: string
    /** 节点边界 [left, top, right, bottom] */
    bounds: Bounds
    /** 是否可点击 */
    clickable: boolean
    /** 是否可用 */
    enabled: boolean
    /** 是否可编辑输入 */
    editable: boolean
    /** 节点的标准 XPath（用于跨模块定位和调试） */
    xpath?: string
  }

  /** 页面快照：由引擎采集并传给插件。 */
  interface PageSnapshot {
    /**
     * 当前页面名。
     * 当 page_source_type="uia" 时，默认优先使用 `dumpsys activity top` 解析出的 Activity 名。
     * 插件可在 `transformPage` 中返回 `page_name` 进行覆盖。
     */
    page_name: string
    /** 当前页面 XML（插件可在 transformPage 中返回 xml 覆盖） */
    xml: string
    /** 截图（需运行时开启截图采集） */
    screenshot?: Screenshot
    /** 从 XML 提取的节点列表，便于脚本筛选控件 */
    nodes: PageNode[]
  }

  interface PageInfo {
    /** 覆盖页面名；不返回则沿用原值 */
    page_name?: string
    /** 覆盖页面 XML；不返回则沿用原值 */
    xml?: string
  }

  // ── 运行时上下文 ─────────────────────────────────────────────

  /** 运行时上下文：用于感知当前执行进度与历史统计。 */
  interface RuntimeContext {
    /** 当前步号（从 1 开始） */
    step: number
    /** 被测应用包名 */
    package_name: string
      /** 页面源类型，例如 "uia" / "poco" / "screenshot" */
      page_source_type: string
    /** 上一步动作（如有） */
    last_action?: Action
    /** 上一步错误信息（如有） */
    last_error?: string
    /** 连续失败次数 */
    consecutive_failures: number
    /** 页面访问计数：key 为页面名 */
    page_visit_count: Record<string, number>
    /** 动作执行计数：key 为动作类型 */
    action_count: Record<string, number>
    /** 阻塞恢复上下文：仅在 monkey 识别阻塞后触发兜底决策时存在。值为阻塞原因字符串，无原因时为 true */
    block_recovery?: string | true
  }

  interface PluginContext {
    page: PageSnapshot
    runtime: RuntimeContext
  }

  interface StepResult {
    /** 对应步号 */
    step: number
    /** 实际执行动作 */
    action: Action
    /** 执行是否成功 */
    success: boolean
    /** 失败错误文本 */
    error?: string
    /** 本步耗时（毫秒） */
    duration_ms: number
    /** 是否检测到 crash */
    crash: boolean
    /** 是否检测到 anr */
    anr: boolean
    /** 执行前页面快照 */
    before: PageSnapshot
    /** 执行后页面快照 */
    after?: PageSnapshot
  }

  interface StepResultContext extends PluginContext {
    result: StepResult
  }

  /** 生命周期钩子上下文（onInit / onDestroy），不含页面快照。 */
  interface LifecycleContext {
    /** 被测应用包名 */
    package_name: string
      /** 页面源类型，例如 "uia" / "poco" / "screenshot" */
      page_source_type: string
  }

  // ── 插件钩子 ─────────────────────────────────────────────────

  /**
   * Trek 插件钩子接口。
   *
   * 完整生命周期：
   *
   *   1. onInit          — 插件加载完成后调用（仅一次）。适合初始化 trek.store。
   *   ┌─ 每个遍历步骤循环 ─────────────────────────────────────┐
   *   │  2. transformPage  — 引擎抓取页面源后调用，可改页面名和 XML。   │
   *   │                      注意：会被调用两次（页面名解析+决策阶段）。  │
   *   │  3. beforeDecide   — 决策前调用，返回 Action 可短路算法决策。    │
   *   │  4. [算法决策]      — Monkey / Reuse / UctBandit 选择执行计划。  │
   *   │  5. afterDecide    — 决策后调用，可改写/替换动作。               │
   *   │  6. [执行动作]      — 引擎执行最终动作。                        │
   *   │  7. onStepResult   — 执行完毕后调用，含结果/截图/健康信号。      │
   *   └──────────────────────────────────────────────────────────────┘
   *   8. onDestroy       — 插件被清除或替换前调用（仅一次）。适合清理资源。
   *
   * trek.store 是跨步骤持久的，在同一脚本实例生命周期内一直存在。
   * 多插件时按 plugins 数组顺序链式调用，前一个 transformPage 的输出是下一个的输入。
   */
  interface Plugin {
    /**
     * 初始化钩子。
     * 插件加载完成后调用，仅触发一次。适合在此初始化 trek.store 或执行一次性配置。
     * 注意：由于 Goja 每次钩子调用都会重建 VM，此钩子中的 trek.store 会跨步持久。
     */
    onInit?(ctx: LifecycleContext): void
    /**
     * 自定义页面名解析钩子。
     * 在页面名策略解析阶段调用（早于 transformPage），返回自定义页面名。
     * 适用于 image_fingerprint 策略或需要完全自定义页面名生成逻辑的场景。
     * ctx.page.screenshot 包含截图数据（需开启截图采集）。
     * 返回非空字符串覆盖页面名，返回 null/void 走默认策略。
     */
    resolvePageName?(ctx: PluginContext): string | null | void
    /**
     * 页面改造钩子。
     * 引擎抓取页面源后调用，可修改页面名和 XML，结果进入后续整条决策链路。
     * 返回 { page_name?, xml? } 覆盖原值，返回 void 保持原值。
     */
    transformPage?(ctx: PluginContext): PageInfo | void
    /**
     * 决策前钩子。
     * 在算法决策前调用。返回 Action 可直接短路，跳过算法决策和 afterDecide。
     * 返回 null/void 则继续走算法决策。
     */
    beforeDecide?(ctx: PluginContext): Action | null | void
    /**
     * 决策后钩子。
     * 算法决策完成后调用，可改写或替换动作。
     * 返回新 Action 替换原动作，返回 null 保持原动作。
     */
    afterDecide?(ctx: PluginContext, action: Action): Action | null | void
    /**
     * 步结果回调。
     * 动作执行完毕后调用。ctx.result 包含：
     * - step / action / success / error：执行基本信息
     * - duration_ms：本步耗时（毫秒）
     * - crash / anr：健康检测信号
     * - before / after：执行前后页面快照
     * 适合在此用 trek.store 维护跨步骤的策略状态。
     */
    onStepResult?(ctx: StepResultContext): void
    /**
     * 卸载钩子。
     * 插件被清除或替换前调用，仅触发一次。适合在此清理资源或保存状态。
     * 注意：此钩子在新 VM 中执行，无法访问之前钩子的局部变量，但 trek.store 数据仍然可用。
     */
    onDestroy?(ctx: LifecycleContext): void
  }

  // ── 配置枚举 ─────────────────────────────────────────────────

  type LogLevel = "debug" | "info" | "warn" | "warning" | "error" | "fatal"
  type PageSourceType = "uia" | "poco" | "screenshot"
  type TouchMode = "adb" | "motion" | "uia"
  type PageControlStrategy = "raw" | "ocr" | "llm" | "chain"
  type PageNameStrategy =
    | "structure_fingerprint"
    | "activity_only"
    | "image_fingerprint"
  type AlgorithmType = "random" | "reuse" | "uct_bandit"
  type PocoEngine =
    | "COCOS_2DX_JS"
    | "COCOS_2DX_C++"
    | "COCOS_CREATOR"
    | "EGRET"
    | "UNITY_3D"
    | "UE4"
    | "COCOS_2DX_LUA"

  // ── 配置接口 ─────────────────────────────────────────────────

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

  interface StaticConfig {
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
  }

  // ── 运行时 API ───────────────────────────────────────────────

  interface ActionAPI {
    /** 点击 */
    click(bounds: Bounds): Action
    /** 长按 */
    longClick(bounds: Bounds): Action
    /** 点击并输入文本 */
    input(bounds: Bounds, text: string, options?: { clear?: boolean; adb_input?: boolean }): Action
    /** 返回键 */
    back(): Action
    /** 滑动 */
    scroll(direction: ScrollDirection, bounds?: Bounds): Action
    /** 启动应用 */
    start(): Action
    /** 重启应用 */
    restart(options?: { clean?: boolean }): Action
    /** 拉起到前台 */
    activate(): Action
    /** 空动作 */
    nop(): Action
  }

  interface PageAPI {
    /**
     * 通过标准 XPath 从页面 XML 中查找节点，返回 PageNode 或 null。
     * @example
     * const node = trek.page.findByXpath(ctx.page, '//node[@resource-id="com.example:id/btn"]');
     * if (node) trek.log.info(`found: ${node.text}`);
     */
    findByXpath(page: PageSnapshot, xpath: string): PageNode | null
    /**
     * 从 XML 中移除所有包含指定文本的节点（字符串精确匹配）。
     * @example
     * ctx.page.xml = trek.page.excludeByText(ctx.page.xml, '广告');
     */
    excludeByText(xml: string, text: string): string
    /**
     * 从 XML 中移除所有包含指定 resource-id 的节点（字符串精确匹配）。
     * @example
     * ctx.page.xml = trek.page.excludeByResourceId(ctx.page.xml, 'com.example:id/ad_container');
     */
    excludeByResourceId(xml: string, id: string): string
    /**
     * 替换 XML 中的文本；from 支持字符串或正则（如 /pattern/flags）。
     * @example
     * ctx.page.xml = trek.page.replaceText(ctx.page.xml, '登录', 'Sign In');
     * ctx.page.xml = trek.page.replaceText(ctx.page.xml, /用户\d+/g, '用户***');
     */
    replaceText(xml: string, from: string | RegExp, to: string): string
    /**
     * 替换 XML 中的 resource-id；from 支持字符串或正则。
     * @example
     * ctx.page.xml = trek.page.replaceResourceId(ctx.page.xml, 'com.example.v2', 'com.example');
     */
    replaceResourceId(xml: string, from: string | RegExp, to: string): string
    /**
     * 判断页面快照是否包含截图。
     * @example
     * if (trek.page.hasScreenshot(ctx.page)) { ... }
     */
    hasScreenshot(page: PageSnapshot): boolean
    /**
     * 获取截图原始字节，无截图时返回 null。
     * @example
     * const bytes = trek.page.screenshotBytes(ctx.page);
     * if (bytes) trek.log.info(`size: ${bytes.length}`);
     */
    screenshotBytes(page: PageSnapshot): Uint8Array | null
    /**
     * 获取截图字节数，无截图时返回 0。
     * @example
     * trek.log.info(`screenshot: ${trek.page.screenshotSize(ctx.page)} bytes`);
     */
    screenshotSize(page: PageSnapshot): number
  }

  /**
   * 跨步骤持久键值存储（插件私有，不参与引擎内部决策）。
   * key 为插件脚本中自定义的任意字符串，无预定义 schema，按需命名即可。
   * 同一插件生命周期内（onInit → onDestroy），所有钩子共享此状态；
   * 适合记录访问计数、决策历史、页面标记等策略数据。
   * 多插件时各插件实例独立，互不影响。
   *
   * 注意：引擎决策层使用的页面访问计数、动作计数等数据在 ctx.runtime 中提供，
   * 与 trek.store 互不干扰。
   */
  interface StoreAPI {
    /**
     * 读取持久状态值，key 不存在返回 undefined。
     * @example
     * const count = trek.store.get('click_count') || 0;
     */
    get<T = unknown>(key: string): T | undefined
    /**
     * 写入持久状态值，跨步骤可用。
     * @example
     * trek.store.set('last_page', ctx.page.page_name);
     */
    set<T = unknown>(key: string, value: T): void
    /**
     * 对指定 key 做整数自增（默认 +1），返回自增后的值。
     * @example
     * const n = trek.store.increment('visit_count');
     * trek.store.increment('score', 10);
     */
    increment(key: string, delta?: number): number
    /**
     * 删除指定 key。
     * @example
     * trek.store.delete('temp_data');
     */
    delete(key: string): void
    /**
     * 清空所有持久状态。
     * @example
     * trek.store.clear();
     */
    clear(): void
  }

  interface LogAPI {
    debug(message: string): void
    info(message: string): void
    warn(message: string): void
    error(message: string): void
  }

  interface HTTPRequestOptions {
    method?: string
    url: string
    headers?: Record<string, string>
    body?: string | Uint8Array | number[]
    /** 请求超时（毫秒）。默认 10000，最大 30000 */
    timeout_ms?: number
  }

  interface HTTPResponse {
    status: number
    status_text: string
    ok: boolean
    headers: Record<string, string>
    body: string
    bytes: Uint8Array
  }

  interface HTTPAPI {
    /** 发起同步 HTTP 请求，仅支持 http / https。响应体最大 2MB。 */
    request(options: HTTPRequestOptions): HTTPResponse
    /** 发起 GET 请求。 */
    get(url: string, options?: Omit<HTTPRequestOptions, "method" | "url" | "body">): HTTPResponse
    /** 发起 POST 请求。 */
    post(url: string, body?: string | Uint8Array | number[], options?: Omit<HTTPRequestOptions, "method" | "url" | "body">): HTTPResponse
  }

  // ── OCR API ──────────────────────────────────────────────────

  /** OCR 识别出的文本区域。 */
  interface OCRRegion {
    /** 识别出的文本内容（格式为 intent 字符串，如 "ocr_click:确定"）。 */
    text: string
    /** 置信度 [0, 1]。 */
    confidence: number
    /** 归一化边界 [left, top, right, bottom]，范围 [0, 1]。 */
    bounds: [number, number, number, number]
  }

  interface OCRRecognizeOptions {
    /** 截图字节（来自 trek.page.screenshotBytes）。 */
    screenshot: Uint8Array | number[]
    /** OCR 服务端点 URL。缺省读 PADDLEOCR_API_URL 环境变量。 */
    endpoint?: string
    /** 认证密钥。缺省读 PADDLEOCR_API_KEY 环境变量。 */
    api_key?: string
    /** 请求超时毫秒。默认 10000。 */
    timeout_ms?: number
    /** 额外请求头。 */
    headers?: Record<string, string>
  }

  interface OCRAPI {
    /**
     * 调用 OCR 服务识别截图中的文本区域，返回归一化坐标的区域列表。
     * @example
     * const regions = trek.ocr.recognize({
     *   screenshot: trek.page.screenshotBytes(ctx.page),
     *   endpoint: 'http://ocr-server:8080/ocr',
     * });
     * for (const r of regions) {
     *   trek.log.info(`text=${r.text} bounds=${r.bounds}`);
     * }
     */
    recognize(options: OCRRecognizeOptions): OCRRegion[]
  }

  // ── LLM API ──────────────────────────────────────────────────

  interface LLMChatOptions {
    /** 用户提示词。 */
    prompt: string
    /** 可选截图（多模态输入）。 */
    screenshot?: Uint8Array | number[]
    /** LLM 端点 URL（OpenAI Chat Completions 格式）。缺省读 LLM_API_URL / OPENAI_API_URL。 */
    endpoint?: string
    /** 认证密钥。缺省读 LLM_API_KEY / OPENAI_API_KEY。 */
    api_key?: string
    /** 模型名称。缺省读 LLM_MODEL / OPENAI_MODEL。 */
    model?: string
    /** 请求超时毫秒。默认 30000。 */
    timeout_ms?: number
    /** 额外请求头。 */
    headers?: Record<string, string>
    /** 最大输出 token 数。默认 4096。 */
    max_tokens?: number
  }

  interface LLMAPI {
    /**
     * 调用 LLM 多模态对话，返回文本响应。
     * 支持所有 OpenAI Chat Completions 格式的端点（GPT-4o、Gemini、Qwen-VL 等）。
     * @example
     * const text = trek.llm.chat({
     *   prompt: '根据截图描述当前页面内容',
     *   screenshot: trek.page.screenshotBytes(ctx.page),
     * });
     */
    chat(options: LLMChatOptions): string
  }

  // ── File API ─────────────────────────────────────────────────

  /** trek.file.open() 返回的文件句柄。 */
  interface FileHandle {
    /** 读取全部内容为字符串。 */
    readString(): string
    /** 读取全部内容为字节数组。 */
    readBytes(): Uint8Array
    /** 读取 n 字节；n<=0 或省略则读取全部。 */
    read(n?: number): Uint8Array
    /** 读取一行（不含换行符）。 */
    readLine(): string
    /** 读取所有行，返回字符串数组。 */
    readLines(): string[]
    /** 写入字符串，返回写入字节数。 */
    writeString(data: string): number
    /** 写入字节数组，返回写入字节数。 */
    writeBytes(data: Uint8Array | number[]): number
    /** 写入字符串或字节，返回写入字节数。 */
    write(data: string | Uint8Array | number[]): number
    /** 移动文件指针。whence: "start"（默认）/ "current" / "end"。 */
    seek(offset: number, whence?: string): number
    /** 返回当前文件指针位置。 */
    tell(): number
    /** 返回文件大小（字节）。 */
    size(): number
    /** 关闭文件句柄。 */
    close(): void
    /** 返回文件路径。 */
    path(): string
  }

  interface FileAPI {
    /**
     * 打开文件，返回文件句柄。
     * @param path 文件路径
     * @param mode 打开模式："r" 只读（默认）, "w" 写入（清空）, "a" 追加, "r+" 读写
     * @example
     * const f = trek.file.open('/sdcard/config.json');
     * const text = f.readString();
     * f.close();
     *
     * const w = trek.file.open('/sdcard/output.txt', 'w');
     * w.writeString('hello\n');
     * w.close();
     */
    open(path: string, mode?: string): FileHandle
    /**
     * 检查文件是否存在。
     * @example
     * if (trek.file.exists('/sdcard/config.json')) { ... }
     */
    exists(path: string): boolean
  }

  interface API {
    action: ActionAPI
    page: PageAPI
    store: StoreAPI
    log: LogAPI
    http: HTTPAPI
    ocr: OCRAPI
    llm: LLMAPI
    file: FileAPI
    /** 同步暂停指定毫秒数，最大 30000。 */
    sleep(milliseconds: number): void
  }
}

// ── 全局常量 ───────────────────────────────────────────────────

/** Trek 运行时 API：动作构建、页面操作、插件存储、日志 */
declare const trek: Trek.API
/** Trek 静态配置（config.js 中定义） */
declare const config: Trek.StaticConfig
/** Trek 插件钩子（plugin.js 中定义） */
declare const plugin: Trek.Plugin
