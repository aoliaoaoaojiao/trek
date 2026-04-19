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
  text: string
  resource_id: string
  content_desc: string
  class_name: string
  bounds: Bounds
  clickable: boolean
  enabled: boolean
  editable: boolean
  path: string
}

interface PageSnapshot {
  name: string
  xml: string
  screenshot?: Screenshot
  nodes: PageNode[]
}

interface RuntimeContext {
  step: number
  package_name: string
  page_source_type: string
  last_action?: Action
  last_error?: string
  consecutive_failures: number
  page_visit_count: Record<string, number>
  action_count: Record<string, number>
}

interface PluginContext {
  page: PageSnapshot
  runtime: RuntimeContext
}

interface PageInfo {
  page_name?: string
  xml?: string
}

interface Action {
  action: ActionType
  bounds?: Bounds
  text?: string
  clear?: boolean
  adb_input?: boolean
  allow_fuzzing?: boolean
  throttle?: number
  wait_time?: number
}

interface StepResult {
  step: number
  action: Action
  success: boolean
  error?: string
  duration_ms: number
  crash: boolean
  anr: boolean
  before: PageSnapshot
  after?: PageSnapshot
}

interface StepResultContext extends PluginContext {
  result: StepResult
}

interface TrekPlugin {
  transformPage?(ctx: PluginContext): PageInfo | void
  beforeDecide?(ctx: PluginContext): Action | null | void
  afterDecide?(ctx: PluginContext, action: Action): Action | null | void
  onStepResult?(ctx: StepResultContext): void
}

type LogLevel = "debug" | "info" | "warn" | "warning" | "error" | "fatal"

interface TrekStaticConfig {
  res_mapping?: Record<string, string>
  black_rects?: Record<string, Bounds[]>
  skip_all_actions_from_model?: boolean
  log?: {
    /** 文件日志级别；控制台日志级别由命令行 -log-level 控制 */
    file_level?: LogLevel
    fileLevel?: LogLevel
  }
}

interface TrekActionAPI {
  click(bounds: Bounds): Action
  longClick(bounds: Bounds): Action
  input(bounds: Bounds, text: string, options?: { clear?: boolean; adb_input?: boolean }): Action
  back(): Action
  scroll(direction: ScrollDirection, bounds?: Bounds): Action
  start(): Action
  restart(options?: { clean?: boolean }): Action
  activate(): Action
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
