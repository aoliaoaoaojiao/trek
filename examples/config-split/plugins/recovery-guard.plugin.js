/// <reference path="../../../assets/goja/trek-plugin.d.ts" />

const plugin = {
  beforeDecide(ctx) {
    // 示例：阻塞恢复阶段优先尝试返回，避免在弹窗/死路上反复点击。
    if (ctx.runtime.block_recovery && ctx.runtime.block_recovery.requested) {
      return trek.action.back()
    }
  },

  onStepResult(ctx) {
    if (ctx.result.crash || ctx.result.anr) {
      trek.log.warn("检测到 crash/anr，建议结合本地日志排查稳定性问题")
    }
  },
}
