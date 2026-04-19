import { useEffect, useMemo, useState } from "react"

import { Button } from "@/components/ui/button"
import {
  ResizableHandle,
  ResizablePanel,
  ResizablePanelGroup,
} from "@/components/ui/resizable"

type ConfigPayload = {
  page_source: "uia" | "poco"
  touch_mode: "motion" | "uia" | "adb"
  skip_all_actions_from_model: boolean
  uia: { server_port: number }
  poco: { engine: string; port: number }
  log: { file_level: string }
}

type DeviceOption = {
  serial: string
  label: string
}

type BoundsRect = {
  left: number
  top: number
  right: number
  bottom: number
}

type DumpTreeNode = {
  id: string
  tag: string
  attrs: Record<string, string>
  bounds: BoundsRect | null
  children: DumpTreeNode[]
}

type ClickPoint = {
  percentX: number
  percentY: number
  absoluteX: number
  absoluteY: number
}

const DEV_API_BASE = "http://127.0.0.1:17888"

const API_BASE = import.meta.env.DEV
  ? DEV_API_BASE
  : `${window.location.protocol}//${window.location.host}`

async function postJSON<T>(path: string, payload: unknown): Promise<T> {
  const response = await fetch(`${API_BASE}${path}`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  })
  const data = (await response.json()) as T & { error?: string }
  if (!response.ok) {
    throw new Error(data.error ?? `请求失败: ${response.status}`)
  }
  return data
}

function parseBounds(raw: string): BoundsRect | null {
  const matched = raw.match(
    /\[\s*(-?\d*\.?\d+)\s*,\s*(-?\d*\.?\d+)\s*\]\s*\[\s*(-?\d*\.?\d+)\s*,\s*(-?\d*\.?\d+)\s*\]/
  )
  if (matched === null) {
    return null
  }
  const left = Number(matched[1])
  const top = Number(matched[2])
  const right = Number(matched[3])
  const bottom = Number(matched[4])
  if (Number.isNaN(left) || Number.isNaN(top) || Number.isNaN(right) || Number.isNaN(bottom)) {
    return null
  }
  if (right <= left || bottom <= top) {
    return null
  }
  return { left, top, right, bottom }
}

function parseDumpTree(xml: string): { root: DumpTreeNode | null; nodeMap: Map<string, DumpTreeNode> } {
  const nodeMap = new Map<string, DumpTreeNode>()
  if (xml.trim() === "") {
    return { root: null, nodeMap }
  }

  try {
    const parser = new DOMParser()
    const doc = parser.parseFromString(xml, "application/xml")
    const parserError = doc.querySelector("parsererror")
    if (parserError !== null) {
      return { root: null, nodeMap }
    }

    const build = (element: Element, path: string): DumpTreeNode => {
      const attrs: Record<string, string> = {}
      for (const attr of Array.from(element.attributes)) {
        attrs[attr.name] = attr.value
      }
      const bounds = attrs.bounds !== undefined ? parseBounds(attrs.bounds) : null
      const node: DumpTreeNode = {
        id: path,
        tag: element.tagName,
        attrs,
        bounds,
        children: [],
      }
      nodeMap.set(path, node)
      node.children = Array.from(element.children).map((child, index) =>
        build(child, `${path}/${child.tagName}[${index}]`)
      )
      return node
    }

    const rootElement = doc.documentElement
    if (rootElement === null) {
      return { root: null, nodeMap }
    }
    const root = build(rootElement, `/${rootElement.tagName}[0]`)
    return { root, nodeMap }
  } catch {
    return { root: null, nodeMap }
  }
}

function buildNodeTitle(node: DumpTreeNode): string {
  const className = node.attrs.class
  const name = node.attrs.name
  const text = node.attrs.text
  const contentDesc = node.attrs["content-desc"]
  const resourceId = node.attrs["resource-id"]
  const key = className || name || resourceId || contentDesc || text || "<empty>"
  return `${node.tag}  ${key}`
}

