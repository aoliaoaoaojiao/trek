import { useState } from "react"

import { Button } from "@/components/ui/button"

import type { ActionType, DeviceOption, PageActionRule, PageNameStrategy } from "./types"

type ConfigTab = "base" | "action" | "preview"

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
  currentPageName: string
  pageSource: "uia" | "poco"
  setPageSource: (value: "uia" | "poco") => void
  pageNameStrategy: PageNameStrategy
  setPageNameStrategy: (value: PageNameStrategy) => void
  touchMode: "motion" | "uia" | "adb"
  setTouchMode: (value: "motion" | "uia" | "adb") => void
  uiaPort: string
  setUiaPort: (value: string) => void
  fileLevel: string
  setFileLevel: (value: string) => void
  pocoEngine: string
  setPocoEngine: (value: string) => void
  pocoPort: string
  setPocoPort: (value: string) => void
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
  actionType: ActionType
  setActionType: (value: ActionType) => void
  actionPath: string
  setActionPath: (value: string) => void
  actionX: string
  setActionX: (value: string) => void
  actionY: string
  setActionY: (value: string) => void
  actionStartX: string
  setActionStartX: (value: string) => void
  actionStartY: string
  setActionStartY: (value: string) => void
  actionEndX: string
  setActionEndX: (value: string) => void
  actionEndY: string
  setActionEndY: (value: string) => void
  actionRules: PageActionRule[]
  actionLog: string
  onAddActionRule: () => void
  outputPath: string
  setOutputPath: (value: string) => void
  onCopyConfig: () => void
  onSaveConfig: () => void
  status: string
  error: string
  resultText: string
}

