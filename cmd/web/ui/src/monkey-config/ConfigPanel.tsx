import { Button } from "@/components/ui/button"

import type { ActionType, DeviceOption, PageActionRule } from "./types"

type ConfigTab = "base" | "action" | "preview"

type Props = {
  configTab: ConfigTab
  setConfigTab: (tab: ConfigTab) => void
  loading: boolean
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
  onCopyText: (text: string) => Promise<void>
  previewConfigText: string
  outputPath: string
  setOutputPath: (value: string) => void
  onPreviewConfig: () => void
  onSaveConfig: () => void
  status: string
  error: string
  resultText: string
}

export function ConfigPanel(props: Props) {
  const rangeMatchText = `serial=${props.usedSerial || props.deviceSerial || "<empty>"}，package=${props.currentPackageName || "<empty>"}`

  return (
    <>
      <div className="mb-3 flex items-center justify-between">
        <h2 className="text-base font-semibold">配置</h2>
        <Button variant="outline" onClick={props.onRefreshCurrentPage} disabled={props.loading}>
          当前界面
        </Button>
      </div>
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
            页面源 page_source
            <select className="rounded-md border bg-background px-3 py-2" value={props.pageSource} onChange={(e) => props.setPageSource(e.target.value as "uia" | "poco")}>
              <option value="uia">uia</option>
              <option value="poco">poco</option>
            </select>
          </label>
          <label className="flex flex-col gap-1 text-sm">
            触控模式 touch_mode
            <select className="rounded-md border bg-background px-3 py-2" value={props.touchMode} onChange={(e) => props.setTouchMode(e.target.value as "motion" | "uia" | "adb")}>
              <option value="motion">motion</option>
              <option value="uia">uia</option>
              <option value="adb">adb</option>
            </select>
          </label>
          <label className="flex flex-col gap-1 text-sm">
            UIA 端口 uia.server_port
            <input className="rounded-md border bg-background px-3 py-2" type="number" min={0} value={props.uiaPort} onChange={(e) => props.setUiaPort(e.target.value)} placeholder="默认空" />
          </label>
          <label className="flex flex-col gap-1 text-sm">
            日志文件级别 log.file_level
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
                Poco 引擎 poco.engine
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
                Poco 端口 poco.port
                <input className="rounded-md border bg-background px-3 py-2" type="number" min={0} value={props.pocoPort} onChange={(e) => props.setPocoPort(e.target.value)} placeholder="默认空（走引擎默认端口）" />
              </label>
            </>
          ) : null}
          <div className="md:col-span-2 rounded-md border bg-background p-3">
            <p className="mb-2 text-sm font-medium">有效触控区域</p>
            <div className="mb-3 grid grid-cols-1 gap-2 rounded-md border p-2 text-xs md:grid-cols-2">
              <label className="flex flex-col gap-1">
                left
                <input className="rounded border bg-background px-2 py-1 font-mono" value={props.rangeLeftInput} onChange={(event) => props.setRangeLeftInput(event.target.value)} />
              </label>
              <label className="flex flex-col gap-1">
                top
                <input className="rounded border bg-background px-2 py-1 font-mono" value={props.rangeTopInput} onChange={(event) => props.setRangeTopInput(event.target.value)} />
              </label>
              <label className="flex flex-col gap-1">
                right
                <input className="rounded border bg-background px-2 py-1 font-mono" value={props.rangeRightInput} onChange={(event) => props.setRangeRightInput(event.target.value)} />
              </label>
              <label className="flex flex-col gap-1">
                bottom
                <input className="rounded border bg-background px-2 py-1 font-mono" value={props.rangeBottomInput} onChange={(event) => props.setRangeBottomInput(event.target.value)} />
              </label>
              <div className="flex flex-wrap gap-2 md:col-span-2">
                <Button type="button" size="sm" variant="outline" onClick={props.onResetRange}>
                  恢复默认
                </Button>
              </div>
              <p className="break-all font-mono text-[11px] text-muted-foreground md:col-span-2">
                范围匹配: {rangeMatchText}，当前公式: x' = left + (right-left) * x，y' = top + (bottom-top) * y
              </p>
              <p className="break-all font-mono text-[11px] text-muted-foreground md:col-span-2">范围状态: {props.rangeLog}</p>
            </div>
          </div>
          <label className="flex items-center gap-2 text-sm md:col-span-2">
            <input type="checkbox" checked={props.skipAll} onChange={(e) => props.setSkipAll(e.target.checked)} />
            跳过模型动作 skip_all_actions_from_model
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
          <div className="mb-2 flex items-center justify-between">
            <p className="text-sm font-medium">当前界面配置预览</p>
            <Button type="button" size="sm" variant="outline" onClick={() => void props.onCopyText(props.previewConfigText)}>
              复制预览
            </Button>
          </div>
          <p className="mb-2 break-all font-mono text-[11px] text-muted-foreground">
            范围匹配: {rangeMatchText}；页面: {props.currentPageName || "<empty>"}
          </p>
          <textarea className="min-h-[460px] w-full rounded-md border bg-background p-2 font-mono text-xs" readOnly value={props.previewConfigText} />
          <label className="mt-3 flex flex-col gap-1 text-sm">
            输出路径
            <input className="rounded-md border bg-background px-3 py-2" value={props.outputPath} onChange={(e) => props.setOutputPath(e.target.value)} />
          </label>
          <div className="mt-3 flex flex-wrap gap-2">
            <Button onClick={props.onPreviewConfig} disabled={props.loading}>
              预览配置
            </Button>
            <Button variant="outline" onClick={props.onSaveConfig} disabled={props.loading}>
              保存到文件
            </Button>
          </div>
          {props.status !== "" ? <p className="mt-2 text-sm text-emerald-700">{props.status}</p> : null}
          {props.error !== "" ? <p className="mt-2 text-sm text-red-700">{props.error}</p> : null}
          <div className="mt-3">
            <label className="text-sm font-medium">生成结果</label>
            <textarea className="mt-2 min-h-72 w-full rounded-md border bg-background p-3 font-mono text-sm" readOnly value={props.resultText} />
          </div>
        </div>
      ) : null}
    </>
  )
}

