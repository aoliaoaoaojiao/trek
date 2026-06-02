import { useEffect, useMemo, useState } from "react"
import type { MouseEvent } from "react"

import { Button } from "@/components/ui/button"
import {
  ResizableHandle,
  ResizablePanel,
  ResizablePanelGroup,
} from "@/components/ui/resizable"
import { ConfigPanel } from "@/monkey-config/ConfigPanel"
import { DumpPanel } from "@/monkey-config/DumpPanel"
import { ScreenshotPanel } from "@/monkey-config/ScreenshotPanel"
import { API_BASE, postJSON } from "@/monkey-config/api"
import { copyText, parseDumpTree } from "@/monkey-config/utils"
import type {
  ClickPoint,
  ConfigPayload,
  DeviceOption,
  EffectiveRange,
  PageNameStrategy,
  PartialConfigPayload,
} from "@/monkey-config/types"

const pageNameStrategies: PageNameStrategy[] = [
  "",
  "structure_fingerprint",
  "activity_only",
  "image_fingerprint",
]

function isPageNameStrategy(value: unknown): value is PageNameStrategy {
  return typeof value === "string" && pageNameStrategies.includes(value as PageNameStrategy)
}

function parseConfigSource(source: string): PartialConfigPayload {
  const module = { exports: {} as unknown }
  const load = new Function(
    "module",
    "exports",
    `"use strict";\n${source}\n; if (typeof config !== "undefined") return config; return module.exports;`
  ) as (module: { exports: unknown }, exports: unknown) => unknown
  const result = load(module, module.exports)
  const candidate = result === module.exports && isRecord(module.exports) && "default" in module.exports
    ? module.exports.default
    : result
  if (!isRecord(candidate)) {
    throw new Error("配置文件中没有找到 config 对象")
  }
  return candidate as PartialConfigPayload
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null
}

function parseOptionalNumber(raw: string): number | null {
  const text = raw.trim()
  if (text === "") {
    return null
  }
  const value = Number(text)
  if (Number.isNaN(value)) {
    return null
  }
  return value
}

function boolModeToValue(mode: "" | "true" | "false"): boolean | null {
  if (mode === "true") {
    return true
  }
  if (mode === "false") {
    return false
  }
  return null
}

function boolValueToMode(value: boolean | null | undefined): "" | "true" | "false" {
  if (value === true) {
    return "true"
  }
  if (value === false) {
    return "false"
  }
  return ""
}

