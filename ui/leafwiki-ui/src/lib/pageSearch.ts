import { PageNode } from '@/lib/api/pages'

type NormalizedFlatPageSearchItem = {
  normalizedTitle: string
  normalizedPath: string
  normalizedBreadcrumb: string
}

export type FlatPageSearchItem = {
  id: string
  title: string
  path: string
  kind: PageNode['kind']
  breadcrumb: string
  searchText: string
} & NormalizedFlatPageSearchItem

type SearchScoringOptions = {
  pathStartsWithScore?: number
}

type ScoredSearchItem = {
  item: FlatPageSearchItem
  score: number
}

function normalize(value: string) {
  return value.toLowerCase().trim()
}

function compareScoredItems(a: ScoredSearchItem, b: ScoredSearchItem) {
  if (b.score !== a.score) return b.score - a.score
  return a.item.title.localeCompare(b.item.title)
}

function pushTopResult(
  results: ScoredSearchItem[],
  entry: ScoredSearchItem,
  limit: number,
) {
  if (
    results.length === limit &&
    compareScoredItems(results[limit - 1], entry) <= 0
  ) {
    return
  }

  let index = 0
  while (
    index < results.length &&
    compareScoredItems(results[index], entry) <= 0
  ) {
    index += 1
  }

  results.splice(index, 0, entry)

  if (results.length > limit) {
    results.pop()
  }
}

export function buildFlatPageSearchItems(
  root: PageNode | null,
): FlatPageSearchItem[] {
  if (!root) return []

  const items: FlatPageSearchItem[] = []

  const walk = (node: PageNode, parents: string[]) => {
    const breadcrumbParts = [...parents, node.title]

    if (node.path) {
      const breadcrumb = breadcrumbParts.join(' / ')
      const normalizedTitle = normalize(node.title)
      const normalizedPath = normalize(node.path)
      const normalizedBreadcrumb = normalize(breadcrumb)

      items.push({
        id: node.id,
        title: node.title,
        path: node.path,
        kind: node.kind,
        breadcrumb,
        searchText: normalize(
          [node.title, node.path, breadcrumbParts.join(' ')].join(' '),
        ),
        normalizedTitle,
        normalizedPath,
        normalizedBreadcrumb,
      })
    }

    for (const child of node.children || []) {
      walk(child, breadcrumbParts)
    }
  }

  for (const child of root.children || []) {
    walk(child, [])
  }

  return items
}

function scoreItem(
  item: FlatPageSearchItem,
  query: string,
  options: SearchScoringOptions,
) {
  if (!query) return 0

  const title = item.normalizedTitle
  const path = item.normalizedPath
  const breadcrumb = item.normalizedBreadcrumb

  if (title === query) return 1000
  if (path === query) return 980
  if (title.startsWith(query)) return 900
  if (
    typeof options.pathStartsWithScore === 'number' &&
    path.startsWith(query)
  ) {
    return options.pathStartsWithScore
  }
  if (breadcrumb.startsWith(query)) return 700
  if (title.includes(query)) return 600
  if (path.includes(query)) return 500
  if (breadcrumb.includes(query)) return 400

  const queryParts = query.split(/\s+/).filter(Boolean)
  if (
    queryParts.length > 1 &&
    queryParts.every((part) => item.searchText.includes(part))
  ) {
    return 380
  }

  return -1
}

export function searchFlatPageSearchItems(
  items: FlatPageSearchItem[],
  query: string,
  limit = 20,
  options: SearchScoringOptions = {},
) {
  if (limit <= 0) {
    return []
  }

  const normalizedQuery = normalize(query)

  if (!normalizedQuery) {
    return items.slice(0, limit)
  }

  const results: ScoredSearchItem[] = []

  for (const item of items) {
    const score = scoreItem(item, normalizedQuery, options)
    if (score < 0) continue

    pushTopResult(results, { item, score }, limit)
  }

  return results.map((entry) => entry.item)
}
