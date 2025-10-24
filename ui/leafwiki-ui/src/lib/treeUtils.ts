import { PageNode } from '@/lib/api/pages'

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
