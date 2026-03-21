import { PageNode } from '@/lib/api/pages'

const WORD_BOUNDARY_CHARS = new Set(['/', '-', '_', ' '])
const TITLE_FUZZY_BASE_SCORE = 340
const PATH_FUZZY_BASE_SCORE = 320
const BREADCRUMB_FUZZY_BASE_SCORE = 280

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

function scoreSubsequenceMatch(value: string, query: string) {
  if (query.length < 2) return -1

  let queryIndex = 0
  let firstMatchIndex = -1
  let previousMatchIndex = -1
  let gapPenalty = 0
  let consecutiveBonus = 0
  let wordBoundaryBonus = 0

  for (let i = 0; i < value.length && queryIndex < query.length; i += 1) {
    if (value[i] !== query[queryIndex]) continue

    if (firstMatchIndex === -1) {
      firstMatchIndex = i
    }

    if (previousMatchIndex >= 0) {
      if (i === previousMatchIndex + 1) {
        consecutiveBonus += 10
      } else {
        gapPenalty += (i - previousMatchIndex - 1) * 3
      }
    }

    if (i === 0 || WORD_BOUNDARY_CHARS.has(value[i - 1])) {
      wordBoundaryBonus += 8
    }

    previousMatchIndex = i
    queryIndex += 1
  }

  if (queryIndex !== query.length || firstMatchIndex < 0) {
    return -1
  }

  return (
    query.length * 12 +
    consecutiveBonus +
    wordBoundaryBonus -
    gapPenalty -
    firstMatchIndex * 2
  )
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

function scoreFuzzyField(value: string, query: string, baseScore: number) {
  const score = scoreSubsequenceMatch(value, query)
  if (score < 0) return -1

  return baseScore + score
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

  return Math.max(
    scoreFuzzyField(title, query, TITLE_FUZZY_BASE_SCORE),
    scoreFuzzyField(path, query, PATH_FUZZY_BASE_SCORE),
    scoreFuzzyField(breadcrumb, query, BREADCRUMB_FUZZY_BASE_SCORE),
  )
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
