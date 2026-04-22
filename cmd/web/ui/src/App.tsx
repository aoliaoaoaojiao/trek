import { useEffect, useMemo, useState } from "react"
import type { MouseEvent } from "react"

import {
  ResizableHandle,
  ResizablePanel,
  ResizablePanelGroup,
} from "@/components/ui/resizable"
import { ConfigPanel } from "@/monkey-config/ConfigPanel"
import { DumpPanel } from "@/monkey-config/DumpPanel"
import { ScreenshotPanel } from "@/monkey-config/ScreenshotPanel"
import { API_BASE, DEV_API_BASE, postJSON } from "@/monkey-config/api"
import { copyText, parseDumpTree } from "@/monkey-config/utils"
import type {
  ActionType,
  ClickPoint,
  ConfigPayload,
  DeviceOption,
  EffectiveRange,
  PageActionRule,
  PageNameStrategy,
  PartialConfigPayload,
} from "@/monkey-config/types"

const pageNameStrategies: PageNameStrategy[] = [
  "",
  "uia_activity_first",
  "xml_only",
  "xml_fingerprint",
  "structure_fingerprint",
  "activity_only",
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

export function App() {
  const [pageSource, setPageSource] = useState<"uia" | "poco">("uia")
  const [pageNameStrategy, setPageNameStrategy] = useState<PageNameStrategy>("structure_fingerprint")
  const [touchMode, setTouchMode] = useState<"motion" | "uia" | "adb">("motion")
  const [deviceSerial, setDeviceSerial] = useState("")
  const [deviceOptions, setDeviceOptions] = useState<DeviceOption[]>([])
  const [loadingDevices, setLoadingDevices] = useState(false)
  const [skipAll, setSkipAll] = useState(false)
  const [uiaPort, setUiaPort] = useState("")
  const [pocoEngine, setPocoEngine] = useState("UNITY_3D")
  const [pocoPort, setPocoPort] = useState("")
  const [fileLevel, setFileLevel] = useState("info")
  const [outputPath, setOutputPath] = useState("./config.generated.js")
  const [resultText, setResultText] = useState("")
  const [xmlPreview, setXmlPreview] = useState("")
  const [screenshotBase64, setScreenshotBase64] = useState("")
  const [usedSerial, setUsedSerial] = useState("")
  const [currentPackageName, setCurrentPackageName] = useState("")
  const [currentPageName, setCurrentPageName] = useState("")
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
  const [configTab, setConfigTab] = useState<"base" | "action" | "preview">("base")
  const [actionType, setActionType] = useState<ActionType>("click")
  const [actionPath, setActionPath] = useState("")
  const [actionX, setActionX] = useState("")
  const [actionY, setActionY] = useState("")
  const [actionStartX, setActionStartX] = useState("")
  const [actionStartY, setActionStartY] = useState("")
  const [actionEndX, setActionEndX] = useState("")
  const [actionEndY, setActionEndY] = useState("")
  const [actionRules, setActionRules] = useState<PageActionRule[]>([])
  const [actionLog, setActionLog] = useState("暂无页面动作配置")

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
      page_name_strategy: pageNameStrategy,
      touch_mode: touchMode,
      skip_all_actions_from_model: skipAll,
      uia: { server_port: Number(uiaPort || 0) },
      poco: { engine: pocoEngine, port: Number(pocoPort || 0) },
      log: { file_level: fileLevel },
      effective_touch_area: {
        serial: usedSerial || deviceSerial || "",
        package_name: currentPackageName || "",
        range: effectiveRange,
      },
    }),
    [currentPackageName, deviceSerial, effectiveRange, fileLevel, pageNameStrategy, pageSource, pocoEngine, pocoPort, skipAll, touchMode, uiaPort, usedSerial]
  )

  useEffect(() => {
    if (configTab !== "preview") {
      return
    }
    let ignored = false
    const renderConfig = async () => {
      setLoading(true)
      setStatus("")
      setError("")
      try {
        const data = await postJSON<{ js: string }>("/api/render", payload)
        if (ignored) {
          return
        }
        setResultText(data.js)
        setStatus("已生成配置")
      } catch (err) {
        if (!ignored) {
          setError(err instanceof Error ? err.message : "生成失败")
        }
      } finally {
        if (!ignored) {
          setLoading(false)
        }
      }
    }
    void renderConfig()
    return () => {
      ignored = true
    }
  }, [configTab, payload])

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
    if (imported.page_source === "uia" || imported.page_source === "poco") {
      setPageSource(imported.page_source)
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
    if (typeof imported.skip_all_actions_from_model === "boolean") {
      setSkipAll(imported.skip_all_actions_from_model)
    }
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

  const handleSave = async () => {
    setLoading(true)
    setStatus("")
    setError("")
    try {
      const rendered = await postJSON<{ js: string }>("/api/render", payload)
      setResultText(rendered.js)

      const saved = await postJSON<{ output_path: string; message: string }>(
        "/api/save",
        {
          config: payload,
          output_path: outputPath,
        }
      )
      setStatus(`${saved.message}: ${saved.output_path}`)
    } catch (err) {
      setError(err instanceof Error ? err.message : "保存失败")
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
      setCurrentPageName(data.page_name || "")
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

  const handleAddActionRule = () => {
    const page = currentPageName.trim()
    if (page === "") {
      setActionLog("添加失败：请先点击当前界面获取页面名")
      return
    }
    const path = actionPath.trim()
    const toNum = (raw: string) => {
      const n = Number(raw)
      return Number.isNaN(n) ? null : n
    }
    let rule: PageActionRule | null = null
    if (actionType === "scroll") {
      const sx = toNum(actionStartX)
      const sy = toNum(actionStartY)
      const ex = toNum(actionEndX)
      const ey = toNum(actionEndY)
      if (sx === null || sy === null || ex === null || ey === null) {
        setActionLog("添加失败：滑动必须填写开始/结束坐标")
        return
      }
      rule = { page_name: page, action_type: actionType, path: path || undefined, start: { x: sx, y: sy }, end: { x: ex, y: ey } }
    } else {
      const x = toNum(actionX)
      const y = toNum(actionY)
      if (path === "" && (x === null || y === null)) {
        setActionLog("添加失败：path 和 坐标必须至少填写一个")
        return
      }
      rule = { page_name: page, action_type: actionType, path: path || undefined }
      if (x !== null && y !== null) {
        rule.point = { x, y }
      }
    }
    const next = [...actionRules, rule]
    setActionRules(next)
    setActionLog(`已添加动作，当前共 ${next.length} 条`)
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
      currentPageName={currentPageName}
      pageSource={pageSource}
      setPageSource={setPageSource}
      pageNameStrategy={pageNameStrategy}
      setPageNameStrategy={setPageNameStrategy}
      touchMode={touchMode}
      setTouchMode={setTouchMode}
      uiaPort={uiaPort}
      setUiaPort={setUiaPort}
      fileLevel={fileLevel}
      setFileLevel={setFileLevel}
      pocoEngine={pocoEngine}
      setPocoEngine={setPocoEngine}
      pocoPort={pocoPort}
      setPocoPort={setPocoPort}
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
      actionType={actionType}
      setActionType={setActionType}
      actionPath={actionPath}
      setActionPath={setActionPath}
      actionX={actionX}
      setActionX={setActionX}
      actionY={actionY}
      setActionY={setActionY}
      actionStartX={actionStartX}
      setActionStartX={setActionStartX}
      actionStartY={actionStartY}
      setActionStartY={setActionStartY}
      actionEndX={actionEndX}
      setActionEndX={setActionEndX}
      actionEndY={actionEndY}
      setActionEndY={setActionEndY}
      actionRules={actionRules}
      actionLog={actionLog}
      onAddActionRule={handleAddActionRule}
      outputPath={outputPath}
      setOutputPath={setOutputPath}
      onCopyConfig={() => void handleCopyConfig()}
      onSaveConfig={() => void handleSave()}
      status={status}
      error={error}
      resultText={resultText}
    />
  )

  return (
    <div className="mx-auto flex min-h-svh w-[calc(100vw-2rem)] max-w-[1800px] flex-col gap-4 p-4 lg:p-6">
      <section className="rounded-xl border bg-card p-4">
        <h1 className="text-xl font-semibold">Trek 配置 JS 生成器</h1>
        <p className="mt-1 text-sm text-muted-foreground">
          开发模式连接 {DEV_API_BASE}，生产模式自动使用
          <code className="mx-1">window.location.host</code> 作为后端地址。
        </p>
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



