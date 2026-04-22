import { useState } from "react"

import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip"
import { buildNodeAttrSummary, buildNodeTitle, copyText } from "./utils"
import type { DumpTreeNode } from "./types"

type Props = {
  root: DumpTreeNode | null
  heightClassName: string
  expandedNodeIds: Record<string, boolean>
  selectedDumpNodeId: string
  onToggleNode: (id: string) => void
  onSelectNode: (id: string) => void
}

export function DumpPanel(props: Props) {
  const [detailExpandedNodeIds, setDetailExpandedNodeIds] = useState<Record<string, boolean>>({})
  const [copiedNodeId, setCopiedNodeId] = useState("")

  const toggleDetail = (id: string) => {
    setDetailExpandedNodeIds((prev) => ({
      ...prev,
      [id]: !(prev[id] ?? false),
    }))
  }

  const handleCopyAttrs = async (id: string, attrs: Record<string, string>) => {
    await copyText(JSON.stringify(attrs, null, 2))
    setCopiedNodeId(id)
    setTimeout(() => {
      setCopiedNodeId((prev) => (prev === id ? "" : prev))
    }, 1200)
  }

  const renderDumpTree = (node: DumpTreeNode, depth: number) => {
    const hasChildren = node.children.length > 0
    const expanded = props.expandedNodeIds[node.id] ?? false
    const selected = props.selectedDumpNodeId === node.id
    const detailExpanded = detailExpandedNodeIds[node.id] ?? false
    return (
      <li key={node.id}>
        <div
          className={`mb-1 flex items-start gap-1 rounded px-1 py-0.5 ${selected ? "bg-emerald-100" : ""}`}
          style={{ marginLeft: `${depth * 12}px` }}
        >
          {hasChildren ? (
            <button
              type="button"
              className="mt-[2px] h-5 w-5 rounded border text-xs"
              onClick={() => props.onToggleNode(node.id)}
              aria-label={expanded ? "收起节点" : "展开节点"}
            >
              {expanded ? "-" : "+"}
            </button>
          ) : (
            <span className="inline-block h-5 w-5 text-center text-xs text-muted-foreground">·</span>
          )}
          <Tooltip>
            <TooltipTrigger asChild>
              <button
                type="button"
                className="min-w-0 flex-1 text-left font-mono text-xs"
                onClick={() => props.onSelectNode(node.id)}
              >
                <div className="truncate">{buildNodeTitle(node)}</div>
                <div className="truncate text-[11px] text-muted-foreground">
                  bounds: {node.attrs.bounds || "<none>"}
                </div>
                <div className="truncate text-[11px] text-muted-foreground">
                  {buildNodeAttrSummary(node)}
                </div>
              </button>
            </TooltipTrigger>
            <TooltipContent side="right" align="start" sideOffset={8} className="max-w-xl">
              <pre className="max-h-56 overflow-auto whitespace-pre-wrap break-all">
                {JSON.stringify(node.attrs, null, 2)}
              </pre>
            </TooltipContent>
          </Tooltip>
          <button
            type="button"
            className="shrink-0 rounded border px-2 py-1 text-[11px] text-muted-foreground hover:bg-muted"
            onClick={() => toggleDetail(node.id)}
          >
            {detailExpanded ? "收起" : "详情"}
          </button>
          <button
            type="button"
            className="shrink-0 rounded border px-2 py-1 text-[11px] text-muted-foreground hover:bg-muted"
            onClick={() => void handleCopyAttrs(node.id, node.attrs)}
          >
            {copiedNodeId === node.id ? "已复制" : "复制"}
          </button>
        </div>
        {detailExpanded ? (
          <div style={{ marginLeft: `${depth * 12 + 24}px` }} className="mb-2">
            <pre className="max-h-40 overflow-auto rounded border bg-muted/30 p-2 text-[11px] whitespace-pre-wrap break-all">
              {JSON.stringify(node.attrs, null, 2)}
            </pre>
          </div>
        ) : null}
        {hasChildren && expanded ? <ul>{node.children.map((child) => renderDumpTree(child, depth + 1))}</ul> : null}
      </li>
    )
  }

  return (
    <TooltipProvider delayDuration={120}>
      <div className={`${props.heightClassName} min-h-0 w-full overflow-auto rounded-md border bg-background p-3`}>
        {props.root !== null ? (
          <ul>{renderDumpTree(props.root, 0)}</ul>
        ) : (
          <p className="font-mono text-xs text-muted-foreground">暂无 XML，请先刷新预览。</p>
        )}
      </div>
    </TooltipProvider>
  )
}
