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

interface TrekPlugin {
  /** 页面改造钩子：可改页面名和 xml，结果会进入后续整条决策链路。 */
  transformPage?(ctx: PluginContext): PageInfo | void
  /** 决策前钩子：返回动作可直接短路默认模型决策。 */
  beforeDecide?(ctx: PluginContext): Action | null | void
  /** 决策后钩子：可改写模型动作；返回 null 表示保持原动作。 */
  afterDecide?(ctx: PluginContext, action: Action): Action | null | void
  /** 步结果回调：可基于 crash/anr、前后页面、截图维护策略状态。 */
  onStepResult?(ctx: StepResultContext): void
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
type AlgorithmType = "random" | "reuse" | "server" | "uct_bandit"
type PocoEngine =
  | "COCOS_2DX_JS"
  | "COCOS_2DX_C++"
  | "COCOS_CREATOR"
  | "EGRET"
  | "UNITY_3D"
  | "UE4"
  | "COCOS_2DX_LUA"

interface UCTBanditConfig {
  /** 两状态循环惩罚系数 */
  two_state_loop_penalty?: number
  /** 边重复惩罚系数 */
  edge_repeat_penalty?: number
  /** 边重复惩罚触发阈值 */
  edge_repeat_threshold?: number
  /** 动作冷却惩罚系数 */
  action_cooldown_penalty?: number
  /** 近期动作窗口大小 */
  recent_action_window?: number
  /** 循环逃逸探索加成系数 */
  loop_escape_explore_boost?: number
}

interface TrekStaticConfig {
  res_mapping?: Record<string, string>
  black_rects?: Record<string, Bounds[]>
  skip_all_actions_from_model?: boolean
  /** 遍历算法：random / reuse / server / uct_bandit */
  algorithm?: AlgorithmType
  /** 外部插件脚本路径列表 */
  plugins?: string[]
  /** 指定 monkey 运行使用的页面源类型 */
  page_source?: PageSourceType
  /** 指定 monkey 运行使用的触控模式 */
  touch_mode?: TouchMode
  /** 指定页面名生成策略（不填时按页面源自动选择） */
  page_name_strategy?: PageNameStrategy
  /** 是否采集截图给决策层 */
  capture_screenshot?: boolean
  /** 是否保留每步记录 */
  keep_step_records?: boolean
  /** OCR 探索超时（毫秒） */
  explore_ocr_timeout_ms?: number
  /** 恢复 LLM 超时（毫秒） */
  recovery_llm_timeout_ms?: number
  /** 恢复冷却步数 */
  recovery_cooldown_steps?: number
  /** 恢复 LLM 最大调用次数 */
  recovery_llm_max_calls?: number
  /** 恢复 LLM 调用窗口步数 */
  recovery_llm_window_steps?: number
  /** 两状态循环检测阈值 */
  recovery_two_state_loop_threshold?: number
  /** 高访问页面阈值 */
  recovery_high_visit_threshold?: number
  /** 低奖励窗口大小 */
  recovery_low_reward_window?: number
  /** 候选歧义顶部间距阈值 */
  candidate_ambiguity_top_gap_threshold?: number
  /** 高价值页面访问上限 */
  high_value_page_visit_limit?: number
  /** 候选风险下降阈值 */
  candidate_risk_drop_threshold?: number
  /** 候选最小融合分数 */
  candidate_min_fusion_score?: number
  /** 滚动推断阈值 */
  scroll_infer_threshold?: number
  /** UIA 端口相关配置 */
  uia?: {
    /** 设备端 UIA server 端口（默认 6790） */
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
  /** 有效触控区域映射（坐标映射公式：x' = left + (right-left) * x） */
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
