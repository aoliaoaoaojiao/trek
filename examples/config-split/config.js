/// <reference path="../../assets/goja/trek-plugin.d.ts" />

const config = {
  page_source: "uia",
  page_name_strategy: "uia_activity_first",
  capture_screenshot: true,
  keep_step_records: true,

  // 插件按数组顺序执行。
  plugins: [
    "./plugins/normalize-page.plugin.js",
    "./plugins/recovery-guard.plugin.js",
  ],

  // 可选：恢复与候选调参统一放在配置文件中。
  recovery_cooldown_steps: 2,
  llm_max_calls: 3,
  llm_window_steps: 30,
  recovery_two_state_loop_threshold: 2,
  recovery_high_visit_threshold: 8,
  recovery_low_reward_window: 6,
  candidate_ambiguity_top_gap_threshold: 0.15,
  high_value_page_visit_limit: 2,
  candidate_risk_drop_threshold: 2.1,
  candidate_min_fusion_score: -0.3,
}
