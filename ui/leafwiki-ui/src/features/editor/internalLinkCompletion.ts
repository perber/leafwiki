import type {
  Completion,
  CompletionContext,
  CompletionResult,
} from '@codemirror/autocomplete'
import { pickedCompletion } from '@codemirror/autocomplete'
import type { EditorState } from '@codemirror/state'
import type { EditorView } from '@codemirror/view'
import { FlatPageSearchItem, searchFlatPageSearchItems } from '@/lib/pageSearch'
import { useTreeStore } from '@/stores/tree'

const MAX_RESULTS = 20
const SUPPRESSED_EXTERNAL_PREFIXES = ['http', 'https', 'mailto']

export type InternalLinkCompletion = Completion & {
  path: string
}

export function hasSuppressedExternalPrefix(value: string) {
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

function getWikiLinkRangeForPosition(state: EditorState, pos: number) {
  const line = state.doc.lineAt(pos)
  const beforeCursor = line.text.slice(0, pos - line.from)
  const afterCursor = line.text.slice(pos - line.from)
  const match = beforeCursor.match(/\[\[([^\]\n]*)$/)

  if (!match) return null

  const typedQuery = match[1] ?? ''
  // Only replace text to the right when we're already inside a wikilink that
  // closes later on the same line, and stop before any new link syntax starts.
  const suffix = afterCursor.match(/^[^[\]\n]*\]{1,2}/)?.[0] ?? ''

  return {
    from: pos - typedQuery.length,
    to: pos + suffix.length,
    query: typedQuery,
  }
}

function getWikiLinkRange(context: CompletionContext) {
  return getWikiLinkRangeForPosition(context.state, context.pos)
}

export function buildMarkdownLinkOptions(
  items: FlatPageSearchItem[],
): InternalLinkCompletion[] {
  return items.map((item) => ({
    label: item.title,
    displayLabel: item.title,
    info: item.breadcrumb,
    type: 'text',
    apply: `/${item.path}`,
    path: item.path,
  }))
}

export function buildWikiLinkOptions(
  items: FlatPageSearchItem[],
): InternalLinkCompletion[] {
  return items.map((item) => ({
    label: item.title,
    displayLabel: item.title,
    info: item.breadcrumb,
    type: 'text',
    apply: (view: EditorView, completion, from: number, to: number) => {
      const cursorPos = view.state.selection.main.head
      const range = getWikiLinkRangeForPosition(view.state, cursorPos)
      const safeFrom = range?.from ?? from
      const safeTo = range?.to ?? to
      const insert = `${item.title}]]`

      view.dispatch({
        changes: { from: safeFrom, to: safeTo, insert },
        selection: { anchor: safeFrom + insert.length },
        annotations: pickedCompletion.of(completion),
      })
    },
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

  const query =
    range.query.startsWith('/') && range.query.length > 1
      ? range.query.slice(1)
      : range.query

  const matches = searchFlatPageSearchItems(items, query, MAX_RESULTS, {
    pathStartsWithScore: 820,
  })
  if (matches.length === 0) return null

  return {
    from: range.from,
    to: range.to,
    options: buildMarkdownLinkOptions(matches),
    filter: false,
  }
}

export function wikiLinkCompletionSource(
  context: CompletionContext,
): CompletionResult | null {
  const range = getWikiLinkRange(context)
  if (!range) return null

  const items = useTreeStore.getState().flatPages
  if (items.length === 0) return null

  const matches = searchFlatPageSearchItems(items, range.query, MAX_RESULTS)
  if (matches.length === 0) return null

  return {
    from: range.from,
    to: range.to,
    options: buildWikiLinkOptions(matches),
    filter: false,
  }
}
