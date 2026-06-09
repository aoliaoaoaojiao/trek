/**
 * Trek 插件钩子类型定义
 */

declare namespace Trek {
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
  export interface Plugin {
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
}
