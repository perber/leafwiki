import type {
  Completion,
  CompletionContext,
  CompletionResult,
} from '@codemirror/autocomplete'
import { FlatPageSearchItem, searchFlatPageSearchItems } from '@/lib/pageSearch'
import { useTreeStore } from '@/stores/tree'

const MAX_RESULTS = 20
const SUPPRESSED_EXTERNAL_PREFIXES = ['http', 'https', 'mailto']

export type InternalLinkCompletion = Completion & {
  path: string
}

function hasSuppressedExternalPrefix(value: string) {
  const normalizedValue = value.trimStart().toLowerCase()

  return SUPPRESSED_EXTERNAL_PREFIXES.some((prefix) => {
    if (normalizedValue === prefix) return true
    return normalizedValue.startsWith(`${prefix}:`)
  })
}

function getLinkTargetRange(context: CompletionContext) {
  const { state, pos } = context
  const line = state.doc.lineAt(pos)
  const beforeCursor = line.text.slice(0, pos - line.from)
  const afterCursor = line.text.slice(pos - line.from)
  const match = beforeCursor.match(/!?\[[^\]]*\]\(([^)\s]*)$/)

  if (!match) return null

  const typedTarget = match[1] ?? ''
  const suffix = afterCursor.match(/^[^)\s]*/)?.[0] ?? ''
  const fullTarget = `${typedTarget}${suffix}`

  if (hasSuppressedExternalPrefix(fullTarget)) {
    return null
  }

  return {
    from: pos - typedTarget.length,
    to: pos + suffix.length,
    query: typedTarget,
  }
}

function buildCompletionOptions(
  items: FlatPageSearchItem[],
): InternalLinkCompletion[] {
  return items.map((item) => ({
    label: item.title,
    displayLabel: item.title,
    info: item.breadcrumb,
    type: 'text',
    apply: `/${item.path}`,
    section: item.kind === 'section' ? 'Sections' : 'Pages',
    path: item.path,
  }))
}

export function internalLinkCompletionSource(
  context: CompletionContext,
): CompletionResult | null {
  const range = getLinkTargetRange(context)
  if (!range) return null

  const items = useTreeStore.getState().flatPages
  if (items.length === 0) return null

  const matches = searchFlatPageSearchItems(items, range.query, MAX_RESULTS, {
    pathStartsWithScore: 820,
  })
  if (matches.length === 0) return null

  return {
    from: range.from,
    to: range.to,
    options: buildCompletionOptions(matches),
    validFor: /^[^)\s]*$/,
  }
}
