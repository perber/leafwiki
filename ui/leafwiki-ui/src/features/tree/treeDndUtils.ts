import { NODE_KIND_SECTION, PageNode } from '@/lib/api/pages'

export const ROOT_ID = 'root'

export type DropZone = 'before' | 'after' | 'inside'

export type DropTarget = {
  nodeId: string
  zone: DropZone
}

export type DropResolution = {
  parentId: string
  // Index among the parent's children with the dragged node removed
  index: number
}

export function collectSubtreeIds(node: PageNode): Set<string> {
  const ids = new Set<string>()
  const walk = (n: PageNode) => {
    ids.add(n.id)
    for (const child of n.children || []) walk(child)
  }
  walk(node)
  return ids
}

// Every row splits 25/50/25: top quarter = before, middle half = into,
// bottom quarter = after. Dropping "into" a page nests the dragged node
// inside it — the backend converts the page into a section on move.
export function getDropZone(
  pointerY: number,
  rect: { top: number; height: number },
): DropZone | null {
  if (rect.height <= 0) return null
  const fraction = (pointerY - rect.top) / rect.height

  if (fraction < 0.25) return 'before'
  if (fraction > 0.75) return 'after'
  return 'inside'
}

export function resolveDrop({
  dragged,
  target,
  zone,
  byId,
  isNodeOpen,
}: {
  dragged: PageNode
  target: PageNode
  zone: DropZone
  byId: Record<string, PageNode>
  isNodeOpen: (id: string) => boolean
}): DropResolution | null {
  if (zone === 'inside') {
    const children = (target.children ?? []).filter((c) => c.id !== dragged.id)
    return { parentId: target.id, index: children.length }
  }

  const parentId = target.parentId ?? ROOT_ID
  const parent = byId[parentId]
  const siblings = (parent?.children ?? []).filter((c) => c.id !== dragged.id)
  const targetIndex = siblings.findIndex((c) => c.id === target.id)
  if (targetIndex === -1) return null

  if (zone === 'after') {
    // Dropping right below an expanded section visually points at the slot
    // above its first child, so place the node there instead of after the
    // whole section.
    if (
      target.kind === NODE_KIND_SECTION &&
      isNodeOpen(target.id) &&
      (target.children ?? []).some((c) => c.id !== dragged.id)
    ) {
      return { parentId: target.id, index: 0 }
    }
    return { parentId, index: targetIndex + 1 }
  }

  return { parentId, index: targetIndex }
}

export function buildOrderedIds(
  children: PageNode[],
  draggedId: string,
  index: number,
): string[] {
  const ids = children.map((c) => c.id).filter((id) => id !== draggedId)
  const insertAt = Math.max(0, Math.min(index, ids.length))
  ids.splice(insertAt, 0, draggedId)
  return ids
}
