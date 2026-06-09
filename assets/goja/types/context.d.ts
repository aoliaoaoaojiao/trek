/**
 * Trek 上下文相关类型定义
 */

declare namespace Trek {
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
}
