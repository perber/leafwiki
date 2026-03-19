import { PageNode } from '@/lib/api/pages'

export type FlatPageSearchItem = {
  id: string
  title: string
  path: string
  kind: PageNode['kind']
  breadcrumb: string
  searchText: string
}

type SearchScoringOptions = {
  pathStartsWithScore?: number
}

function normalize(value: string) {
  return value.toLowerCase().trim()
}

export function buildFlatPageSearchItems(
  root: PageNode | null,
): FlatPageSearchItem[] {
  if (!root) return []

  const items: FlatPageSearchItem[] = []

  const walk = (node: PageNode, parents: string[]) => {
    const breadcrumbParts = [...parents, node.title]

    if (node.path) {
      items.push({
        id: node.id,
        title: node.title,
        path: node.path,
        kind: node.kind,
        breadcrumb: breadcrumbParts.join(' / '),
        searchText: normalize(
          [node.title, node.path, breadcrumbParts.join(' ')].join(' '),
        ),
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

  const title = normalize(item.title)
  const path = normalize(item.path)
  const breadcrumb = normalize(item.breadcrumb)

  if (title === query) return 1000
  if (path === query) return 980
  if (title.startsWith(query)) return 900
  if (options.pathStartsWithScore && path.startsWith(query)) {
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
    return 300
  }

  return -1
}

export function searchFlatPageSearchItems(
  items: FlatPageSearchItem[],
  query: string,
  limit = 20,
  options: SearchScoringOptions = {},
) {
  const normalizedQuery = normalize(query)

  if (!normalizedQuery) {
    return items
      .slice()
      .sort((a, b) => a.title.localeCompare(b.title))
      .slice(0, limit)
  }

  return items
    .map((item) => ({
      item,
      score: scoreItem(item, normalizedQuery, options),
    }))
    .filter((entry) => entry.score >= 0)
    .sort((a, b) => {
      if (b.score !== a.score) return b.score - a.score
      return a.item.title.localeCompare(b.item.title)
    })
    .slice(0, limit)
    .map((entry) => entry.item)
}
