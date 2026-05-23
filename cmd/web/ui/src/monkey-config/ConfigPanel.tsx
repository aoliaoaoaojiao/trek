import { useEffect, useState } from "react"
import { CircleHelpIcon } from "lucide-react"

import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip"

import type { DeviceOption, PageNameStrategy } from "./types"

type ConfigTab = "base" | "page" | "recovery" | "uct"
type SelectOption = {
  value: string
  label: string
}

const emptySelectValue = "__empty__"

type Props = {
  configTab: ConfigTab
  setConfigTab: (tab: ConfigTab) => void
  loading: boolean
  onImportConfig: (source: string) => void
  onRefreshCurrentPage: () => void
  deviceSerial: string
  setDeviceSerial: (value: string) => void
  deviceOptions: DeviceOption[]
  loadingDevices: boolean
  onRefreshDevices: () => void
  usedSerial: string
  currentPackageName: string
  pageSource: "uia" | "poco" | "screenshot"
  setPageSource: (value: "uia" | "poco" | "screenshot") => void
  pageNameStrategy: PageNameStrategy
  setPageNameStrategy: (value: PageNameStrategy) => void
  touchMode: "motion" | "uia" | "adb"
  setTouchMode: (value: "motion" | "uia" | "adb") => void
  pageControlStrategy: "" | "raw" | "ocr" | "llm"
  setPageControlStrategy: (value: "" | "raw" | "ocr" | "llm") => void
  algorithm: "" | "reuse" | "uctbandit" | "random"
  setAlgorithm: (value: "" | "reuse" | "uctbandit" | "random") => void
  captureScreenshotMode: "" | "true" | "false"
  setCaptureScreenshotMode: (value: "" | "true" | "false") => void
  keepStepRecordsMode: "" | "true" | "false"
  setKeepStepRecordsMode: (value: "" | "true" | "false") => void
  uiaPort: string
  setUiaPort: (value: string) => void
  fileLevel: string
  setFileLevel: (value: string) => void
  pocoEngine: string
  setPocoEngine: (value: string) => void
  pocoPort: string
  setPocoPort: (value: string) => void
  scrollInferThreshold: string
  setScrollInferThreshold: (value: string) => void
  imageSimilarityThreshold: string
  setImageSimilarityThreshold: (value: string) => void
  imageFingerprintHammingThreshold: string
  setImageFingerprintHammingThreshold: (value: string) => void
  pageControlCacheTTLSeconds: string
  setPageControlCacheTTLSeconds: (value: string) => void
  exploreOCRTimeoutMs: string
  setExploreOCRTimeoutMs: (value: string) => void
  llmTimeoutMs: string
  setLLMTimeoutMs: (value: string) => void
  recoveryCooldownSteps: string
  setRecoveryCooldownSteps: (value: string) => void
  recoveryTwoStateLoopThreshold: string
  setRecoveryTwoStateLoopThreshold: (value: string) => void
  recoveryHighVisitThreshold: string
  setRecoveryHighVisitThreshold: (value: string) => void
  recoveryLowRewardWindow: string
  setRecoveryLowRewardWindow: (value: string) => void
  candidateAmbiguityTopGapThreshold: string
  setCandidateAmbiguityTopGapThreshold: (value: string) => void
  highValuePageVisitLimit: string
  setHighValuePageVisitLimit: (value: string) => void
  candidateRiskDropThreshold: string
  setCandidateRiskDropThreshold: (value: string) => void
  candidateMinFusionScore: string
  setCandidateMinFusionScore: (value: string) => void
  uctTwoStateLoopPenalty: string
  setUctTwoStateLoopPenalty: (value: string) => void
  uctEdgeRepeatPenalty: string
  setUctEdgeRepeatPenalty: (value: string) => void
  uctEdgeRepeatThreshold: string
  setUctEdgeRepeatThreshold: (value: string) => void
  uctActionCooldownPenalty: string
  setUctActionCooldownPenalty: (value: string) => void
  uctRecentActionWindow: string
  setUctRecentActionWindow: (value: string) => void
  uctLoopEscapeExploreBoost: string
  setUctLoopEscapeExploreBoost: (value: string) => void
  reuseEpsilon: string
  setReuseEpsilon: (value: string) => void
  reuseGamma: string
  setReuseGamma: (value: string) => void
  reuseNStep: string
  setReuseNStep: (value: string) => void
  reuseModelSavePath: string
  setReuseModelSavePath: (value: string) => void
  reuseEnableModelPersistenceMode: "" | "true" | "false"
  setReuseEnableModelPersistenceMode: (value: "" | "true" | "false") => void
  reuseResetModelOnStartMode: "" | "true" | "false"
  setReuseResetModelOnStartMode: (value: "" | "true" | "false") => void
  skipAll: boolean
  setSkipAll: (value: boolean) => void
  rangeLeftInput: string
  setRangeLeftInput: (value: string) => void
  rangeTopInput: string
  setRangeTopInput: (value: string) => void
  rangeRightInput: string
  setRangeRightInput: (value: string) => void
  rangeBottomInput: string
  setRangeBottomInput: (value: string) => void
  rangeLog: string
  onResetRange: () => void
  onCopyConfig: () => void
  onDownloadConfig: () => void
  savePreviewOpen: boolean
  setSavePreviewOpen: (value: boolean) => void
  status: string
  error: string
  resultText: string
}

