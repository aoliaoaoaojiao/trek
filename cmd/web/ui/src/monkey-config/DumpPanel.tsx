import { buildNodeTitle } from "./utils"
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
  const renderDumpTree = (node: DumpTreeNode, depth: number) => {
    const hasChildren = node.children.length > 0
    const expanded = props.expandedNodeIds[node.id] ?? false
    const selected = props.selectedDumpNodeId === node.id
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
          <button
            type="button"
            className="min-w-0 flex-1 text-left font-mono text-xs"
            onClick={() => props.onSelectNode(node.id)}
            title={node.attrs.bounds || ""}
          >
            <div className="truncate">{buildNodeTitle(node)}</div>
            <div className="truncate text-[11px] text-muted-foreground">
              bounds: {node.attrs.bounds || "<none>"}
            </div>
          </button>
        </div>
        {hasChildren && expanded ? <ul>{node.children.map((child) => renderDumpTree(child, depth + 1))}</ul> : null}
      </li>
    )
  }

  return (
    <div className={`${props.heightClassName} min-h-0 w-full overflow-auto rounded-md border bg-background p-3`}>
      {props.root !== null ? (
        <ul>{renderDumpTree(props.root, 0)}</ul>
      ) : (
        <p className="font-mono text-xs text-muted-foreground">暂无 XML，请先刷新预览。</p>
      )}
    </div>
  )
}

