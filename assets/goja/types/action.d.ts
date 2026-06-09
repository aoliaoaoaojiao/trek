/**
 * Trek 动作相关类型定义
 */

declare namespace Trek {
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
}
