import type { MouseEvent } from "react"

import { Button } from "@/components/ui/button"

import type { ClickPoint, EffectiveRange } from "./types"

type HighlightRect = {
  left: number
  top: number
  width: number
  height: number
}

type Props = {
  screenshotBase64: string
  imgClassName: string
  effectiveRange: EffectiveRange
  rangeWidth: number
  rangeHeight: number
  highlightRect: HighlightRect | null
  clickPoint: ClickPoint | null
  highlightLog: string
  clickLog: string
  absoluteWidth: number
  absoluteHeight: number
  pageName: string
  onImageLoad: (width: number, height: number) => void
  onImageClick: (event: MouseEvent<HTMLImageElement>) => void
  onCopyText: (text: string) => Promise<void>
}

export function ScreenshotPanel(props: Props) {
  return (
    <div className="rounded-md border bg-muted/30 p-2">
      {props.screenshotBase64 !== "" ? (
        <div className="space-y-2">
          <div className="flex justify-center">
            <div className="relative inline-block overflow-hidden rounded">
              <img
                alt="设备截图"
                className={`${props.imgClassName} cursor-crosshair`}
                src={`data:image/png;base64,${props.screenshotBase64}`}
                onLoad={(event) =>
                  props.onImageLoad(event.currentTarget.naturalWidth, event.currentTarget.naturalHeight)
                }
                onClick={props.onImageClick}
              />
              <div
                className="pointer-events-none absolute z-[5] border border-yellow-400"
                style={{
                  left: `${props.effectiveRange.left * 100}%`,
                  top: `${props.effectiveRange.top * 100}%`,
                  width: `${props.rangeWidth * 100}%`,
                  height: `${props.rangeHeight * 100}%`,
                }}
              />
              {props.highlightRect !== null ? (
                <div
                  className="pointer-events-none absolute z-10 border-2 border-red-500 bg-red-500/20"
                  style={{
                    left: `${props.highlightRect.left}%`,
                    top: `${props.highlightRect.top}%`,
                    width: `${props.highlightRect.width}%`,
                    height: `${props.highlightRect.height}%`,
                  }}
                />
              ) : null}
              {props.clickPoint !== null ? (
                <div
                  className="pointer-events-none absolute z-20 h-3 w-3 -translate-x-1/2 -translate-y-1/2 rounded-full border-2 border-blue-600 bg-blue-300/80"
                  style={{
                    left: `${props.clickPoint.imagePercentX * 100}%`,
                    top: `${props.clickPoint.imagePercentY * 100}%`,
                  }}
                />
              ) : null}
            </div>
          </div>
          <div className="rounded-md border bg-background p-3">
            {props.pageName !== "" && (
              <div className="mb-2 flex items-center gap-2">
                <p className="break-all font-mono text-[11px] text-muted-foreground">页面名: {props.pageName}</p>
                <Button
                  type="button"
                  size="sm"
                  variant="outline"
                  onClick={() => void props.onCopyText(props.pageName)}
                >
                  复制
                </Button>
              </div>
            )}
            <p className="break-all font-mono text-[11px] text-muted-foreground">高亮日志: {props.highlightLog}</p>
            <div className="mt-2 grid grid-cols-1 gap-2 text-xs md:grid-cols-[1fr_auto] md:items-center">
              <p className="break-all font-mono">
                有效区百分比(0~1):{" "}
                {props.clickPoint === null
                  ? "-"
                  : `x=${props.clickPoint.percentX.toFixed(6)}, y=${props.clickPoint.percentY.toFixed(6)}`}
              </p>
              <Button
                type="button"
                size="sm"
                variant="outline"
                disabled={props.clickPoint === null}
                onClick={() =>
                  void props.onCopyText(
                    props.clickPoint === null ? "" : `${props.clickPoint.percentX.toFixed(6)},${props.clickPoint.percentY.toFixed(6)}`
                  )
                }
              >
                复制百分比
              </Button>
              <p className="break-all font-mono">
                整图百分比(0~1):{" "}
                {props.clickPoint === null
                  ? "-"
                  : `x=${props.clickPoint.imagePercentX.toFixed(6)}, y=${props.clickPoint.imagePercentY.toFixed(6)}`}
              </p>
              <Button
                type="button"
                size="sm"
                variant="outline"
                disabled={props.clickPoint === null}
                onClick={() =>
                  void props.onCopyText(
                    props.clickPoint === null ? "" : `${props.clickPoint.imagePercentX.toFixed(6)},${props.clickPoint.imagePercentY.toFixed(6)}`
                  )
                }
              >
                复制整图百分比
              </Button>
              <p className="break-all font-mono">
                绝对坐标(设备原始):{" "}
                {props.clickPoint === null
                  ? "-"
                  : `x=${props.clickPoint.absoluteX.toFixed(1)}, y=${props.clickPoint.absoluteY.toFixed(1)}`}
              </p>
              <Button
                type="button"
                size="sm"
                variant="outline"
                disabled={props.clickPoint === null}
                onClick={() =>
                  void props.onCopyText(
                    props.clickPoint === null ? "" : `${props.clickPoint.absoluteX.toFixed(1)},${props.clickPoint.absoluteY.toFixed(1)}`
                  )
                }
              >
                复制绝对坐标
              </Button>
            </div>
            <p className="mt-2 break-all font-mono text-[11px] text-muted-foreground">调试日志: {props.clickLog}</p>
            <p className="mt-1 break-all font-mono text-[11px] text-muted-foreground">
              当前坐标基准: 截图原始像素 ({props.absoluteWidth.toFixed(1)}x{props.absoluteHeight.toFixed(1)})
            </p>
          </div>
        </div>
      ) : (
        <p className="text-sm text-muted-foreground">暂无截图，请先刷新预览。</p>
      )}
    </div>
  )
}