export function ConfigPanel(props: Props) {
  const rangeMatchText = `serial=${props.usedSerial || props.deviceSerial || "<empty>"}，package=${props.currentPackageName || "<empty>"}`
  const [importOpen, setImportOpen] = useState(false)
  const [importText, setImportText] = useState("")
  const [importStatus, setImportStatus] = useState("")

  useEffect(() => {
    const openImportDialog = () => {
      setImportOpen(true)
    }
    document.addEventListener("trek-open-import-config", openImportDialog)
    return () => {
      document.removeEventListener("trek-open-import-config", openImportDialog)
    }
  }, [])

  const renderSelect = (
    value: string,
    onValueChange: (value: string) => void,
    placeholder: string,
    options: SelectOption[],
    disabled = false
  ) => (
    <Select value={value === "" ? emptySelectValue : value} onValueChange={(next) => onValueChange(next === emptySelectValue ? "" : next)} disabled={disabled}>
      <SelectTrigger className="w-full bg-background px-3 py-2 disabled:cursor-not-allowed disabled:opacity-60">
        <SelectValue placeholder={placeholder} />
      </SelectTrigger>
      <SelectContent>
        {options.map((option) => (
          <SelectItem key={option.value} value={option.value === "" ? emptySelectValue : option.value}>
            {option.label}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  )

  const handleImportFile = (file: File | undefined) => {
    if (file === undefined) {
      return
    }
    const reader = new FileReader()
    reader.onload = () => {
      setImportText(String(reader.result ?? ""))
      setImportStatus(`已读取文件：${file.name}`)
    }
    reader.onerror = () => {
      setImportStatus("读取配置文件失败")
    }
    reader.readAsText(file, "UTF-8")
  }

  const handleApplyImport = () => {
    try {
      props.onImportConfig(importText)
      setImportStatus("配置已加载到表单")
      setImportOpen(false)
    } catch (err) {
      setImportStatus(err instanceof Error ? err.message : "导入配置失败")
    }
  }

  const renderFieldTitle = (title: string, tip?: string) => (
    <div className="flex items-center gap-1.5">
      <span>{title}</span>
      {tip ? (
        <Tooltip>
          <TooltipTrigger asChild>
            <button
              type="button"
              className="inline-flex h-4 w-4 items-center justify-center rounded text-muted-foreground transition hover:text-foreground"
              aria-label={`${title} 说明`}
            >
              <CircleHelpIcon className="h-3.5 w-3.5" />
            </button>
          </TooltipTrigger>
          <TooltipContent side="top" align="start" sideOffset={6} className="max-w-sm whitespace-pre-wrap leading-5">
            {tip}
          </TooltipContent>
        </Tooltip>
      ) : null}
    </div>
  )

  return (
    <TooltipProvider delayDuration={120}>
      <div className="mb-3 flex items-center justify-between">
        <h2 className="text-base font-semibold">配置</h2>
      </div>
      {importOpen ? (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/35 p-4">
          <div className="w-full max-w-2xl rounded-xl border bg-background p-4 shadow-lg">
            <div className="mb-3 flex items-center justify-between">
              <h3 className="text-base font-semibold">导入 JS 配置</h3>
              <Button type="button" variant="outline" size="sm" onClick={() => setImportOpen(false)}>
                关闭
              </Button>
            </div>
            <div className="space-y-3">
              <label className="flex flex-col gap-1 text-sm">
                选择配置文件
                <input
                  className="rounded-md border bg-background px-3 py-2"
                  type="file"
                  accept=".js,application/javascript,text/javascript,text/plain"
                  onChange={(event) => handleImportFile(event.target.files?.[0])}
                />
              </label>
              <label className="flex flex-col gap-1 text-sm">
                配置内容
                <textarea
                  className="min-h-72 rounded-md border bg-background p-3 font-mono text-xs"
                  value={importText}
                  onChange={(event) => setImportText(event.target.value)}
                  placeholder="粘贴或上传 const config = { ... }"
                />
              </label>
              {importStatus !== "" ? <p className="text-sm text-muted-foreground">{importStatus}</p> : null}
              <div className="flex justify-end gap-2">
                <Button type="button" variant="outline" onClick={() => setImportOpen(false)}>
                  取消
                </Button>
                <Button type="button" onClick={handleApplyImport} disabled={importText.trim() === ""}>
                  加载配置
                </Button>
              </div>
            </div>
          </div>
        </div>
      ) : null}
      <Dialog open={props.savePreviewOpen} onOpenChange={props.setSavePreviewOpen}>
        <DialogContent className="max-w-4xl p-0 sm:max-w-4xl">
          <DialogHeader className="px-6 pt-6">
            <DialogTitle>配置导出</DialogTitle>
            <DialogDescription>
              先确认生成结果，再选择复制配置或下载配置。
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4 px-6 pb-4">
            {props.status !== "" ? <p className="text-sm text-emerald-700">{props.status}</p> : null}
            {props.error !== "" ? <p className="text-sm text-red-700">{props.error}</p> : null}
            <div>
              <label className="text-sm font-medium">生成结果</label>
              <textarea className="mt-2 min-h-[420px] w-full rounded-md border bg-background p-3 font-mono text-sm" readOnly value={props.resultText} />
            </div>
          </div>
          <DialogFooter className="mx-0 mb-0 rounded-b-xl px-6 py-4">
            <Button variant="outline" onClick={props.onCopyConfig} disabled={props.resultText.trim() === ""}>
              复制配置
            </Button>
            <Button onClick={props.onDownloadConfig} disabled={props.resultText.trim() === ""}>
              下载配置
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
      <div className="mb-3 flex gap-2">
        <Button type="button" variant={props.configTab === "base" ? "default" : "outline"} onClick={() => props.setConfigTab("base")}>
          基础运行
        </Button>
        <Button type="button" variant={props.configTab === "page" ? "default" : "outline"} onClick={() => props.setConfigTab("page")}>
          页面识别
        </Button>
        <Button type="button" variant={props.configTab === "recovery" ? "default" : "outline"} onClick={() => props.setConfigTab("recovery")}>
          恢复策略
        </Button>
        <Button type="button" variant={props.configTab === "uct" ? "default" : "outline"} onClick={() => props.setConfigTab("uct")}>
          决策算法
        </Button>
      </div>
      {props.configTab === "base" ? (
        <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
          <div className="flex flex-col gap-1 text-sm md:col-span-2">
            <label>设备列表</label>
            <div className="flex items-center gap-2">
              <div className="min-w-0 flex-1">
                {renderSelect(
                  props.deviceSerial,
                  props.setDeviceSerial,
                  "选择设备",
                  [
                    { value: "", label: "不指定（由系统自动选择）" },
                    ...props.deviceOptions.map((item) => ({
                      value: item.serial,
                      label: item.label,
                    })),
                  ]
                )}
              </div>
              <Button type="button" variant="outline" onClick={props.onRefreshDevices} disabled={props.loadingDevices}>
                {props.loadingDevices ? "刷新中" : "刷新"}
              </Button>
            </div>
          </div>
          <p className="text-sm text-muted-foreground md:col-span-2">
            当前预览设备序列号：{props.usedSerial !== "" ? props.usedSerial : "尚未确定（先点当前界面）"}
          </p>
          <label className="flex flex-col gap-1 text-sm">
            包名（当前界面）
            <input className="rounded-md border bg-background px-3 py-2 font-mono" value={props.currentPackageName} readOnly placeholder="点击当前界面后自动填充" />
          </label>
          <label className="flex flex-col gap-1 text-sm">
            {renderFieldTitle("页面源", "选择 screenshot 时，运行过程会默认每一步都截图，并优先适合配合 image_fingerprint 使用。")}
            {renderSelect(props.pageSource, (value) => props.setPageSource(value as "uia" | "poco" | "screenshot"), "选择页面源", [
              { value: "uia", label: "uia" },
              { value: "poco", label: "poco" },
              { value: "screenshot", label: "screenshot" },
            ])}
          </label>
          <label className="flex flex-col gap-1 text-sm">
            触控模式
            {renderSelect(props.touchMode, (value) => props.setTouchMode(value as "motion" | "uia" | "adb"), "选择触控模式", [
              { value: "motion", label: "motion" },
              { value: "uia", label: "uia" },
              { value: "adb", label: "adb" },
            ])}
          </label>
          <label className="flex flex-col gap-1 text-sm">
            UIA 端口
            <input className="rounded-md border bg-background px-3 py-2" type="number" min={0} value={props.uiaPort} onChange={(e) => props.setUiaPort(e.target.value)} placeholder="留空=使用驱动默认端口" />
          </label>
          <label className="flex flex-col gap-1 text-sm">
            日志文件级别
            {renderSelect(props.fileLevel, props.setFileLevel, "选择日志级别", [
              { value: "", label: "关闭文件日志输出" },
              { value: "debug", label: "debug" },
              { value: "info", label: "info" },
              { value: "warn", label: "warn" },
              { value: "error", label: "error" },
            ])}
          </label>
          <label className="flex flex-col gap-1 text-sm">
            {renderFieldTitle(
              "截图采集",
              props.pageSource === "screenshot"
                ? "页面源为 screenshot 时固定开启，默认每一步都会截图。"
                : "默认自动；当页面理解策略不是 raw 时也会自动开启截图采集。"
            )}
            {renderSelect(
              props.pageSource === "screenshot" ? "true" : props.captureScreenshotMode,
              (value) => props.setCaptureScreenshotMode(value as "" | "true" | "false"),
              "选择截图采集",
              [
                { value: "", label: "自动（按内置规则决定）" },
                { value: "true", label: "开启" },
                { value: "false", label: "关闭" },
              ],
              props.pageSource === "screenshot"
            )}
          </label>
          <label className="flex flex-col gap-1 text-sm">
            步骤记录
            {renderSelect(props.keepStepRecordsMode, (value) => props.setKeepStepRecordsMode(value as "" | "true" | "false"), "选择步骤记录", [
              { value: "", label: "自动（使用内置默认行为）" },
              { value: "true", label: "保留" },
              { value: "false", label: "关闭" },
            ])}
          </label>
          {props.pageSource === "poco" ? (
            <>
              <label className="flex flex-col gap-1 text-sm">
                Poco 引擎
                {renderSelect(props.pocoEngine, props.setPocoEngine, "选择 Poco 引擎", [
                  { value: "UNITY_3D", label: "UNITY_3D" },
                  { value: "UE4", label: "UE4" },
                  { value: "COCOS2DX_JS", label: "COCOS2DX_JS" },
                  { value: "COCOS_CREATOR", label: "COCOS_CREATOR" },
                  { value: "EGRET", label: "EGRET" },
                  { value: "COCOS2DX_LUA", label: "COCOS2DX_LUA" },
                  { value: "COCOS2DX_CPLUS", label: "COCOS2DX_CPLUS" },
                ])}
              </label>
              <label className="flex flex-col gap-1 text-sm">
                Poco 端口
                <input className="rounded-md border bg-background px-3 py-2" type="number" min={0} value={props.pocoPort} onChange={(e) => props.setPocoPort(e.target.value)} placeholder="留空=按引擎默认端口（如 UNITY_3D 为 5001）" />
              </label>
            </>
          ) : null}
          <label className="flex items-center gap-2 text-sm md:col-span-2">
            <input type="checkbox" checked={props.skipAll} onChange={(e) => props.setSkipAll(e.target.checked)} />
            跳过模型动作
          </label>
        </div>
      ) : null}
      {props.configTab === "page" ? (
          <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
            <label className="flex flex-col gap-1 text-sm">
              {renderFieldTitle(
                "页面理解策略",
                props.pageSource === "screenshot"
                  ? "截图页面源不支持 raw，默认使用 ocr，也可以改成 llm；ocr/llm 会按图片指纹缓存控件树，相同图片优先复用缓存，连续命中过多或进入阻塞恢复时会自动重新识别。"
                  : "控制页面控件信息来自原始节点、OCR 还是 LLM 理解；ocr/llm 会按图片指纹缓存控件树，相同图片优先复用缓存，连续命中过多或进入阻塞恢复时会自动重新识别。"
              )}
              {renderSelect(props.pageControlStrategy, (value) => props.setPageControlStrategy(value as "" | "raw" | "ocr" | "llm"), "选择页面理解策略", [
                ...(props.pageSource === "screenshot"
                  ? []
                  : [
                      { value: "", label: "自动（普通页面源默认 raw）" },
                      { value: "raw", label: "raw" },
                    ]),
                { value: "ocr", label: "ocr" },
                { value: "llm", label: "llm" },
              ])}
            </label>
          <label className="flex flex-col gap-1 text-sm">
            {renderFieldTitle("页面名策略", "默认：普通页面源使用 structure_fingerprint，截图页面源更适合 image_fingerprint。")}
            {renderSelect(props.pageNameStrategy, (value) => props.setPageNameStrategy(value as PageNameStrategy), "选择页面名策略", [
              { value: "", label: "自动（普通源默认 structure_fingerprint）" },
              { value: "structure_fingerprint", label: "structure_fingerprint（结构指纹）" },
              { value: "activity_only", label: "activity_only（仅 Activity）" },
              { value: "image_fingerprint", label: "image_fingerprint（图片指纹）" },
            ])}
          </label>
          <label className="flex flex-col gap-1 text-sm">
            {renderFieldTitle(
              "滚动识别阈值",
              props.pageControlStrategy === "llm"
                ? "默认：5。LLM 模式下已禁用此推断，修改该值不会生效，避免对合成控件树误判滚动容器。"
                : "默认：5。值越小越容易生成滚动动作，值越大越保守，0 表示关闭；raw 模式最有效，ocr 模式作用较弱。"
            )}
            <input
              className="rounded-md border bg-background px-3 py-2 disabled:cursor-not-allowed disabled:opacity-60"
              type="number"
              step="1"
              value={props.scrollInferThreshold}
              onChange={(e) => props.setScrollInferThreshold(e.target.value)}
              placeholder={props.pageControlStrategy === "llm" ? "LLM 模式下固定禁用" : "留空=使用默认值 5"}
              disabled={props.pageControlStrategy === "llm"}
            />
          </label>
          <label className="flex flex-col gap-1 text-sm">
            {renderFieldTitle("图片相似度 SSIM 阈值", "默认：0.995，取值范围 0~1，越接近 1 越严格。")}
            <input className="rounded-md border bg-background px-3 py-2" type="number" min={0} max={1} step="0.001" value={props.imageSimilarityThreshold} onChange={(e) => props.setImageSimilarityThreshold(e.target.value)} placeholder="留空=使用默认值 0.995" />
          </label>
          <label className="flex flex-col gap-1 text-sm">
            {renderFieldTitle("图片指纹模糊匹配阈值", "默认：10。Hamming 距离 ≤ 阈值的截图会被合并为同一页面名。10 表示 512 bit 中允许约 2% 差异，可过滤状态栏时间/电量变化；设为 0 可关闭模糊匹配。")}
            <input className="rounded-md border bg-background px-3 py-2" type="number" min={0} step="1" value={props.imageFingerprintHammingThreshold} onChange={(e) => props.setImageFingerprintHammingThreshold(e.target.value)} placeholder="留空=使用默认值 10" />
          </label>
          <label className="flex flex-col gap-1 text-sm">
            {renderFieldTitle("页面缓存 TTL(秒)", "默认：1800（30分钟）。OCR/LLM 页面理解结果的缓存有效期，过期后会重新识别。设为 0 表示永不过期。")}
            <input className="rounded-md border bg-background px-3 py-2" type="number" min={0} step="1" value={props.pageControlCacheTTLSeconds} onChange={(e) => props.setPageControlCacheTTLSeconds(e.target.value)} placeholder="留空=使用默认值 1800" />
          </label>
          <label className="flex flex-col gap-1 text-sm">
            {renderFieldTitle("OCR 超时(ms)", "默认：10000，统一控制 OCR 请求超时：页面理解（page_control_strategy=ocr）、goja 脚本 trek.ocr.recognize()。")}
            <input className="rounded-md border bg-background px-3 py-2" type="number" min={0} step="1" value={props.exploreOCRTimeoutMs} onChange={(e) => props.setExploreOCRTimeoutMs(e.target.value)} placeholder="留空=使用默认值 10000" />
          </label>
          <label className="flex flex-col gap-1 text-sm">
            {renderFieldTitle("LLM 超时(ms)", "默认：15000，统一控制所有 LLM 请求超时：页面理解（page_control_strategy=llm）、goja 脚本 trek.llm.chat()。含一次自动重试，实际最大等待约为 2×超时+2s。")}
            <input className="rounded-md border bg-background px-3 py-2" type="number" min={0} step="1" value={props.llmTimeoutMs} onChange={(e) => props.setLLMTimeoutMs(e.target.value)} placeholder="留空=使用默认值 15000" />
          </label>
          <div className="md:col-span-2 rounded-md border bg-background p-3">
            <p className="mb-2 text-sm font-medium">有效触控区域</p>
            <p className="mb-3 text-xs text-muted-foreground">默认整图：left=0，top=0，right=1，bottom=1</p>
            <div className="mb-3 grid grid-cols-1 gap-2 rounded-md border p-2 text-xs md:grid-cols-2">
              <label className="flex flex-col gap-1">
                左边界
                <input className="rounded border bg-background px-2 py-1 font-mono" type="number" min={0} max={1} step="0.001" value={props.rangeLeftInput} onChange={(event) => props.setRangeLeftInput(event.target.value)} />
              </label>
              <label className="flex flex-col gap-1">
                上边界
                <input className="rounded border bg-background px-2 py-1 font-mono" type="number" min={0} max={1} step="0.001" value={props.rangeTopInput} onChange={(event) => props.setRangeTopInput(event.target.value)} />
              </label>
              <label className="flex flex-col gap-1">
                右边界
                <input className="rounded border bg-background px-2 py-1 font-mono" type="number" min={0} max={1} step="0.001" value={props.rangeRightInput} onChange={(event) => props.setRangeRightInput(event.target.value)} />
              </label>
              <label className="flex flex-col gap-1">
                下边界
                <input className="rounded border bg-background px-2 py-1 font-mono" type="number" min={0} max={1} step="0.001" value={props.rangeBottomInput} onChange={(event) => props.setRangeBottomInput(event.target.value)} />
              </label>
              <div className="flex flex-wrap gap-2 md:col-span-2">
                <Button type="button" size="sm" variant="outline" onClick={props.onResetRange}>
                  恢复默认
                </Button>
              </div>
              <p className="break-all font-mono text-[11px] text-muted-foreground md:col-span-2">
                范围匹配: {rangeMatchText}，当前映射: 横向 = 左边界 + (右边界 - 左边界) * x，纵向 = 上边界 + (下边界 - 上边界) * y
              </p>
              <p className="break-all font-mono text-[11px] text-muted-foreground md:col-span-2">范围状态: {props.rangeLog}</p>
            </div>
          </div>
        </div>
      ) : null}
      {props.configTab === "recovery" ? (
        <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
          <label className="flex flex-col gap-1 text-sm">
            {renderFieldTitle("恢复冷却步数", "默认：2。恢复成功后在接下来 N 步内暂停再次触发恢复。")}
            <input className="rounded-md border bg-background px-3 py-2" type="number" min={0} step="1" value={props.recoveryCooldownSteps} onChange={(e) => props.setRecoveryCooldownSteps(e.target.value)} placeholder="留空=使用默认值 2" />
          </label>
          <label className="flex flex-col gap-1 text-sm">
            {renderFieldTitle("双态循环阈值", "默认：2。用于检测 A→B→A→B 这类来回循环并触发恢复。")}
            <input className="rounded-md border bg-background px-3 py-2" type="number" min={0} step="1" value={props.recoveryTwoStateLoopThreshold} onChange={(e) => props.setRecoveryTwoStateLoopThreshold(e.target.value)} placeholder="留空=使用默认值 2" />
          </label>
          <label className="flex flex-col gap-1 text-sm">
            {renderFieldTitle("高访问阈值", "默认：8。当页面访问次数过高且近期页面变化很少时触发恢复。")}
            <input className="rounded-md border bg-background px-3 py-2" type="number" min={0} step="1" value={props.recoveryHighVisitThreshold} onChange={(e) => props.setRecoveryHighVisitThreshold(e.target.value)} placeholder="留空=使用默认值 8" />
          </label>
          <label className="flex flex-col gap-1 text-sm">
            {renderFieldTitle("低奖励窗口", "默认：6。统计最近多少步页面变化，用来配合高访问阈值判断是否卡住。")}
            <input className="rounded-md border bg-background px-3 py-2" type="number" min={0} step="1" value={props.recoveryLowRewardWindow} onChange={(e) => props.setRecoveryLowRewardWindow(e.target.value)} placeholder="留空=使用默认值 6" />
          </label>
          <label className="flex flex-col gap-1 text-sm">
            {renderFieldTitle("候选歧义 Top Gap", "默认：0.15。候选分数越接近越容易判定为“区分度低”，从而触发更保守的候选增强判断。")}
            <input className="rounded-md border bg-background px-3 py-2" type="number" step="0.01" value={props.candidateAmbiguityTopGapThreshold} onChange={(e) => props.setCandidateAmbiguityTopGapThreshold(e.target.value)} placeholder="留空=使用默认值 0.15" />
          </label>
          <label className="flex flex-col gap-1 text-sm">
            {renderFieldTitle("高价值页面访问上限", "默认：2。页面访问次数不超过该值时仍视作“高价值新页”，会影响候选增强触发判断。")}
            <input className="rounded-md border bg-background px-3 py-2" type="number" min={0} step="1" value={props.highValuePageVisitLimit} onChange={(e) => props.setHighValuePageVisitLimit(e.target.value)} placeholder="留空=使用默认值 2" />
          </label>
          <label className="flex flex-col gap-1 text-sm">
            {renderFieldTitle("候选风险下降阈值", "默认：2.1。候选动作风险分高于该值时会被直接丢弃。")}
            <input className="rounded-md border bg-background px-3 py-2" type="number" step="0.1" value={props.candidateRiskDropThreshold} onChange={(e) => props.setCandidateRiskDropThreshold(e.target.value)} placeholder="留空=使用默认值 2.1" />
          </label>
          <label className="flex flex-col gap-1 text-sm md:col-span-2">
            {renderFieldTitle("候选最小融合分数", "默认：-0.3。融合分数低于该值的候选动作会被过滤。")}
            <input className="rounded-md border bg-background px-3 py-2" type="number" step="0.1" value={props.candidateMinFusionScore} onChange={(e) => props.setCandidateMinFusionScore(e.target.value)} placeholder="留空=使用默认值 -0.3" />
          </label>
        </div>
      ) : null}
      {props.configTab === "uct" ? (
        <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
          <label className="flex flex-col gap-1 text-sm md:col-span-2">
            {renderFieldTitle("决策算法", "默认：reuse，用于控制动作选择的核心策略。")}
            {renderSelect(props.algorithm, (value) => props.setAlgorithm(value as "" | "reuse" | "uctbandit" | "random"), "选择决策算法", [
              { value: "", label: "不指定（当前默认 reuse）" },
              { value: "reuse", label: "reuse" },
              { value: "uctbandit", label: "uctbandit" },
              { value: "random", label: "random" },
            ])}
          </label>
          {props.algorithm === "uctbandit" ? (
            <>
              <p className="text-sm text-muted-foreground md:col-span-2">
                当前已选择 `uctbandit`，下面显示该算法专属调参。
              </p>
              <label className="flex flex-col gap-1 text-sm">
                {renderFieldTitle("双态循环惩罚", "默认：-3.0。检测到 A↔B 往返循环时追加负奖励，降低重复选择概率。")}
                <input className="rounded-md border bg-background px-3 py-2" type="number" step="0.1" value={props.uctTwoStateLoopPenalty} onChange={(e) => props.setUctTwoStateLoopPenalty(e.target.value)} placeholder="留空=使用默认值 -3.0" />
              </label>
              <label className="flex flex-col gap-1 text-sm">
                {renderFieldTitle("边重复惩罚", "默认：-1.0。同一状态转移边重复过多时按次数叠加惩罚。")}
                <input className="rounded-md border bg-background px-3 py-2" type="number" step="0.1" value={props.uctEdgeRepeatPenalty} onChange={(e) => props.setUctEdgeRepeatPenalty(e.target.value)} placeholder="留空=使用默认值 -1.0" />
              </label>
              <label className="flex flex-col gap-1 text-sm">
                {renderFieldTitle("边重复阈值", "默认：2。同一条边重复超过该次数后才开始施加边重复惩罚。")}
                <input className="rounded-md border bg-background px-3 py-2" type="number" min={0} step="1" value={props.uctEdgeRepeatThreshold} onChange={(e) => props.setUctEdgeRepeatThreshold(e.target.value)} placeholder="留空=使用默认值 2" />
              </label>
              <label className="flex flex-col gap-1 text-sm">
                {renderFieldTitle("动作冷却惩罚", "默认：1.5。短时间内在同一页反复选同一动作时逐步降权，防止重复点击同一按钮。")}
                <input className="rounded-md border bg-background px-3 py-2" type="number" step="0.1" value={props.uctActionCooldownPenalty} onChange={(e) => props.setUctActionCooldownPenalty(e.target.value)} placeholder="留空=使用默认值 1.5" />
              </label>
              <label className="flex flex-col gap-1 text-sm">
                {renderFieldTitle("最近动作窗口", "默认：6。记录最近多少次动作选择，用于计算动作冷却惩罚。")}
                <input className="rounded-md border bg-background px-3 py-2" type="number" min={0} step="1" value={props.uctRecentActionWindow} onChange={(e) => props.setUctRecentActionWindow(e.target.value)} placeholder="留空=使用默认值 6" />
              </label>
              <label className="flex flex-col gap-1 text-sm">
                {renderFieldTitle("脱环探索增益", "默认：0.40。检测到回环或同状态卡死时临时提高随机探索概率以尽快脱困。")}
                <input className="rounded-md border bg-background px-3 py-2" type="number" step="0.1" value={props.uctLoopEscapeExploreBoost} onChange={(e) => props.setUctLoopEscapeExploreBoost(e.target.value)} placeholder="留空=使用默认值 0.40" />
              </label>
            </>
          ) : null}
          {props.algorithm === "reuse" ? (
            <>
              <p className="text-sm text-muted-foreground md:col-span-2">
                当前已选择 `reuse`，下面显示该算法的经验复用与学习参数。
              </p>
              <label className="flex flex-col gap-1 text-sm">
                {renderFieldTitle("探索率 epsilon", "默认：0.05。越大越偏向随机探索，越小越偏向复用历史高价值动作。")}
                <input className="rounded-md border bg-background px-3 py-2" type="number" min={0} max={1} step="0.001" value={props.reuseEpsilon} onChange={(e) => props.setReuseEpsilon(e.target.value)} placeholder="留空=使用默认值 0.05" />
              </label>
              <label className="flex flex-col gap-1 text-sm">
                {renderFieldTitle("折扣因子 gamma", "默认：0.8。越大越重视长期回报，越小越重视短期收益。")}
                <input className="rounded-md border bg-background px-3 py-2" type="number" min={0} max={1} step="0.001" value={props.reuseGamma} onChange={(e) => props.setReuseGamma(e.target.value)} placeholder="留空=使用默认值 0.8" />
              </label>
              <label className="flex flex-col gap-1 text-sm">
                {renderFieldTitle("N-Step 步数", "默认：5。学习更新时向后回看多少步奖励，越大越平滑。")}
                <input className="rounded-md border bg-background px-3 py-2" type="number" min={1} step="1" value={props.reuseNStep} onChange={(e) => props.setReuseNStep(e.target.value)} placeholder="留空=使用默认值 5" />
              </label>
              <label className="flex flex-col gap-1 text-sm">
                {renderFieldTitle("模型保存路径", "默认：./<包名>_reuse.model；无包名时为 ./default_reuse.model，用于持久化历史经验。")}
                <input className="rounded-md border bg-background px-3 py-2 font-mono" value={props.reuseModelSavePath} onChange={(e) => props.setReuseModelSavePath(e.target.value)} placeholder="留空=按包名自动生成默认路径" />
              </label>
              <label className="flex flex-col gap-1 text-sm">
                {renderFieldTitle("启用模型持久化", "默认：开启。关闭后不会加载也不会保存 reuse 经验模型。")}
                {renderSelect(props.reuseEnableModelPersistenceMode, (value) => props.setReuseEnableModelPersistenceMode(value as "" | "true" | "false"), "选择是否持久化", [
                  { value: "", label: "不指定（当前默认开启）" },
                  { value: "true", label: "开启" },
                  { value: "false", label: "关闭" },
                ])}
              </label>
              <label className="flex flex-col gap-1 text-sm">
                {renderFieldTitle("启动时重置模型", "默认：关闭。开启后会在启动时删除已有模型文件，从空经验重新学习。")}
                {renderSelect(props.reuseResetModelOnStartMode, (value) => props.setReuseResetModelOnStartMode(value as "" | "true" | "false"), "选择是否重置", [
                  { value: "", label: "不指定（当前默认关闭）" },
                  { value: "true", label: "开启" },
                  { value: "false", label: "关闭" },
                ])}
              </label>
            </>
          ) : null}
          {props.algorithm === "random" ? (
            <div className="rounded-md border bg-background p-3 text-sm text-muted-foreground md:col-span-2">
              `random` 当前没有额外的专属参数，主要用于随机探索或对比实验。
            </div>
          ) : null}
          {props.algorithm === "" ? (
            <div className="rounded-md border bg-background p-3 text-sm text-muted-foreground md:col-span-2">
              先选择一个决策算法，再显示对应的算法专属配置项。
            </div>
          ) : null}
        </div>
      ) : null}
    </TooltipProvider>
  )
}