export function App() {
  const [pageSource, setPageSource] = useState<"uia" | "poco" | "screenshot">("uia")
  const [mixedMode, setMixedMode] = useState(false)
  const [pageNameStrategy, setPageNameStrategy] = useState<PageNameStrategy>("structure_fingerprint")
  const [touchMode, setTouchMode] = useState<"motion" | "uia" | "adb">("motion")
  const [deviceSerial, setDeviceSerial] = useState("")
  const [deviceOptions, setDeviceOptions] = useState<DeviceOption[]>([])
  const [loadingDevices, setLoadingDevices] = useState(false)
  const [skipAll, setSkipAll] = useState(false)
  const [pageControlStrategy, setPageControlStrategy] = useState<"" | "raw" | "ocr" | "llm" | "chain">("")
  const [algorithm, setAlgorithm] = useState<"" | "reuse" | "uctbandit" | "random">("reuse")
  const [captureScreenshotMode, setCaptureScreenshotMode] = useState<"" | "true" | "false">("")
  const [keepStepRecordsMode, setKeepStepRecordsMode] = useState<"" | "true" | "false">("")
  const [uiaPort, setUiaPort] = useState("")
  const [pocoEngine, setPocoEngine] = useState("UNITY_3D")
  const [pocoPort, setPocoPort] = useState("")
  const [fileLevel, setFileLevel] = useState("info")
  const [scrollInferThreshold, setScrollInferThreshold] = useState("")
  const [imageSimilarityThreshold, setImageSimilarityThreshold] = useState("")
  const [imageFingerprintHammingThreshold, setImageFingerprintHammingThreshold] = useState("")
  const [pageControlCacheTTLSeconds, setPageControlCacheTTLSeconds] = useState("")
  const [exploreOCRTimeoutMs, setExploreOCRTimeoutMs] = useState("")
  const [llmTimeoutMs, setLLMTimeoutMs] = useState("")
  const [recoveryCooldownSteps, setRecoveryCooldownSteps] = useState("")
  const [recoveryTwoStateLoopThreshold, setRecoveryTwoStateLoopThreshold] = useState("")
  const [recoveryHighVisitThreshold, setRecoveryHighVisitThreshold] = useState("")
  const [recoveryLowRewardWindow, setRecoveryLowRewardWindow] = useState("")
  const [candidateAmbiguityTopGapThreshold, setCandidateAmbiguityTopGapThreshold] = useState("")
  const [highValuePageVisitLimit, setHighValuePageVisitLimit] = useState("")
  const [candidateRiskDropThreshold, setCandidateRiskDropThreshold] = useState("")
  const [candidateMinFusionScore, setCandidateMinFusionScore] = useState("")
  const [inputCharset, setInputCharset] = useState("")
  const [uctTwoStateLoopPenalty, setUctTwoStateLoopPenalty] = useState("")
  const [uctEdgeRepeatPenalty, setUctEdgeRepeatPenalty] = useState("")
  const [uctEdgeRepeatThreshold, setUctEdgeRepeatThreshold] = useState("")
  const [uctActionCooldownPenalty, setUctActionCooldownPenalty] = useState("")
  const [uctRecentActionWindow, setUctRecentActionWindow] = useState("")
  const [uctLoopEscapeExploreBoost, setUctLoopEscapeExploreBoost] = useState("")

  // 后端默认值，用于 placeholder 显示
  const [defaultScrollInferThreshold, setDefaultScrollInferThreshold] = useState(5)
  const [defaultImageSimilarityThreshold, setDefaultImageSimilarityThreshold] = useState(0.995)
  const [defaultImageFingerprintHammingThreshold, setDefaultImageFingerprintHammingThreshold] = useState(10)
  const [defaultPageControlCacheTTLSeconds, setDefaultPageControlCacheTTLSeconds] = useState(1800)
  const [defaultExploreOCRTimeoutMs, setDefaultExploreOCRTimeoutMs] = useState(10000)
  const [defaultLLMTimeoutMs, setDefaultLLMTimeoutMs] = useState(15000)
  const [defaultUctTwoStateLoopPenalty, setDefaultUctTwoStateLoopPenalty] = useState(-6)
  const [defaultUctEdgeRepeatPenalty, setDefaultUctEdgeRepeatPenalty] = useState(-1)
  const [defaultUctEdgeRepeatThreshold, setDefaultUctEdgeRepeatThreshold] = useState(2)
  const [defaultUctActionCooldownPenalty, setDefaultUctActionCooldownPenalty] = useState(1.5)
  const [defaultUctRecentActionWindow, setDefaultUctRecentActionWindow] = useState(6)
  const [defaultUctLoopEscapeExploreBoost, setDefaultUctLoopEscapeExploreBoost] = useState(0.4)
  const [reuseEpsilon, setReuseEpsilon] = useState("")
  const [reuseGamma, setReuseGamma] = useState("")
  const [reuseNStep, setReuseNStep] = useState("")
  const [reuseModelSavePath, setReuseModelSavePath] = useState("")
  const [reuseEnableModelPersistenceMode, setReuseEnableModelPersistenceMode] = useState<"" | "true" | "false">("")
  const [reuseResetModelOnStartMode, setReuseResetModelOnStartMode] = useState<"" | "true" | "false">("")
  const [resultText, setResultText] = useState("")
  const [xmlPreview, setXmlPreview] = useState("")
  const [screenshotBase64, setScreenshotBase64] = useState("")
  const [usedSerial, setUsedSerial] = useState("")
  const [currentPackageName, setCurrentPackageName] = useState("")
  const [status, setStatus] = useState("")
  const [error, setError] = useState("")
  const [loading, setLoading] = useState(false)
  const [selectedDumpNodeId, setSelectedDumpNodeId] = useState("")
  const [expandedNodeIds, setExpandedNodeIds] = useState<Record<string, boolean>>({})
  const [imageNaturalSize, setImageNaturalSize] = useState({ width: 0, height: 0 })
  const [highlightLog, setHighlightLog] = useState("未选中控件")
  const [clickPoint, setClickPoint] = useState<ClickPoint | null>(null)
  const [clickLog, setClickLog] = useState("未点击图片")
  const [rangeLeftInput, setRangeLeftInput] = useState("0")
  const [rangeTopInput, setRangeTopInput] = useState("0")
  const [rangeRightInput, setRangeRightInput] = useState("1")
  const [rangeBottomInput, setRangeBottomInput] = useState("1")
  const [rangeLog, setRangeLog] = useState("当前范围仅内存生效（不持久化）")
  const [configTab, setConfigTab] = useState<"base" | "page" | "recovery" | "uct">("base")
  const [savePreviewOpen, setSavePreviewOpen] = useState(false)

  const fetchDevices = async () => {
    setLoadingDevices(true)
    setError("")
    try {
      const response = await fetch(`${API_BASE}/api/devices`)
      const data = (await response.json()) as {
        devices?: DeviceOption[]
        error?: string
      }
      if (!response.ok) {
        throw new Error(data.error ?? `获取设备失败: ${response.status}`)
      }
      const next = data.devices ?? []
      setDeviceOptions(next)
      if (
        deviceSerial !== "" &&
        next.find((item) => item.serial === deviceSerial) === undefined
      ) {
        setDeviceSerial("")
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "获取设备失败")
    } finally {
      setLoadingDevices(false)
    }
  }

  useEffect(() => {
    void fetchDevices()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  // 启动时获取后端默认值，更新 placeholder 显示
  useEffect(() => {
    const fetchDefaults = async () => {
      try {
        const response = await fetch(`${API_BASE}/api/defaults`)
        if (response.ok) {
          const data = (await response.json()) as Record<string, unknown>
          // 更新各字段的默认值（仅当用户未手动输入时）
          if (typeof data.scroll_infer_threshold === "number") {
            setDefaultScrollInferThreshold(data.scroll_infer_threshold)
          }
          if (typeof data.image_similarity_ssim_threshold === "number") {
            setDefaultImageSimilarityThreshold(data.image_similarity_ssim_threshold)
          }
          if (typeof data.image_fingerprint_hamming_threshold === "number") {
            setDefaultImageFingerprintHammingThreshold(data.image_fingerprint_hamming_threshold)
          }
          if (typeof data.page_control_cache_ttl_seconds === "number") {
            setDefaultPageControlCacheTTLSeconds(data.page_control_cache_ttl_seconds)
          }
          if (typeof data.explore_ocr_timeout_ms === "number") {
            setDefaultExploreOCRTimeoutMs(data.explore_ocr_timeout_ms)
          }
          if (typeof data.llm_timeout_ms === "number") {
            setDefaultLLMTimeoutMs(data.llm_timeout_ms)
          }
          if (typeof data.two_state_loop_penalty === "number") {
            setDefaultUctTwoStateLoopPenalty(data.two_state_loop_penalty)
          }
          if (typeof data.edge_repeat_penalty === "number") {
            setDefaultUctEdgeRepeatPenalty(data.edge_repeat_penalty)
          }
          if (typeof data.edge_repeat_threshold === "number") {
            setDefaultUctEdgeRepeatThreshold(data.edge_repeat_threshold)
          }
          if (typeof data.action_cooldown_penalty === "number") {
            setDefaultUctActionCooldownPenalty(data.action_cooldown_penalty)
          }
          if (typeof data.recent_action_window === "number") {
            setDefaultUctRecentActionWindow(data.recent_action_window)
          }
          if (typeof data.loop_escape_explore_boost === "number") {
            setDefaultUctLoopEscapeExploreBoost(data.loop_escape_explore_boost)
          }
        }
      } catch {
        // 静默失败，使用硬编码默认值
      }
    }
    void fetchDefaults()
  }, [])

  useEffect(() => {
    if (pageSource === "screenshot") {
      if (captureScreenshotMode !== "true") {
        setCaptureScreenshotMode("true")
      }
      if (pageControlStrategy === "" || pageControlStrategy === "raw") {
        setPageControlStrategy("ocr")
      }
      if (pageNameStrategy === "structure_fingerprint") {
        setPageNameStrategy("image_fingerprint")
      }
    }
  }, [captureScreenshotMode, pageControlStrategy, pageNameStrategy, pageSource])

  useEffect(() => {
    if (mixedMode) {
      // 混合模式：固定开启截图，策略建议 chain
      if (captureScreenshotMode !== "true") {
        setCaptureScreenshotMode("true")
      }
      if (pageControlStrategy === "" || pageControlStrategy === "raw" || pageControlStrategy === "ocr" || pageControlStrategy === "llm") {
        setPageControlStrategy("chain")
      }
    }
  }, [mixedMode])

  const parsedDump = useMemo(() => parseDumpTree(xmlPreview), [xmlPreview])

  useEffect(() => {
    if (parsedDump.root === null) {
      setSelectedDumpNodeId("")
      setExpandedNodeIds({})
      return
    }
    const expanded: Record<string, boolean> = {}
    for (const id of parsedDump.nodeMap.keys()) {
      expanded[id] = true
    }
    setExpandedNodeIds(expanded)
  }, [parsedDump.root?.id, parsedDump.nodeMap])

  const selectedDumpNode = useMemo(
    () => parsedDump.nodeMap.get(selectedDumpNodeId) ?? null,
    [parsedDump.nodeMap, selectedDumpNodeId]
  )

  const effectiveRange = useMemo<EffectiveRange>(() => {
    const parseWithDefault = (raw: string, fallback: number) => {
      const num = Number(raw)
      if (Number.isNaN(num)) {
        return fallback
      }
      return Math.min(Math.max(num, 0), 1)
    }
    const left = parseWithDefault(rangeLeftInput, 0)
    const top = parseWithDefault(rangeTopInput, 0)
    const right = parseWithDefault(rangeRightInput, 1)
    const bottom = parseWithDefault(rangeBottomInput, 1)
    return { left, top, right, bottom }
  }, [rangeBottomInput, rangeLeftInput, rangeRightInput, rangeTopInput])

  const rangeWidth = Math.max(effectiveRange.right - effectiveRange.left, 0.0001)
  const rangeHeight = Math.max(effectiveRange.bottom - effectiveRange.top, 0.0001)

  const payload: ConfigPayload = useMemo(
    () => ({
      page_source: pageSource,
      mixed_mode: mixedMode,
      page_name_strategy: pageNameStrategy,
      touch_mode: touchMode,
      skip_all_actions_from_model: skipAll,
      page_control_strategy: pageControlStrategy,
      algorithm,
      capture_screenshot: boolModeToValue(captureScreenshotMode),
      keep_step_records: boolModeToValue(keepStepRecordsMode),
      scroll_infer_threshold: parseOptionalNumber(scrollInferThreshold),
      image_similarity_ssim_threshold: parseOptionalNumber(imageSimilarityThreshold),
      image_fingerprint_hamming_threshold: parseOptionalNumber(imageFingerprintHammingThreshold),
      page_control_cache_ttl_seconds: parseOptionalNumber(pageControlCacheTTLSeconds),
      explore_ocr_timeout_ms: parseOptionalNumber(exploreOCRTimeoutMs),
      llm_timeout_ms: parseOptionalNumber(llmTimeoutMs),
      recovery_cooldown_steps: parseOptionalNumber(recoveryCooldownSteps),
      recovery_two_state_loop_threshold: parseOptionalNumber(recoveryTwoStateLoopThreshold),
      recovery_high_visit_threshold: parseOptionalNumber(recoveryHighVisitThreshold),
      recovery_low_reward_window: parseOptionalNumber(recoveryLowRewardWindow),
      candidate_ambiguity_top_gap_threshold: parseOptionalNumber(candidateAmbiguityTopGapThreshold),
      high_value_page_visit_limit: parseOptionalNumber(highValuePageVisitLimit),
      candidate_risk_drop_threshold: parseOptionalNumber(candidateRiskDropThreshold),
      candidate_min_fusion_score: parseOptionalNumber(candidateMinFusionScore),
      input_charset: inputCharset,
      uia: { server_port: Number(uiaPort || 0) },
      poco: { engine: pocoEngine, port: Number(pocoPort || 0) },
      log: { file_level: fileLevel },
      uct_bandit: {
        two_state_loop_penalty: parseOptionalNumber(uctTwoStateLoopPenalty),
        edge_repeat_penalty: parseOptionalNumber(uctEdgeRepeatPenalty),
        edge_repeat_threshold: parseOptionalNumber(uctEdgeRepeatThreshold),
        action_cooldown_penalty: parseOptionalNumber(uctActionCooldownPenalty),
        recent_action_window: parseOptionalNumber(uctRecentActionWindow),
        loop_escape_explore_boost: parseOptionalNumber(uctLoopEscapeExploreBoost),
      },
      reuse: {
        epsilon: parseOptionalNumber(reuseEpsilon),
        gamma: parseOptionalNumber(reuseGamma),
        n_step: parseOptionalNumber(reuseNStep),
        model_save_path: reuseModelSavePath.trim(),
        enable_model_persistence: boolModeToValue(reuseEnableModelPersistenceMode),
        reset_model_on_start: boolModeToValue(reuseResetModelOnStartMode),
      },
      effective_touch_area: {
        serial: usedSerial || deviceSerial || "",
        package_name: currentPackageName || "",
        range: effectiveRange,
      },
    }),
    [
      algorithm,
      candidateAmbiguityTopGapThreshold,
      candidateMinFusionScore,
      candidateRiskDropThreshold,
      captureScreenshotMode,
      currentPackageName,
      deviceSerial,
      effectiveRange,
      exploreOCRTimeoutMs,
      fileLevel,
      highValuePageVisitLimit,
      imageSimilarityThreshold,
      imageFingerprintHammingThreshold,
      keepStepRecordsMode,
      llmTimeoutMs,
      pageControlCacheTTLSeconds,
      pageControlStrategy,
      pageNameStrategy,
      pageSource,
      mixedMode,
      pocoEngine,
      pocoPort,
      recoveryCooldownSteps,
      recoveryHighVisitThreshold,
      recoveryLowRewardWindow,
      recoveryTwoStateLoopThreshold,
      reuseEnableModelPersistenceMode,
      reuseEpsilon,
      reuseGamma,
      reuseModelSavePath,
      reuseNStep,
      reuseResetModelOnStartMode,
      scrollInferThreshold,
      skipAll,
      touchMode,
      uctActionCooldownPenalty,
      uctEdgeRepeatPenalty,
      uctEdgeRepeatThreshold,
      uctLoopEscapeExploreBoost,
      uctRecentActionWindow,
      uctTwoStateLoopPenalty,
      uiaPort,
      usedSerial,
    ]
  )

  const selectedBounds = selectedDumpNode?.bounds ?? null
  const absoluteSpace = useMemo(
    () => ({
      width: imageNaturalSize.width,
      height: imageNaturalSize.height,
      source: "image" as const,
    }),
    [imageNaturalSize.height, imageNaturalSize.width]
  )

  const highlightRect = useMemo(() => {
    if (selectedBounds === null) {
      return null
    }
    const isNormalizedBounds =
      selectedBounds.left >= 0 &&
      selectedBounds.top >= 0 &&
      selectedBounds.right <= 1 &&
      selectedBounds.bottom <= 1
    const coordWidth = imageNaturalSize.width
    const coordHeight = imageNaturalSize.height
    if (coordWidth <= 0 || coordHeight <= 0) {
      return null
    }
    const left = Math.min(
      Math.max(
        isNormalizedBounds
          ? (effectiveRange.left + rangeWidth * selectedBounds.left) * 100
          : (selectedBounds.left / coordWidth) * 100,
        0
      ),
      100
    )
    const top = Math.min(
      Math.max(
        isNormalizedBounds
          ? (effectiveRange.top + rangeHeight * selectedBounds.top) * 100
          : (selectedBounds.top / coordHeight) * 100,
        0
      ),
      100
    )
    const right = Math.min(
      Math.max(
        isNormalizedBounds
          ? (effectiveRange.left + rangeWidth * selectedBounds.right) * 100
          : (selectedBounds.right / coordWidth) * 100,
        0
      ),
      100
    )
    const bottom = Math.min(
      Math.max(
        isNormalizedBounds
          ? (effectiveRange.top + rangeHeight * selectedBounds.bottom) * 100
          : (selectedBounds.bottom / coordHeight) * 100,
        0
      ),
      100
    )
    const width = Math.max(right - left, 0.4)
    const height = Math.max(bottom - top, 0.4)
    return { left, top, width, height, coordWidth, coordHeight, isNormalizedBounds }
  }, [
    effectiveRange.left,
    effectiveRange.top,
    imageNaturalSize.height,
    imageNaturalSize.width,
    rangeHeight,
    rangeWidth,
    selectedBounds,
  ])

  useEffect(() => {
    if (selectedDumpNode === null) {
      setHighlightLog("未选中控件")
      return
    }
    const rawBounds = selectedDumpNode.attrs.bounds || "<none>"
    if (selectedBounds === null) {
      const message = `节点=${selectedDumpNode.id} bounds=${rawBounds}，解析失败或为空`
      setHighlightLog(message)
      console.info("[trek-web] highlight", message)
      return
    }
    if (highlightRect === null) {
      const message = `节点=${selectedDumpNode.id} bounds=${rawBounds}，等待坐标系或截图尺寸`
      setHighlightLog(message)
      console.info("[trek-web] highlight", message)
      return
    }
    const message =
      `节点=${selectedDumpNode.id} bounds=${rawBounds} ` +
      `映射(left=${highlightRect.left.toFixed(2)}%, top=${highlightRect.top.toFixed(2)}%, ` +
      `width=${highlightRect.width.toFixed(2)}%, height=${highlightRect.height.toFixed(2)}%) ` +
      `bounds模式=${highlightRect.isNormalizedBounds ? "归一化(0~1)" : "像素"} ` +
      `有效范围=[${effectiveRange.left.toFixed(3)},${effectiveRange.top.toFixed(3)}]-[${effectiveRange.right.toFixed(3)},${effectiveRange.bottom.toFixed(3)}] ` +
      `坐标系=${highlightRect.coordWidth.toFixed(3)}x${highlightRect.coordHeight.toFixed(3)} ` +
      `截图原始=${imageNaturalSize.width}x${imageNaturalSize.height}`
    setHighlightLog(message)
    console.info("[trek-web] highlight", message)
  }, [
    effectiveRange.bottom,
    effectiveRange.left,
    effectiveRange.right,
    effectiveRange.top,
    highlightRect,
    imageNaturalSize.height,
    imageNaturalSize.width,
    selectedBounds,
    selectedDumpNode,
  ])

  useEffect(() => {
    setClickPoint(null)
    setClickLog("未点击图片")
    setRangeLog("当前范围仅内存生效（不持久化）")
  }, [screenshotBase64])

  const handleClearRange = () => {
    setRangeLeftInput("0")
    setRangeTopInput("0")
    setRangeRightInput("1")
    setRangeBottomInput("1")
    setRangeLog("已恢复整图默认范围（0,0,1,1）")
  }

  const handleImportConfig = (source: string) => {
    const imported = parseConfigSource(source)
    if (
      imported.page_source === "uia" ||
      imported.page_source === "poco" ||
      imported.page_source === "screenshot"
    ) {
      setPageSource(imported.page_source)
    }
    if (typeof imported.mixed_mode === "boolean") {
      setMixedMode(imported.mixed_mode)
    }
    if (isPageNameStrategy(imported.page_name_strategy)) {
      setPageNameStrategy(imported.page_name_strategy)
    }
    if (
      imported.touch_mode === "motion" ||
      imported.touch_mode === "uia" ||
      imported.touch_mode === "adb"
    ) {
      setTouchMode(imported.touch_mode)
    }
    if (
      imported.page_control_strategy === "" ||
      imported.page_control_strategy === "raw" ||
      imported.page_control_strategy === "ocr" ||
      imported.page_control_strategy === "llm"
    ) {
      setPageControlStrategy(imported.page_control_strategy ?? "")
    }
    if (
      imported.algorithm === "" ||
      imported.algorithm === "reuse" ||
      imported.algorithm === "uctbandit" ||
      imported.algorithm === "random"
    ) {
      setAlgorithm(imported.algorithm ?? "")
    }
    if (typeof imported.skip_all_actions_from_model === "boolean") {
      setSkipAll(imported.skip_all_actions_from_model)
    }
    setCaptureScreenshotMode(boolValueToMode(imported.capture_screenshot))
    setKeepStepRecordsMode(boolValueToMode(imported.keep_step_records))
    if (typeof imported.uia?.server_port === "number") {
      setUiaPort(imported.uia.server_port > 0 ? String(imported.uia.server_port) : "")
    }
    if (typeof imported.poco?.engine === "string" && imported.poco.engine.trim() !== "") {
      setPocoEngine(imported.poco.engine.trim())
    }
    if (typeof imported.poco?.port === "number") {
      setPocoPort(imported.poco.port > 0 ? String(imported.poco.port) : "")
    }
    if (typeof imported.log?.file_level === "string") {
      setFileLevel(imported.log.file_level.trim())
    }
    setScrollInferThreshold(imported.scroll_infer_threshold != null ? String(imported.scroll_infer_threshold) : "")
    setImageSimilarityThreshold(imported.image_similarity_ssim_threshold != null ? String(imported.image_similarity_ssim_threshold) : "")
    setImageFingerprintHammingThreshold(imported.image_fingerprint_hamming_threshold != null ? String(imported.image_fingerprint_hamming_threshold) : "")
    setPageControlCacheTTLSeconds(imported.page_control_cache_ttl_seconds != null ? String(imported.page_control_cache_ttl_seconds) : "")
    setExploreOCRTimeoutMs(imported.explore_ocr_timeout_ms != null ? String(imported.explore_ocr_timeout_ms) : "")
    setLLMTimeoutMs(imported.llm_timeout_ms != null ? String(imported.llm_timeout_ms) : "")
    setRecoveryCooldownSteps(imported.recovery_cooldown_steps != null ? String(imported.recovery_cooldown_steps) : "")
    setRecoveryTwoStateLoopThreshold(imported.recovery_two_state_loop_threshold != null ? String(imported.recovery_two_state_loop_threshold) : "")
    setRecoveryHighVisitThreshold(imported.recovery_high_visit_threshold != null ? String(imported.recovery_high_visit_threshold) : "")
    setRecoveryLowRewardWindow(imported.recovery_low_reward_window != null ? String(imported.recovery_low_reward_window) : "")
    setCandidateAmbiguityTopGapThreshold(imported.candidate_ambiguity_top_gap_threshold != null ? String(imported.candidate_ambiguity_top_gap_threshold) : "")
    setHighValuePageVisitLimit(imported.high_value_page_visit_limit != null ? String(imported.high_value_page_visit_limit) : "")
    setCandidateRiskDropThreshold(imported.candidate_risk_drop_threshold != null ? String(imported.candidate_risk_drop_threshold) : "")
    setCandidateMinFusionScore(imported.candidate_min_fusion_score != null ? String(imported.candidate_min_fusion_score) : "")
    setUctTwoStateLoopPenalty(imported.uct_bandit?.two_state_loop_penalty != null ? String(imported.uct_bandit.two_state_loop_penalty) : "")
    setUctEdgeRepeatPenalty(imported.uct_bandit?.edge_repeat_penalty != null ? String(imported.uct_bandit.edge_repeat_penalty) : "")
    setUctEdgeRepeatThreshold(imported.uct_bandit?.edge_repeat_threshold != null ? String(imported.uct_bandit.edge_repeat_threshold) : "")
    setUctActionCooldownPenalty(imported.uct_bandit?.action_cooldown_penalty != null ? String(imported.uct_bandit.action_cooldown_penalty) : "")
    setUctRecentActionWindow(imported.uct_bandit?.recent_action_window != null ? String(imported.uct_bandit.recent_action_window) : "")
    setUctLoopEscapeExploreBoost(imported.uct_bandit?.loop_escape_explore_boost != null ? String(imported.uct_bandit.loop_escape_explore_boost) : "")
    setReuseEpsilon(imported.reuse?.epsilon != null ? String(imported.reuse.epsilon) : "")
    setReuseGamma(imported.reuse?.gamma != null ? String(imported.reuse.gamma) : "")
    setReuseNStep(imported.reuse?.n_step != null ? String(imported.reuse.n_step) : "")
    setReuseModelSavePath(typeof imported.reuse?.model_save_path === "string" ? imported.reuse.model_save_path : "")
    setReuseEnableModelPersistenceMode(boolValueToMode(imported.reuse?.enable_model_persistence))
    setReuseResetModelOnStartMode(boolValueToMode(imported.reuse?.reset_model_on_start))
    if (typeof imported.effective_touch_area?.serial === "string") {
      setDeviceSerial(imported.effective_touch_area.serial.trim())
      setUsedSerial(imported.effective_touch_area.serial.trim())
    }
    if (typeof imported.effective_touch_area?.package_name === "string") {
      setCurrentPackageName(imported.effective_touch_area.package_name.trim())
    }
    const importedRange = imported.effective_touch_area?.range
    if (importedRange !== undefined) {
      if (typeof importedRange.left === "number") {
        setRangeLeftInput(String(importedRange.left))
      }
      if (typeof importedRange.top === "number") {
        setRangeTopInput(String(importedRange.top))
      }
      if (typeof importedRange.right === "number") {
        setRangeRightInput(String(importedRange.right))
      }
      if (typeof importedRange.bottom === "number") {
        setRangeBottomInput(String(importedRange.bottom))
      }
      setRangeLog("已从导入配置加载有效触控区域")
    }
    setStatus("配置已导入")
    setConfigTab("base")
  }

  const handleCopyConfig = async () => {
    const text = resultText.trim()
    if (text === "") {
      setError("当前没有可复制的配置")
      return
    }
    await copyText(resultText)
    setStatus("配置已复制")
    setError("")
  }

  const renderConfigText = async () => {
    const rendered = await postJSON<{ js: string }>("/api/render", payload)
    setResultText(rendered.js)
    return rendered.js
  }

  const handleOpenSavePreview = async () => {
    setLoading(true)
    setStatus("")
    setError("")
    try {
      await renderConfigText()
      setSavePreviewOpen(true)
      setStatus("已生成配置预览")
    } catch (err) {
      setError(err instanceof Error ? err.message : "生成预览失败")
    } finally {
      setLoading(false)
    }
  }

  const handleDownloadConfig = async () => {
    setLoading(true)
    setStatus("")
    setError("")
    try {
      const text = resultText.trim() === "" ? await renderConfigText() : resultText
      const fileName = "config.generated.js"
      const blob = new Blob([text], { type: "application/javascript;charset=utf-8" })
      const url = URL.createObjectURL(blob)
      const anchor = document.createElement("a")
      anchor.href = url
      anchor.download = fileName
      document.body.appendChild(anchor)
      anchor.click()
      anchor.remove()
      URL.revokeObjectURL(url)
      setStatus(`已下载配置: ${fileName}`)
    } catch (err) {
      setError(err instanceof Error ? err.message : "下载失败")
    } finally {
      setLoading(false)
    }
  }

  const handleRefreshPreview = async () => {
    setLoading(true)
    setStatus("")
    setError("")
    try {
      const data = await postJSON<{
        used_serial: string
        xml: string
        screenshot_base64: string
        package_name?: string
        page_name?: string
      }>(
        "/api/preview",
        {
          serial: deviceSerial,
          config: payload,
        }
      )
      setUsedSerial(data.used_serial || "")
      setCurrentPackageName(data.package_name || "")
      setXmlPreview(data.xml)
      setScreenshotBase64(data.screenshot_base64)
      setStatus(`预览已刷新，当前设备序列号: ${data.used_serial || "未知"}`)
    } catch (err) {
      setError(err instanceof Error ? err.message : "刷新预览失败")
    } finally {
      setLoading(false)
    }
  }

  const toggleNodeExpanded = (id: string) => {
    setExpandedNodeIds((prev) => ({
      ...prev,
      [id]: !prev[id],
    }))
  }

  const handleScreenshotClick = (event: MouseEvent<HTMLImageElement>) => {
    const rect = event.currentTarget.getBoundingClientRect()
    if (rect.width <= 0 || rect.height <= 0) {
      return
    }
    const rawX = (event.clientX - rect.left) / rect.width
    const rawY = (event.clientY - rect.top) / rect.height
    const ratioX = Math.min(Math.max(rawX, 0), 1)
    const ratioY = Math.min(Math.max(rawY, 0), 1)
    const normalizedX = Math.min(Math.max((ratioX - effectiveRange.left) / rangeWidth, 0), 1)
    const normalizedY = Math.min(Math.max((ratioY - effectiveRange.top) / rangeHeight, 0), 1)
    const point: ClickPoint = {
      imagePercentX: ratioX,
      imagePercentY: ratioY,
      percentX: normalizedX,
      percentY: normalizedY,
      absoluteX: ratioX * imageNaturalSize.width,
      absoluteY: ratioY * imageNaturalSize.height,
    }
    setClickPoint(point)
    const message =
      `有效区百分比(0~1)=(${point.percentX.toFixed(6)}, ${point.percentY.toFixed(6)}), ` +
      `整图百分比(0~1)=(${point.imagePercentX.toFixed(6)}, ${point.imagePercentY.toFixed(6)}), ` +
      `绝对坐标(设备原始)=(${point.absoluteX.toFixed(1)}, ${point.absoluteY.toFixed(1)}), ` +
      `映射公式=x'=${effectiveRange.left.toFixed(3)}+(${rangeWidth.toFixed(3)})*x, y'=${effectiveRange.top.toFixed(3)}+(${rangeHeight.toFixed(3)})*y`
    setClickLog(message)
    console.info("[trek-web] click-point", message)
  }

  const renderConfigPanel = () => (
    <ConfigPanel
      configTab={configTab}
      setConfigTab={setConfigTab}
      loading={loading}
      onImportConfig={handleImportConfig}
      onRefreshCurrentPage={handleRefreshPreview}
      deviceSerial={deviceSerial}
      setDeviceSerial={setDeviceSerial}
      deviceOptions={deviceOptions}
      loadingDevices={loadingDevices}
      onRefreshDevices={() => void fetchDevices()}
      usedSerial={usedSerial}
      currentPackageName={currentPackageName}
      pageSource={pageSource}
      setPageSource={setPageSource}
      mixedMode={mixedMode}
      setMixedMode={setMixedMode}
      pageNameStrategy={pageNameStrategy}
      setPageNameStrategy={setPageNameStrategy}
      touchMode={touchMode}
      setTouchMode={setTouchMode}
      pageControlStrategy={pageControlStrategy}
      setPageControlStrategy={setPageControlStrategy}
      algorithm={algorithm}
      setAlgorithm={setAlgorithm}
      captureScreenshotMode={captureScreenshotMode}
      setCaptureScreenshotMode={setCaptureScreenshotMode}
      keepStepRecordsMode={keepStepRecordsMode}
      setKeepStepRecordsMode={setKeepStepRecordsMode}
      uiaPort={uiaPort}
      setUiaPort={setUiaPort}
      fileLevel={fileLevel}
      setFileLevel={setFileLevel}
      pocoEngine={pocoEngine}
      setPocoEngine={setPocoEngine}
      pocoPort={pocoPort}
      setPocoPort={setPocoPort}
      scrollInferThreshold={scrollInferThreshold}
      setScrollInferThreshold={setScrollInferThreshold}
      imageSimilarityThreshold={imageSimilarityThreshold}
      setImageSimilarityThreshold={setImageSimilarityThreshold}
      imageFingerprintHammingThreshold={imageFingerprintHammingThreshold}
      setImageFingerprintHammingThreshold={setImageFingerprintHammingThreshold}
      pageControlCacheTTLSeconds={pageControlCacheTTLSeconds}
      setPageControlCacheTTLSeconds={setPageControlCacheTTLSeconds}
      exploreOCRTimeoutMs={exploreOCRTimeoutMs}
      setExploreOCRTimeoutMs={setExploreOCRTimeoutMs}
      llmTimeoutMs={llmTimeoutMs}
      setLLMTimeoutMs={setLLMTimeoutMs}
      recoveryCooldownSteps={recoveryCooldownSteps}
      setRecoveryCooldownSteps={setRecoveryCooldownSteps}
      recoveryTwoStateLoopThreshold={recoveryTwoStateLoopThreshold}
      setRecoveryTwoStateLoopThreshold={setRecoveryTwoStateLoopThreshold}
      recoveryHighVisitThreshold={recoveryHighVisitThreshold}
      setRecoveryHighVisitThreshold={setRecoveryHighVisitThreshold}
      recoveryLowRewardWindow={recoveryLowRewardWindow}
      setRecoveryLowRewardWindow={setRecoveryLowRewardWindow}
      candidateAmbiguityTopGapThreshold={candidateAmbiguityTopGapThreshold}
      setCandidateAmbiguityTopGapThreshold={setCandidateAmbiguityTopGapThreshold}
      highValuePageVisitLimit={highValuePageVisitLimit}
      setHighValuePageVisitLimit={setHighValuePageVisitLimit}
      candidateRiskDropThreshold={candidateRiskDropThreshold}
      setCandidateRiskDropThreshold={setCandidateRiskDropThreshold}
      candidateMinFusionScore={candidateMinFusionScore}
      setCandidateMinFusionScore={setCandidateMinFusionScore}
      inputCharset={inputCharset}
      setInputCharset={setInputCharset}
      uctTwoStateLoopPenalty={uctTwoStateLoopPenalty}
      setUctTwoStateLoopPenalty={setUctTwoStateLoopPenalty}
      uctEdgeRepeatPenalty={uctEdgeRepeatPenalty}
      setUctEdgeRepeatPenalty={setUctEdgeRepeatPenalty}
      uctEdgeRepeatThreshold={uctEdgeRepeatThreshold}
      setUctEdgeRepeatThreshold={setUctEdgeRepeatThreshold}
      uctActionCooldownPenalty={uctActionCooldownPenalty}
      setUctActionCooldownPenalty={setUctActionCooldownPenalty}
      uctRecentActionWindow={uctRecentActionWindow}
      setUctRecentActionWindow={setUctRecentActionWindow}
      uctLoopEscapeExploreBoost={uctLoopEscapeExploreBoost}
      setUctLoopEscapeExploreBoost={setUctLoopEscapeExploreBoost}
      defaultScrollInferThreshold={defaultScrollInferThreshold}
      defaultImageSimilarityThreshold={defaultImageSimilarityThreshold}
      defaultImageFingerprintHammingThreshold={defaultImageFingerprintHammingThreshold}
      defaultPageControlCacheTTLSeconds={defaultPageControlCacheTTLSeconds}
      defaultExploreOCRTimeoutMs={defaultExploreOCRTimeoutMs}
      defaultLLMTimeoutMs={defaultLLMTimeoutMs}
      defaultUctTwoStateLoopPenalty={defaultUctTwoStateLoopPenalty}
      defaultUctEdgeRepeatPenalty={defaultUctEdgeRepeatPenalty}
      defaultUctEdgeRepeatThreshold={defaultUctEdgeRepeatThreshold}
      defaultUctActionCooldownPenalty={defaultUctActionCooldownPenalty}
      defaultUctRecentActionWindow={defaultUctRecentActionWindow}
      defaultUctLoopEscapeExploreBoost={defaultUctLoopEscapeExploreBoost}
      reuseEpsilon={reuseEpsilon}
      setReuseEpsilon={setReuseEpsilon}
      reuseGamma={reuseGamma}
      setReuseGamma={setReuseGamma}
      reuseNStep={reuseNStep}
      setReuseNStep={setReuseNStep}
      reuseModelSavePath={reuseModelSavePath}
      setReuseModelSavePath={setReuseModelSavePath}
      reuseEnableModelPersistenceMode={reuseEnableModelPersistenceMode}
      setReuseEnableModelPersistenceMode={setReuseEnableModelPersistenceMode}
      reuseResetModelOnStartMode={reuseResetModelOnStartMode}
      setReuseResetModelOnStartMode={setReuseResetModelOnStartMode}
      skipAll={skipAll}
      setSkipAll={setSkipAll}
      rangeLeftInput={rangeLeftInput}
      setRangeLeftInput={setRangeLeftInput}
      rangeTopInput={rangeTopInput}
      setRangeTopInput={setRangeTopInput}
      rangeRightInput={rangeRightInput}
      setRangeRightInput={setRangeRightInput}
      rangeBottomInput={rangeBottomInput}
      setRangeBottomInput={setRangeBottomInput}
      rangeLog={rangeLog}
      onResetRange={handleClearRange}
      onCopyConfig={() => void handleCopyConfig()}
      onDownloadConfig={() => void handleDownloadConfig()}
      savePreviewOpen={savePreviewOpen}
      setSavePreviewOpen={setSavePreviewOpen}
      status={status}
      error={error}
      resultText={resultText}
    />
  )

  return (
    <div className="mx-auto flex min-h-svh w-[calc(100vw-2rem)] max-w-[1800px] flex-col gap-4 p-4 lg:p-6">
      <section className="rounded-xl border bg-card p-4">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <h1 className="text-xl font-semibold">Trek 配置生成器</h1>
          <div className="flex flex-wrap items-center justify-end gap-2">
            <Button type="button" variant="outline" onClick={() => void handleOpenSavePreview()} disabled={loading}>
              导出配置
            </Button>
            <Button type="button" variant="outline" onClick={() => document.dispatchEvent(new Event("trek-open-import-config"))} disabled={loading}>
              导入配置
            </Button>
            <Button type="button" variant="outline" onClick={() => void handleRefreshPreview()} disabled={loading}>
              抓取当前设备界面
            </Button>
          </div>
        </div>
      </section>

      <section className="grid grid-cols-1 gap-4 xl:hidden">
        <div className="rounded-xl border bg-card p-4">
          <div className="mb-3 flex flex-col gap-2">
            <h2 className="text-base font-semibold">界面截图</h2>
          </div>
          <ScreenshotPanel
            screenshotBase64={screenshotBase64}
            imgClassName="max-h-[520px] max-w-full rounded object-contain"
            effectiveRange={effectiveRange}
            rangeWidth={rangeWidth}
            rangeHeight={rangeHeight}
            highlightRect={
              highlightRect === null
                ? null
                : {
                    left: highlightRect.left,
                    top: highlightRect.top,
                    width: highlightRect.width,
                    height: highlightRect.height,
                  }
            }
            clickPoint={clickPoint}
            highlightLog={highlightLog}
            clickLog={clickLog}
            absoluteWidth={absoluteSpace.width}
            absoluteHeight={absoluteSpace.height}
            onImageLoad={(width, height) => setImageNaturalSize({ width, height })}
            onImageClick={handleScreenshotClick}
            onCopyText={copyText}
          />
        </div>

        <div className="rounded-xl border bg-card p-4">
          <h2 className="mb-3 text-base font-semibold">Dump</h2>
          <DumpPanel
            root={parsedDump.root}
            heightClassName="min-h-[640px]"
            expandedNodeIds={expandedNodeIds}
            selectedDumpNodeId={selectedDumpNodeId}
            onToggleNode={toggleNodeExpanded}
            onSelectNode={setSelectedDumpNodeId}
          />
        </div>

        <div className="rounded-xl border bg-card p-4">{renderConfigPanel()}</div>
      </section>

      <section className="hidden xl:block">
        <ResizablePanelGroup
          className="min-h-[760px] w-full"
          orientation="horizontal"
        >
          <ResizablePanel defaultSize={34} minSize={20}>
            <div className="h-full rounded-xl border bg-card p-4">
              <div className="mb-3 flex flex-col gap-2">
                <h2 className="text-base font-semibold">界面截图</h2>
              </div>
              <ScreenshotPanel
                screenshotBase64={screenshotBase64}
                imgClassName="max-h-[680px] max-w-full rounded object-contain"
                effectiveRange={effectiveRange}
                rangeWidth={rangeWidth}
                rangeHeight={rangeHeight}
                highlightRect={
                  highlightRect === null
                    ? null
                    : {
                        left: highlightRect.left,
                        top: highlightRect.top,
                        width: highlightRect.width,
                        height: highlightRect.height,
                      }
                }
                clickPoint={clickPoint}
                highlightLog={highlightLog}
                clickLog={clickLog}
                absoluteWidth={absoluteSpace.width}
                absoluteHeight={absoluteSpace.height}
                onImageLoad={(width, height) => setImageNaturalSize({ width, height })}
                onImageClick={handleScreenshotClick}
                onCopyText={copyText}
              />
            </div>
          </ResizablePanel>
          <ResizableHandle withHandle className="mx-2 rounded-full" />
          <ResizablePanel defaultSize={33} minSize={20}>
            <div className="flex h-full flex-col rounded-xl border bg-card p-4">
              <h2 className="mb-3 text-base font-semibold">Dump</h2>
              <DumpPanel
                root={parsedDump.root}
                heightClassName="h-full flex-1"
                expandedNodeIds={expandedNodeIds}
                selectedDumpNodeId={selectedDumpNodeId}
                onToggleNode={toggleNodeExpanded}
                onSelectNode={setSelectedDumpNodeId}
              />
            </div>
          </ResizablePanel>
          <ResizableHandle withHandle className="mx-2 rounded-full" />
          <ResizablePanel defaultSize={33} minSize={22}>
            <div className="h-full overflow-y-auto rounded-xl border bg-card p-4">{renderConfigPanel()}</div>
          </ResizablePanel>
        </ResizablePanelGroup>
      </section>
    </div>
  )
}

export default App



