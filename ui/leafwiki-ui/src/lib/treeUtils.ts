import { PageNode } from '@/lib/api'

export function filterTreeWithOpenNodes(
  node: PageNode | null,
  query: string,
): { filtered: PageNode | null; expandedIds: string[] } {
  const expandedIds: string[] = []

  if (!node) {
    return { filtered: null, expandedIds }
  }

  function recurse(current: PageNode): PageNode | null {
    const matches = current.title.toLowerCase().includes(query.toLowerCase())

    const children = current.children || []
    const filteredChildren = children.map(recurse).filter(Boolean) as PageNode[]

    if (matches || filteredChildren.length > 0) {
      if (filteredChildren.length > 0) {
        expandedIds.push(current.id)
      }

      return {
        ...current,
        children: filteredChildren,
      }
    }

    return null
  }

  const filtered = recurse(node)
  return { filtered, expandedIds }
}

export function getAncestorIds(tree: PageNode, pageId: string): string[] {
  const byId = new Map<string, PageNode>()

  function collect(node: PageNode) {
    byId.set(node.id, node)
    for (const child of node.children || []) {
      collect(child)
    }
  }

  collect(tree)

  const result: string[] = []
  let current = byId.get(pageId)

  while (current?.parentId) {
    result.unshift(current.parentId)
    current = byId.get(current.parentId)
  }

  return result
}

export function assignParentIds(
  node: PageNode,
  parentId: string | null = null,
) {
  node.parentId = parentId
  for (const child of node.children || []) {
    assignParentIds(child, node.id)
  }
}
