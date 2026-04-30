/**
 * Trek Goja 策略插件类型声明
 * 用法：在插件脚本顶部添加
 * /// <reference path="./trek-plugin.d.ts" />
 */

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
  /** 节点在树中的路径（调试定位用） */
  path: string
  /** 节点的标准 XPath（优先用于跨模块定位） */
  xpath?: string
}

/** 页面快照：由引擎采集并传给插件。 */
interface PageSnapshot {
  /**
   * 当前页面名。
   * 当 page_source_type="uia" 时，默认优先使用 `dumpsys activity top` 解析出的 Activity 名。
   * 插件可在 `transformPage` 中返回 `page_name` 进行覆盖。
   */
  name: string
  /** 当前页面 XML（插件可在 transformPage 中返回 xml 覆盖） */
  xml: string
  /** 截图（需运行时开启截图采集） */
  screenshot?: Screenshot
  /** 从 XML 提取的节点列表，便于脚本筛选控件 */
  nodes: PageNode[]
}

/** 运行时上下文：用于感知当前执行进度与历史统计。 */
interface RuntimeContext {
  /** 当前步号（从 1 开始） */
  step: number
  /** 被测应用包名 */
  package_name: string
  /** 页面源类型，例如 "uia" / "poco" */
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
  /** 阻塞恢复请求上下文：仅在 monkey 识别阻塞后触发兜底决策时为 true */
  block_recovery?: {
    /** 当前是否处于阻塞恢复阶段 */
    requested: boolean
    /** 阻塞原因（可选） */
    reason?: string
  }
}

interface PluginContext {
  page: PageSnapshot
  runtime: RuntimeContext
}

interface PageInfo {
  /** 覆盖页面名；不返回则沿用原值 */
  page_name?: string
  /** 覆盖页面 XML；不返回则沿用原值 */
  xml?: string
}

interface Action {
  /** 动作类型 */
  action: ActionType
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
  /** 页面源类型，例如 "uia" / "poco" */
  page_source_type: string
}

/**
 * Trek 插件钩子接口。
 *
 * 完整生命周期：
 *
 *   1. onInit          — 插件加载完成后调用（仅一次）。适合初始化 trek.state。
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
 * trek.state 是跨步骤持久的，在同一脚本实例生命周期内一直存在。
 * 多插件时按 plugins 数组顺序链式调用，前一个 transformPage 的输出是下一个的输入。
 */
interface TrekPlugin {
  /**
   * 初始化钩子。
   * 插件加载完成后调用，仅触发一次。适合在此初始化 trek.state 或执行一次性配置。
   * 注意：由于 Goja 每次钩子调用都会重建 VM，此钩子中的 trek.state 会跨步持久。
   */
  onInit?(ctx: LifecycleContext): void
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
   * 适合在此用 trek.state 维护跨步骤的策略状态。
   */
  onStepResult?(ctx: StepResultContext): void
  /**
   * 卸载钩子。
   * 插件被清除或替换前调用，仅触发一次。适合在此清理资源或保存状态。
   * 注意：此钩子在新 VM 中执行，无法访问之前钩子的局部变量，但 trek.state 数据仍然可用。
   */
  onDestroy?(ctx: LifecycleContext): void
}

type LogLevel = "debug" | "info" | "warn" | "warning" | "error" | "fatal"
type PageSourceType = "uia" | "poco"
type TouchMode = "adb" | "motion" | "uia"
type PageNameStrategy =
  | "uia_activity_first"
  | "xml_only"
  | "xml_fingerprint"
  | "structure_fingerprint"
  | "activity_only"
type AlgorithmType = "random" | "monkey" | "reuse" | "uct_bandit"
type PocoEngine =
  | "COCOS_2DX_JS"
  | "COCOS_2DX_C++"
  | "COCOS_CREATOR"
  | "EGRET"
  | "UNITY_3D"
  | "UE4"
  | "COCOS_2DX_LUA"

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

