/**
 * Trek 基础类型定义
 */

declare namespace Trek {
  // ── 动作类型 ──────────────────────────────────────────────────

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

  /** 坐标边界 [left, top, right, bottom] */
  export type Bounds = [number, number, number, number]

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
}
