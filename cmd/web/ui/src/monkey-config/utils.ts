import type { BoundsRect, DumpTreeNode } from "./types"

export function parseBounds(raw: string): BoundsRect | null {
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

export function parseDumpTree(xml: string): { root: DumpTreeNode | null; nodeMap: Map<string, DumpTreeNode> } {
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

export function buildNodeTitle(node: DumpTreeNode): string {
  const textLikeKeys = [
    "text",
    "label",
    "value",
    "title",
    "content-desc",
    "name",
    "resource-id",
    "class",
  ]
  const key =
    textLikeKeys
      .map((k) => node.attrs[k])
      .find((v) => typeof v === "string" && v.trim() !== "") ?? "<empty>"
  return `${node.tag}  ${key}`
}

export function buildNodeAttrSummary(node: DumpTreeNode): string {
  const preferredKeys = [
    "text",
    "label",
    "value",
    "title",
    "name",
    "content-desc",
    "resource-id",
    "class",
    "type",
    "widget",
    "clickable",
    "enabled",
    "focusable",
    "scrollable",
  ]
  const summary: string[] = []
  const used = new Set<string>()

  for (const key of preferredKeys) {
    const value = node.attrs[key]
    if (value === undefined || value.trim() === "") {
      continue
    }
    summary.push(`${key}=${value}`)
    used.add(key)
    if (summary.length >= 4) {
      break
    }
  }

  if (summary.length < 4) {
    const rest = Object.entries(node.attrs)
      .filter(([key, value]) => !used.has(key) && value.trim() !== "")
      .slice(0, 4 - summary.length)
      .map(([key, value]) => `${key}=${value}`)
    summary.push(...rest)
  }

  return summary.length > 0 ? summary.join(" | ") : "attrs: <empty>"
}

export async function copyText(text: string) {
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
  } catch {
    // ignore
  }
}