interface TrekStaticConfig {
  res_mapping?: Record<string, string>
  black_rects?: Record<string, Bounds[]>
  skip_all_actions_from_model?: boolean
  /** 遍历算法：random / monkey / reuse / uct_bandit。默认 "reuse" */
  algorithm?: AlgorithmType
  /** 外部插件脚本路径列表 */
  plugins?: string[]
  /** 指定 monkey 运行使用的页面源类型。默认 "uia" */
  page_source?: PageSourceType
  /** 指定 monkey 运行使用的触控模式。默认 "motion" */
  touch_mode?: TouchMode
  /** 指定页面名生成策略（不填时按页面源自动选择：UIA 默认 uia_activity_first，Poco 默认 xml_only） */
  page_name_strategy?: PageNameStrategy
  /** 是否采集截图给决策层。默认 false */
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
   * 恢复 LLM 超时（毫秒）。
   * 当引擎检测到卡死（如两状态循环、高访问低收益）时进入恢复阶段，
   * 会将当前页面上下文发送给 LLM 请求恢复动作建议。此字段控制该 HTTP 请求的超时时间。默认 15000
   */
  recovery_llm_timeout_ms?: number
  /**
   * 恢复冷却步数。
   * 恢复动作成功脱困后，状态机进入冷却模式持续此步数，期间不会再触发恢复，
   * 防止刚脱困就立即再次进入恢复。默认 2
   */
  recovery_cooldown_steps?: number
  /**
   * 恢复 LLM 最大调用次数。
   * 滑动窗口内允许的 LLM 调用上限（恢复阶段和候选增强共享此预算）。
   * 超出后 LLM 调用被拒绝，回退到纯算法候选。0 表示不限制。默认 0
   */
  recovery_llm_max_calls?: number
  /**
   * 恢复 LLM 调用窗口步数。
   * 配合 recovery_llm_max_calls 使用，控制滑动窗口大小（步数）。
   * 窗口外的历史调用不再计入配额。0 表示不使用滑动窗口（全局累计）。默认 0
   */
  recovery_llm_window_steps?: number
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
   * Explore 模式下，当权重最高的两个候选动作的权重差 <= 此值时，
   * 认为候选"歧义"（难以区分），可能触发 LLM 增强来提供建议。默认 0.15
   */
  candidate_ambiguity_top_gap_threshold?: number
  /**
   * 高价值页面访问上限。
   * 页面访问次数在 1~此值范围内被视为"高价值页面"，会触发 LLM 候选增强。
   * 超过此值后不再对该页面增强，保持保守策略。默认 2
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
   * 用于解决 Poco/Unity 游戏 UI 未声明 ScrollRect 的场景。0 禁用推断。默认 5
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

interface TrekActionAPI {
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

interface TrekPageAPI {
  findByText(page: PageSnapshot, text: string): PageNode | null
  findByResourceId(page: PageSnapshot, id: string): PageNode | null
  findByContentDesc(page: PageSnapshot, desc: string): PageNode | null
  findByClass(page: PageSnapshot, className: string): PageNode | null
  findAll(page: PageSnapshot, predicate: (node: PageNode) => boolean): PageNode[]
  removeByText(xml: string, text: string): string
  removeByResourceId(xml: string, id: string): string
  patchText(xml: string, from: string | RegExp, to: string): string
  patchResourceId(xml: string, from: string | RegExp, to: string): string
  hasScreenshot(page: PageSnapshot): boolean
  screenshotBytes(page: PageSnapshot): Uint8Array | null
  screenshotSize(page: PageSnapshot): number
}

interface TrekStateAPI {
  get<T = unknown>(key: string): T | undefined
  set<T = unknown>(key: string, value: T): void
  inc(key: string, delta?: number): number
  delete(key: string): void
  clear(): void
}

interface TrekLogAPI {
  debug(message: string): void
  info(message: string): void
  warn(message: string): void
  error(message: string): void
}

interface TrekAPI {
  action: TrekActionAPI
  page: TrekPageAPI
  state: TrekStateAPI
  log: TrekLogAPI
}

declare const trek: TrekAPI
declare const config: TrekStaticConfig
declare const plugin: TrekPlugin