export function App() {
  const [pageSource, setPageSource] = useState<"uia" | "poco">("uia")
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
  const [status, setStatus] = useState("")
  const [error, setError] = useState("")
  const [loading, setLoading] = useState(false)
  const [selectedDumpNodeId, setSelectedDumpNodeId] = useState("")
  const [expandedNodeIds, setExpandedNodeIds] = useState<Record<string, boolean>>({})
  const [imageNaturalSize, setImageNaturalSize] = useState({ width: 0, height: 0 })
  const [highlightLog, setHighlightLog] = useState("未选中控件")
  const [clickPoint, setClickPoint] = useState<ClickPoint | null>(null)
  const [clickLog, setClickLog] = useState("未点击图片")
  const [copyStatus, setCopyStatus] = useState("")

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

  const payload: ConfigPayload = useMemo(
    () => ({
      page_source: pageSource,
      touch_mode: touchMode,
      skip_all_actions_from_model: skipAll,
      uia: { server_port: Number(uiaPort || 0) },
      poco: { engine: pocoEngine, port: Number(pocoPort || 0) },
      log: { file_level: fileLevel },
    }),
    [fileLevel, pageSource, pocoEngine, pocoPort, skipAll, touchMode, uiaPort]
  )

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
      Math.max(isNormalizedBounds ? selectedBounds.left * 100 : (selectedBounds.left / coordWidth) * 100, 0),
      100
    )
    const top = Math.min(
      Math.max(isNormalizedBounds ? selectedBounds.top * 100 : (selectedBounds.top / coordHeight) * 100, 0),
      100
    )
    const right = Math.min(
      Math.max(isNormalizedBounds ? selectedBounds.right * 100 : (selectedBounds.right / coordWidth) * 100, 0),
      100
    )
    const bottom = Math.min(
      Math.max(isNormalizedBounds ? selectedBounds.bottom * 100 : (selectedBounds.bottom / coordHeight) * 100, 0),
      100
    )
    const width = Math.max(right - left, 0.4)
    const height = Math.max(bottom - top, 0.4)
    return { left, top, width, height, coordWidth, coordHeight, isNormalizedBounds }
  }, [imageNaturalSize.height, imageNaturalSize.width, selectedBounds])

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
      `坐标系=${highlightRect.coordWidth.toFixed(3)}x${highlightRect.coordHeight.toFixed(3)} ` +
      `截图原始=${imageNaturalSize.width}x${imageNaturalSize.height}`
    setHighlightLog(message)
    console.info("[trek-web] highlight", message)
  }, [highlightRect, imageNaturalSize.height, imageNaturalSize.width, selectedBounds, selectedDumpNode])

  useEffect(() => {
    setClickPoint(null)
    setClickLog("未点击图片")
    setCopyStatus("")
  }, [screenshotBase64])

  const copyText = async (text: string) => {
    try {
      if (navigator.clipboard && window.isSecureContext) {
        await navigator.clipboard.writeText(text)
      } else {
        const textArea = document.createElement("textarea")
        textArea.value = text
        textArea.style.position = "fixed"
        textArea.style.opacity = "0"
        document.body.appendChild(textArea)
        textArea.focus()
        textArea.select()
        document.execCommand("copy")
        document.body.removeChild(textArea)
      }
      setCopyStatus("已复制")
    } catch {
      setCopyStatus("复制失败")
    }
  }

  const handlePreview = async () => {
    setLoading(true)
    setStatus("")
    setError("")
    try {
      const data = await postJSON<{ js: string }>("/api/render", payload)
      setResultText(data.js)
      setStatus("已生成配置")
    } catch (err) {
      setError(err instanceof Error ? err.message : "生成失败")
    } finally {
      setLoading(false)
    }
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
      }>(
        "/api/preview",
        {
          serial: deviceSerial,
          config: payload,
        }
      )
      setUsedSerial(data.used_serial || "")
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

  const renderDumpTree = (node: DumpTreeNode, depth: number) => {
    const hasChildren = node.children.length > 0
    const expanded = expandedNodeIds[node.id] ?? false
    const selected = selectedDumpNodeId === node.id
    return (
      <li key={node.id}>
        <div
          className={`mb-1 flex items-start gap-1 rounded px-1 py-0.5 ${
            selected ? "bg-emerald-100" : ""
          }`}
          style={{ marginLeft: `${depth * 12}px` }}
        >
          {hasChildren ? (
            <button
              type="button"
              className="mt-[2px] h-5 w-5 rounded border text-xs"
              onClick={() => toggleNodeExpanded(node.id)}
              aria-label={expanded ? "收起节点" : "展开节点"}
            >
              {expanded ? "-" : "+"}
            </button>
          ) : (
            <span className="inline-block h-5 w-5 text-center text-xs text-muted-foreground">
              ·
            </span>
          )}
          <button
            type="button"
            className="min-w-0 flex-1 text-left font-mono text-xs"
            onClick={() => setSelectedDumpNodeId(node.id)}
            title={node.attrs.bounds || ""}
          >
            <div className="truncate">{buildNodeTitle(node)}</div>
            <div className="truncate text-[11px] text-muted-foreground">
              bounds: {node.attrs.bounds || "<none>"}
            </div>
          </button>
        </div>
        {hasChildren && expanded ? (
          <ul>{node.children.map((child) => renderDumpTree(child, depth + 1))}</ul>
        ) : null}
      </li>
    )
  }

  const renderScreenshotPanel = (imgClassName: string) => (
    <div className="rounded-md border bg-muted/30 p-2">
      {screenshotBase64 !== "" ? (
        <div className="space-y-2">
          <div className="flex justify-center">
            <div className="relative inline-block overflow-hidden rounded">
            <img
              alt="设备截图"
              className={`${imgClassName} cursor-crosshair`}
              src={`data:image/png;base64,${screenshotBase64}`}
              onLoad={(event) =>
                setImageNaturalSize({
                  width: event.currentTarget.naturalWidth,
                  height: event.currentTarget.naturalHeight,
                })
              }
              onClick={(event) => {
                const rect = event.currentTarget.getBoundingClientRect()
                if (rect.width <= 0 || rect.height <= 0) {
                  return
                }
                const rawX = (event.clientX - rect.left) / rect.width
                const rawY = (event.clientY - rect.top) / rect.height
                const ratioX = Math.min(Math.max(rawX, 0), 1)
                const ratioY = Math.min(Math.max(rawY, 0), 1)
                const point: ClickPoint = {
                  percentX: ratioX,
                  percentY: ratioY,
                  absoluteX: ratioX * imageNaturalSize.width,
                  absoluteY: ratioY * imageNaturalSize.height,
                }
                setClickPoint(point)
                const message =
                  `百分比(0~1)=(${point.percentX.toFixed(6)}, ${point.percentY.toFixed(6)}), ` +
                  `绝对坐标(设备原始)=(${point.absoluteX.toFixed(1)}, ${point.absoluteY.toFixed(1)}), ` +
                  `坐标基准=截图原始像素(${imageNaturalSize.width.toFixed(1)}x${imageNaturalSize.height.toFixed(1)})`
                setClickLog(message)
                console.info("[trek-web] click-point", message)
              }}
            />
            {highlightRect !== null ? (
              <div
                className="pointer-events-none absolute z-10 border-2 border-red-500 bg-red-500/20"
                style={{
                    left: `${highlightRect.left}%`,
                    top: `${highlightRect.top}%`,
                    width: `${highlightRect.width}%`,
                    height: `${highlightRect.height}%`,
                }}
              />
            ) : null}
            {clickPoint !== null ? (
              <div
                className="pointer-events-none absolute z-20 h-3 w-3 -translate-x-1/2 -translate-y-1/2 rounded-full border-2 border-blue-600 bg-blue-300/80"
                style={{
                  left: `${clickPoint.percentX * 100}%`,
                  top: `${clickPoint.percentY * 100}%`,
                }}
              />
            ) : null}
          </div>
          </div>
          <p className="break-all font-mono text-[11px] text-muted-foreground">
            高亮日志: {highlightLog}
          </p>
          <div className="rounded-md border bg-background p-3">
            <div className="mb-2 flex items-center justify-between">
              <p className="text-sm font-medium">点击坐标</p>
              <div className="flex items-center gap-2">
                <Button
                  type="button"
                  size="sm"
                  variant="outline"
                  disabled={clickPoint === null}
                  onClick={() =>
                    void copyText(
                      clickPoint === null
                        ? ""
                        : JSON.stringify(
                            {
                              percent: {
                                x: Number(clickPoint.percentX.toFixed(6)),
                                y: Number(clickPoint.percentY.toFixed(6)),
                              },
                              absolute: {
                                x: Number(clickPoint.absoluteX.toFixed(1)),
                                y: Number(clickPoint.absoluteY.toFixed(1)),
                              },
                            },
                            null,
                            2
                          )
                    )
                  }
                >
                  复制全部
                </Button>
                {copyStatus !== "" ? (
                  <span className="text-xs text-muted-foreground">{copyStatus}</span>
                ) : null}
              </div>
            </div>
            <div className="grid grid-cols-1 gap-2 text-xs md:grid-cols-[1fr_auto] md:items-center">
              <p className="break-all font-mono">
                百分比(0~1):{" "}
                {clickPoint === null
                  ? "-"
                  : `x=${clickPoint.percentX.toFixed(6)}, y=${clickPoint.percentY.toFixed(6)}`}
              </p>
              <Button
                type="button"
                size="sm"
                variant="outline"
                disabled={clickPoint === null}
                onClick={() =>
                  void copyText(
                    clickPoint === null
                      ? ""
                      : `${clickPoint.percentX.toFixed(6)},${clickPoint.percentY.toFixed(6)}`
                  )
                }
              >
                复制百分比
              </Button>
              <p className="break-all font-mono">
                绝对坐标(设备原始):{" "}
                {clickPoint === null
                  ? "-"
                  : `x=${clickPoint.absoluteX.toFixed(1)}, y=${clickPoint.absoluteY.toFixed(1)}`}
              </p>
              <Button
                type="button"
                size="sm"
                variant="outline"
                disabled={clickPoint === null}
                onClick={() =>
                  void copyText(
                    clickPoint === null
                      ? ""
                      : `${clickPoint.absoluteX.toFixed(1)},${clickPoint.absoluteY.toFixed(1)}`
                  )
                }
              >
                复制绝对坐标
              </Button>
            </div>
            <p className="mt-2 break-all font-mono text-[11px] text-muted-foreground">
              调试日志: {clickLog}
            </p>
            <p className="mt-1 break-all font-mono text-[11px] text-muted-foreground">
              当前坐标基准: 截图原始像素
              ({absoluteSpace.width.toFixed(1)}x{absoluteSpace.height.toFixed(1)})
            </p>
          </div>
        </div>
      ) : (
        <p className="text-sm text-muted-foreground">暂无截图，请先刷新预览。</p>
      )}
    </div>
  )

  const renderDumpPanel = (heightClassName: string) => (
    <div className={`${heightClassName} min-h-0 w-full overflow-auto rounded-md border bg-background p-3`}>
      {parsedDump.root !== null ? (
        <ul>{renderDumpTree(parsedDump.root, 0)}</ul>
      ) : (
        <p className="font-mono text-xs text-muted-foreground">暂无 XML，请先刷新预览。</p>
      )}
    </div>
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
          {renderScreenshotPanel("max-h-[520px] max-w-full rounded object-contain")}
        </div>

        <div className="rounded-xl border bg-card p-4">
          <h2 className="mb-3 text-base font-semibold">Dump</h2>
          {renderDumpPanel("min-h-[640px]")}
        </div>

        <div className="rounded-xl border bg-card p-4">
          <div className="mb-3 flex items-center justify-between">
            <h2 className="text-base font-semibold">配置</h2>
            <Button
              variant="outline"
              onClick={handleRefreshPreview}
              disabled={loading}
            >
              打印界面
            </Button>
          </div>
          <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
            <div className="flex flex-col gap-1 text-sm md:col-span-2">
              <label>设备列表</label>
              <div className="flex items-center gap-2">
                <select
                  className="min-w-0 flex-1 rounded-md border bg-background px-3 py-2"
                  value={deviceSerial}
                  onChange={(e) => setDeviceSerial(e.target.value)}
                >
                  <option value="">不指定（由系统自动选择）</option>
                  {deviceOptions.map((item) => (
                    <option key={item.serial} value={item.serial}>
                      {item.label}
                    </option>
                  ))}
                </select>
                <Button
                  type="button"
                  variant="outline"
                  onClick={() => void fetchDevices()}
                  disabled={loadingDevices}
                >
                  {loadingDevices ? "刷新中" : "刷新"}
                </Button>
              </div>
            </div>
            <p className="text-sm text-muted-foreground md:col-span-2">
              当前预览设备序列号：{usedSerial !== "" ? usedSerial : "尚未确定（先点刷新预览）"}
            </p>

            <label className="flex flex-col gap-1 text-sm">
              页面源 page_source
              <select
                className="rounded-md border bg-background px-3 py-2"
                value={pageSource}
                onChange={(e) => setPageSource(e.target.value as "uia" | "poco")}
              >
                <option value="uia">uia</option>
                <option value="poco">poco</option>
              </select>
            </label>

            <label className="flex flex-col gap-1 text-sm">
              触控模式 touch_mode
              <select
                className="rounded-md border bg-background px-3 py-2"
                value={touchMode}
                onChange={(e) =>
                  setTouchMode(e.target.value as "motion" | "uia" | "adb")
                }
              >
                <option value="motion">motion</option>
                <option value="uia">uia</option>
                <option value="adb">adb</option>
              </select>
            </label>

            <label className="flex flex-col gap-1 text-sm">
              UIA 端口 uia.server_port
              <input
                className="rounded-md border bg-background px-3 py-2"
                type="number"
                min={0}
                value={uiaPort}
                onChange={(e) => setUiaPort(e.target.value)}
                placeholder="默认空"
              />
            </label>

            <label className="flex flex-col gap-1 text-sm">
              日志文件级别 log.file_level
              <select
                className="rounded-md border bg-background px-3 py-2"
                value={fileLevel}
                onChange={(e) => setFileLevel(e.target.value)}
              >
                <option value="">空（不输出）</option>
                <option value="debug">debug</option>
                <option value="info">info</option>
                <option value="warn">warn</option>
                <option value="error">error</option>
              </select>
            </label>

            {pageSource === "poco" ? (
              <>
                <label className="flex flex-col gap-1 text-sm">
                  Poco 引擎 poco.engine
                  <select
                    className="rounded-md border bg-background px-3 py-2"
                    value={pocoEngine}
                    onChange={(e) => setPocoEngine(e.target.value)}
                  >
                    <option value="UNITY_3D">UNITY_3D</option>
                    <option value="UE4">UE4</option>
                    <option value="COCOS2DX_JS">COCOS2DX_JS</option>
                    <option value="COCOS_CREATOR">COCOS_CREATOR</option>
                    <option value="EGRET">EGRET</option>
                    <option value="COCOS2DX_LUA">COCOS2DX_LUA</option>
                    <option value="COCOS2DX_CPLUS">COCOS2DX_CPLUS</option>
                  </select>
                </label>
                <label className="flex flex-col gap-1 text-sm">
                  Poco 端口 poco.port
                  <input
                    className="rounded-md border bg-background px-3 py-2"
                    type="number"
                    min={0}
                    value={pocoPort}
                    onChange={(e) => setPocoPort(e.target.value)}
                    placeholder="默认空（走引擎默认端口）"
                  />
                </label>
              </>
            ) : null}

            <label className="flex items-center gap-2 text-sm md:col-span-2">
              <input
                type="checkbox"
                checked={skipAll}
                onChange={(e) => setSkipAll(e.target.checked)}
              />
              跳过模型动作 skip_all_actions_from_model
            </label>

            <label className="flex flex-col gap-1 text-sm md:col-span-2">
              输出路径
              <input
                className="rounded-md border bg-background px-3 py-2"
                value={outputPath}
                onChange={(e) => setOutputPath(e.target.value)}
              />
            </label>

            <div className="mt-4 flex flex-wrap gap-2 md:col-span-2">
              <Button onClick={handlePreview} disabled={loading}>
                预览配置
              </Button>
              <Button variant="outline" onClick={handleSave} disabled={loading}>
                保存到文件
              </Button>
            </div>

            {status !== "" ? (
              <p className="text-sm text-emerald-700 md:col-span-2">{status}</p>
            ) : null}
            {error !== "" ? (
              <p className="text-sm text-red-700 md:col-span-2">{error}</p>
            ) : null}

            <div className="md:col-span-2">
              <label className="text-sm font-medium">生成结果</label>
              <textarea
                className="mt-2 min-h-72 w-full rounded-md border bg-background p-3 font-mono text-sm"
                readOnly
                value={resultText}
              />
            </div>
          </div>
        </div>
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
              {renderScreenshotPanel("max-h-[680px] max-w-full rounded object-contain")}
            </div>
          </ResizablePanel>
          <ResizableHandle withHandle className="mx-2 rounded-full" />
          <ResizablePanel defaultSize={33} minSize={20}>
            <div className="flex h-full flex-col rounded-xl border bg-card p-4">
              <h2 className="mb-3 text-base font-semibold">Dump</h2>
              {renderDumpPanel("h-full flex-1")}
            </div>
          </ResizablePanel>
          <ResizableHandle withHandle className="mx-2 rounded-full" />
          <ResizablePanel defaultSize={33} minSize={22}>
            <div className="h-full overflow-y-auto rounded-xl border bg-card p-4">
              <div className="mb-3 flex items-center justify-between">
                <h2 className="text-base font-semibold">配置</h2>
                <Button
                  variant="outline"
                  onClick={handleRefreshPreview}
                  disabled={loading}
                >
                  当前界面
                </Button>
              </div>
              <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
                <div className="flex flex-col gap-1 text-sm md:col-span-2">
                  <label>设备列表</label>
                  <div className="flex items-center gap-2">
                    <select
                      className="min-w-0 flex-1 rounded-md border bg-background px-3 py-2"
                      value={deviceSerial}
                      onChange={(e) => setDeviceSerial(e.target.value)}
                    >
                      <option value="">不指定（由系统自动选择）</option>
                      {deviceOptions.map((item) => (
                        <option key={item.serial} value={item.serial}>
                          {item.label}
                        </option>
                      ))}
                    </select>
                    <Button
                      type="button"
                      variant="outline"
                      onClick={() => void fetchDevices()}
                      disabled={loadingDevices}
                    >
                      {loadingDevices ? "刷新中" : "刷新"}
                    </Button>
                  </div>
                </div>
                <p className="text-sm text-muted-foreground md:col-span-2">
                  当前预览设备序列号：
                  {usedSerial !== "" ? usedSerial : "尚未确定（先点当前界面）"}
                </p>

                <label className="flex flex-col gap-1 text-sm">
                  页面源 page_source
                  <select
                    className="rounded-md border bg-background px-3 py-2"
                    value={pageSource}
                    onChange={(e) =>
                      setPageSource(e.target.value as "uia" | "poco")
                    }
                  >
                    <option value="uia">uia</option>
                    <option value="poco">poco</option>
                  </select>
                </label>

                <label className="flex flex-col gap-1 text-sm">
                  触控模式 touch_mode
                  <select
                    className="rounded-md border bg-background px-3 py-2"
                    value={touchMode}
                    onChange={(e) =>
                      setTouchMode(e.target.value as "motion" | "uia" | "adb")
                    }
                  >
                    <option value="motion">motion</option>
                    <option value="uia">uia</option>
                    <option value="adb">adb</option>
                  </select>
                </label>

                <label className="flex flex-col gap-1 text-sm">
                  UIA 端口 uia.server_port
                  <input
                    className="rounded-md border bg-background px-3 py-2"
                    type="number"
                    min={0}
                    value={uiaPort}
                    onChange={(e) => setUiaPort(e.target.value)}
                    placeholder="默认空"
                  />
                </label>

                <label className="flex flex-col gap-1 text-sm">
                  日志文件级别 log.file_level
                  <select
                    className="rounded-md border bg-background px-3 py-2"
                    value={fileLevel}
                    onChange={(e) => setFileLevel(e.target.value)}
                  >
                    <option value="">空（不输出）</option>
                    <option value="debug">debug</option>
                    <option value="info">info</option>
                    <option value="warn">warn</option>
                    <option value="error">error</option>
                  </select>
                </label>

                {pageSource === "poco" ? (
                  <>
                    <label className="flex flex-col gap-1 text-sm">
                      Poco 引擎 poco.engine
                      <select
                        className="rounded-md border bg-background px-3 py-2"
                        value={pocoEngine}
                        onChange={(e) => setPocoEngine(e.target.value)}
                      >
                        <option value="UNITY_3D">UNITY_3D</option>
                        <option value="UE4">UE4</option>
                        <option value="COCOS2DX_JS">COCOS2DX_JS</option>
                        <option value="COCOS_CREATOR">COCOS_CREATOR</option>
                        <option value="EGRET">EGRET</option>
                        <option value="COCOS2DX_LUA">COCOS2DX_LUA</option>
                        <option value="COCOS2DX_CPLUS">COCOS2DX_CPLUS</option>
                      </select>
                    </label>
                    <label className="flex flex-col gap-1 text-sm">
                      Poco 端口 poco.port
                      <input
                        className="rounded-md border bg-background px-3 py-2"
                        type="number"
                        min={0}
                        value={pocoPort}
                        onChange={(e) => setPocoPort(e.target.value)}
                        placeholder="默认空（走引擎默认端口）"
                      />
                    </label>
                  </>
                ) : null}

                <label className="flex items-center gap-2 text-sm md:col-span-2">
                  <input
                    type="checkbox"
                    checked={skipAll}
                    onChange={(e) => setSkipAll(e.target.checked)}
                  />
                  跳过模型动作 skip_all_actions_from_model
                </label>

                <label className="flex flex-col gap-1 text-sm md:col-span-2">
                  输出路径
                  <input
                    className="rounded-md border bg-background px-3 py-2"
                    value={outputPath}
                    onChange={(e) => setOutputPath(e.target.value)}
                  />
                </label>

                <div className="mt-4 flex flex-wrap gap-2 md:col-span-2">
                  <Button onClick={handlePreview} disabled={loading}>
                    预览配置
                  </Button>
                  <Button variant="outline" onClick={handleSave} disabled={loading}>
                    保存到文件
                  </Button>
                </div>

                {status !== "" ? (
                  <p className="text-sm text-emerald-700 md:col-span-2">{status}</p>
                ) : null}
                {error !== "" ? (
                  <p className="text-sm text-red-700 md:col-span-2">{error}</p>
                ) : null}

                <div className="md:col-span-2">
                  <label className="text-sm font-medium">生成结果</label>
                  <textarea
                    className="mt-2 min-h-72 w-full rounded-md border bg-background p-3 font-mono text-sm"
                    readOnly
                    value={resultText}
                  />
                </div>
              </div>
            </div>
          </ResizablePanel>
        </ResizablePanelGroup>
      </section>
    </div>
  )
}

export default App
