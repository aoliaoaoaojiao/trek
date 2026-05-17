export type ConfigPayload = {
  page_source: "uia" | "poco" | "screenshot"
  page_name_strategy: PageNameStrategy
  touch_mode: "motion" | "uia" | "adb"
  skip_all_actions_from_model: boolean
  page_control_strategy: "" | "raw" | "ocr" | "llm"
  algorithm: "" | "reuse" | "uctbandit" | "random"
  capture_screenshot: boolean | null
  keep_step_records: boolean | null
  scroll_infer_threshold: number | null
  image_similarity_ssim_threshold: number | null
  explore_ocr_timeout_ms: number | null
  llm_timeout_ms: number | null
  recovery_cooldown_steps: number | null
  recovery_two_state_loop_threshold: number | null
  recovery_high_visit_threshold: number | null
  recovery_low_reward_window: number | null
  candidate_ambiguity_top_gap_threshold: number | null
  high_value_page_visit_limit: number | null
  candidate_risk_drop_threshold: number | null
  candidate_min_fusion_score: number | null
  uia: { server_port: number }
  poco: { engine: string; port: number }
  log: { file_level: string }
  uct_bandit: {
    two_state_loop_penalty: number | null
    edge_repeat_penalty: number | null
    edge_repeat_threshold: number | null
    action_cooldown_penalty: number | null
    recent_action_window: number | null
    loop_escape_explore_boost: number | null
  }
  reuse: {
    epsilon: number | null
    gamma: number | null
    n_step: number | null
    model_save_path: string
    enable_model_persistence: boolean | null
    reset_model_on_start: boolean | null
  }
  effective_touch_area: {
    serial: string
    package_name: string
    range: EffectiveRange
  }
}

export type PartialConfigPayload = Partial<{
  page_source: "uia" | "poco" | "screenshot"
  page_name_strategy: PageNameStrategy
  touch_mode: "motion" | "uia" | "adb"
  skip_all_actions_from_model: boolean
  page_control_strategy: "" | "raw" | "ocr" | "llm"
  algorithm: "" | "reuse" | "uctbandit" | "random"
  capture_screenshot: boolean | null
  keep_step_records: boolean | null
  scroll_infer_threshold: number | null
  image_similarity_ssim_threshold: number | null
  explore_ocr_timeout_ms: number | null
  llm_timeout_ms: number | null
  recovery_cooldown_steps: number | null
  recovery_two_state_loop_threshold: number | null
  recovery_high_visit_threshold: number | null
  recovery_low_reward_window: number | null
  candidate_ambiguity_top_gap_threshold: number | null
  high_value_page_visit_limit: number | null
  candidate_risk_drop_threshold: number | null
  candidate_min_fusion_score: number | null
  uia: Partial<{ server_port: number }>
  poco: Partial<{ engine: string; port: number }>
  log: Partial<{ file_level: string }>
  uct_bandit: Partial<{
    two_state_loop_penalty: number | null
    edge_repeat_penalty: number | null
    edge_repeat_threshold: number | null
    action_cooldown_penalty: number | null
    recent_action_window: number | null
    loop_escape_explore_boost: number | null
  }>
  reuse: Partial<{
    epsilon: number | null
    gamma: number | null
    n_step: number | null
    model_save_path: string
    enable_model_persistence: boolean | null
    reset_model_on_start: boolean | null
  }>
  effective_touch_area: Partial<{
    serial: string
    package_name: string
    range: Partial<EffectiveRange>
  }>
}>

export type PageNameStrategy =
  | ""
  | "structure_fingerprint"
  | "activity_only"
  | "image_fingerprint"

export type DeviceOption = {
  serial: string
  label: string
}

export type BoundsRect = {
  left: number
  top: number
  right: number
  bottom: number
}

export type DumpTreeNode = {
  id: string
  tag: string
  attrs: Record<string, string>
  bounds: BoundsRect | null
  children: DumpTreeNode[]
}

export type ClickPoint = {
  imagePercentX: number
  imagePercentY: number
  percentX: number
  percentY: number
  absoluteX: number
  absoluteY: number
}

export type EffectiveRange = {
  left: number
  top: number
  right: number
  bottom: number
}