export function ConfigPanel(props: Props) {
  const rangeMatchText = `serial=${props.usedSerial || props.deviceSerial || "<empty>"}，package=${props.currentPackageName || "<empty>"}`
  const [importOpen, setImportOpen] = useState(false)
  const [importText, setImportText] = useState("")
  const [importStatus, setImportStatus] = useState("")

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

  return (
    <>
      <div className="mb-3 flex items-center justify-between">
        <h2 className="text-base font-semibold">配置</h2>
        <div className="flex items-center gap-2">
          <Button type="button" variant="outline" onClick={() => setImportOpen(true)} disabled={props.loading}>
            导入配置
          </Button>
          <Button variant="outline" onClick={props.onRefreshCurrentPage} disabled={props.loading}>
            当前界面
          </Button>
        </div>
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
      <div className="mb-3 flex gap-2">
        <Button type="button" variant={props.configTab === "base" ? "default" : "outline"} onClick={() => props.setConfigTab("base")}>
          基础配置
        </Button>
        <Button type="button" variant={props.configTab === "action" ? "default" : "outline"} onClick={() => props.setConfigTab("action")}>
          动作配置
        </Button>
        <Button type="button" variant={props.configTab === "preview" ? "default" : "outline"} onClick={() => props.setConfigTab("preview")}>
          预览配置
        </Button>
      </div>
      {props.configTab === "base" ? (
        <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
          <div className="flex flex-col gap-1 text-sm md:col-span-2">
            <label>设备列表</label>
            <div className="flex items-center gap-2">
              <select
                className="min-w-0 flex-1 rounded-md border bg-background px-3 py-2"
                value={props.deviceSerial}
                onChange={(e) => props.setDeviceSerial(e.target.value)}
              >
                <option value="">不指定（由系统自动选择）</option>
                {props.deviceOptions.map((item) => (
                  <option key={item.serial} value={item.serial}>
                    {item.label}
                  </option>
                ))}
              </select>
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
            页面源
            <select className="rounded-md border bg-background px-3 py-2" value={props.pageSource} onChange={(e) => props.setPageSource(e.target.value as "uia" | "poco")}>
              <option value="uia">uia</option>
              <option value="poco">poco</option>
            </select>
          </label>
          <label className="flex flex-col gap-1 text-sm">
            页面名策略
            <select
              className="rounded-md border bg-background px-3 py-2"
              value={props.pageNameStrategy}
              onChange={(e) => props.setPageNameStrategy(e.target.value as PageNameStrategy)}
            >
              <option value="">不指定（按页面源自动）</option>
              <option value="structure_fingerprint">structure_fingerprint（结构指纹）</option>
              <option value="uia_activity_first">uia_activity_first（UIA 优先 Activity）</option>
              <option value="xml_only">xml_only（XML 指纹）</option>
              <option value="xml_fingerprint">xml_fingerprint（XML 指纹）</option>
              <option value="activity_only">activity_only（仅 Activity）</option>
            </select>
          </label>
          <label className="flex flex-col gap-1 text-sm">
            触控模式
            <select className="rounded-md border bg-background px-3 py-2" value={props.touchMode} onChange={(e) => props.setTouchMode(e.target.value as "motion" | "uia" | "adb")}>
              <option value="motion">motion</option>
              <option value="uia">uia</option>
              <option value="adb">adb</option>
            </select>
          </label>
          <label className="flex flex-col gap-1 text-sm">
            UIA 端口
            <input className="rounded-md border bg-background px-3 py-2" type="number" min={0} value={props.uiaPort} onChange={(e) => props.setUiaPort(e.target.value)} placeholder="默认空" />
          </label>
          <label className="flex flex-col gap-1 text-sm">
            日志文件级别
            <select className="rounded-md border bg-background px-3 py-2" value={props.fileLevel} onChange={(e) => props.setFileLevel(e.target.value)}>
              <option value="">空（不输出）</option>
              <option value="debug">debug</option>
              <option value="info">info</option>
              <option value="warn">warn</option>
              <option value="error">error</option>
            </select>
          </label>
          {props.pageSource === "poco" ? (
            <>
              <label className="flex flex-col gap-1 text-sm">
                Poco 引擎
                <select className="rounded-md border bg-background px-3 py-2" value={props.pocoEngine} onChange={(e) => props.setPocoEngine(e.target.value)}>
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
                Poco 端口
                <input className="rounded-md border bg-background px-3 py-2" type="number" min={0} value={props.pocoPort} onChange={(e) => props.setPocoPort(e.target.value)} placeholder="默认空（走引擎默认端口）" />
              </label>
            </>
          ) : null}
          <div className="md:col-span-2 rounded-md border bg-background p-3">
            <p className="mb-2 text-sm font-medium">有效触控区域</p>
            <div className="mb-3 grid grid-cols-1 gap-2 rounded-md border p-2 text-xs md:grid-cols-2">
              <label className="flex flex-col gap-1">
                左边界
                <input className="rounded border bg-background px-2 py-1 font-mono" value={props.rangeLeftInput} onChange={(event) => props.setRangeLeftInput(event.target.value)} />
              </label>
              <label className="flex flex-col gap-1">
                上边界
                <input className="rounded border bg-background px-2 py-1 font-mono" value={props.rangeTopInput} onChange={(event) => props.setRangeTopInput(event.target.value)} />
              </label>
              <label className="flex flex-col gap-1">
                右边界
                <input className="rounded border bg-background px-2 py-1 font-mono" value={props.rangeRightInput} onChange={(event) => props.setRangeRightInput(event.target.value)} />
              </label>
              <label className="flex flex-col gap-1">
                下边界
                <input className="rounded border bg-background px-2 py-1 font-mono" value={props.rangeBottomInput} onChange={(event) => props.setRangeBottomInput(event.target.value)} />
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
          <label className="flex items-center gap-2 text-sm md:col-span-2">
            <input type="checkbox" checked={props.skipAll} onChange={(e) => props.setSkipAll(e.target.checked)} />
            跳过模型动作
          </label>
        </div>
      ) : null}
      {props.configTab === "action" ? (
        <div className="mt-2 rounded-md border bg-background p-3">
          <p className="mb-3 text-sm font-medium">动作配置</p>
          <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
            <label className="flex flex-col gap-1 text-sm">
              页面名（当前界面）
              <input className="rounded-md border bg-background px-3 py-2 font-mono" value={props.currentPageName} readOnly placeholder="点击当前界面后自动填充" />
            </label>
            <label className="flex flex-col gap-1 text-sm">
              动作类型
              <select className="rounded-md border bg-background px-3 py-2 font-mono" value={props.actionType} onChange={(event) => props.setActionType(event.target.value as ActionType)}>
                <option value="click">点击</option>
                <option value="scroll">滑动</option>
                <option value="long_press">长按</option>
                <option value="custom_touch">自定义触控</option>
              </select>
            </label>
            <label className="flex flex-col gap-1 text-sm md:col-span-2">
              path（可选；与坐标至少填一个）
              <input className="rounded-md border bg-background px-3 py-2 font-mono" value={props.actionPath} onChange={(event) => props.setActionPath(event.target.value)} placeholder="/hierarchy/..." />
            </label>
            {props.actionType === "scroll" ? (
              <>
                <label className="flex flex-col gap-1 text-sm">
                  开始坐标X
                  <input className="rounded-md border bg-background px-3 py-2 font-mono" value={props.actionStartX} onChange={(event) => props.setActionStartX(event.target.value)} />
                </label>
                <label className="flex flex-col gap-1 text-sm">
                  开始坐标Y
                  <input className="rounded-md border bg-background px-3 py-2 font-mono" value={props.actionStartY} onChange={(event) => props.setActionStartY(event.target.value)} />
                </label>
                <label className="flex flex-col gap-1 text-sm">
                  结束坐标X
                  <input className="rounded-md border bg-background px-3 py-2 font-mono" value={props.actionEndX} onChange={(event) => props.setActionEndX(event.target.value)} />
                </label>
                <label className="flex flex-col gap-1 text-sm">
                  结束坐标Y
                  <input className="rounded-md border bg-background px-3 py-2 font-mono" value={props.actionEndY} onChange={(event) => props.setActionEndY(event.target.value)} />
                </label>
              </>
            ) : (
              <>
                <label className="flex flex-col gap-1 text-sm">
                  坐标X
                  <input className="rounded-md border bg-background px-3 py-2 font-mono" value={props.actionX} onChange={(event) => props.setActionX(event.target.value)} />
                </label>
                <label className="flex flex-col gap-1 text-sm">
                  坐标Y
                  <input className="rounded-md border bg-background px-3 py-2 font-mono" value={props.actionY} onChange={(event) => props.setActionY(event.target.value)} />
                </label>
              </>
            )}
            <div className="md:col-span-2">
              <Button type="button" size="sm" variant="outline" onClick={props.onAddActionRule}>
                添加动作
              </Button>
            </div>
            <p className="break-all font-mono text-[11px] text-muted-foreground md:col-span-2">{props.actionLog}</p>
            <div className="md:col-span-2 rounded border p-2">
              <p className="mb-1 text-sm font-medium">页面动作配置</p>
              <textarea className="min-h-40 w-full rounded-md border bg-background p-2 font-mono text-xs" readOnly value={JSON.stringify(props.actionRules, null, 2)} />
            </div>
          </div>
        </div>
      ) : null}
      {props.configTab === "preview" ? (
        <div className="mt-2 rounded-md border bg-background p-3">
          <label className="flex flex-col gap-1 text-sm">
            输出路径
            <input className="rounded-md border bg-background px-3 py-2" value={props.outputPath} onChange={(e) => props.setOutputPath(e.target.value)} />
          </label>
          <div className="mt-3 flex flex-wrap gap-2">
            <Button onClick={props.onCopyConfig} disabled={props.resultText.trim() === ""}>
              复制配置
            </Button>
            <Button variant="outline" onClick={props.onSaveConfig} disabled={props.loading}>
              保存到文件
            </Button>
          </div>
          {props.status !== "" ? <p className="mt-2 text-sm text-emerald-700">{props.status}</p> : null}
          {props.error !== "" ? <p className="mt-2 text-sm text-red-700">{props.error}</p> : null}
          <div className="mt-3">
            <label className="text-sm font-medium">生成结果</label>
            <textarea className="mt-2 min-h-[520px] w-full rounded-md border bg-background p-3 font-mono text-sm" readOnly value={props.resultText} />
          </div>
        </div>
      ) : null}
    </>
  )
}
