import { PageNode } from '@/lib/api'

export function filterTreeWithOpenNodes(
  node: PageNode,
  query: string,
): { filtered: PageNode | null; expandedIds: Set<string> } {
  const expandedIds = new Set<string>()

  function recurse(current: PageNode): PageNode | null {
    const matches = current.title.toLowerCase().includes(query.toLowerCase())

    const children = current.children || []
    const filteredChildren = children.map(recurse).filter(Boolean) as PageNode[]

    if (matches || filteredChildren.length > 0) {
      if (filteredChildren.length > 0) {
        expandedIds.add(current.id)
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
