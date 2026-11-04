import { indentLess, insertTab } from '@codemirror/commands'
import { indentUnit, syntaxTree } from '@codemirror/language'
import { EditorState, Extension, SelectionRange } from '@codemirror/state'
import { Command, EditorView, KeyBinding } from '@codemirror/view'

// Everything up to the cursor when it sits right behind a list marker: indent
// or blockquote markers, the marker, and an optional task checkbox.
const LIST_PREFIX_RE =
  /^(?<lead>[\s>]*)(?:[-*+]|(?<number>\d+)[.)])\s+(?:\[[ xX]\]\s+)?$/

// The list item a Tab at this cursor can nest under its previous sibling.
// Without a previous sibling there is nothing to nest under: Markdown would read
// the indented line as a code block or as plain text of the item above, so the
// list would silently disappear from the page.
function findNestableListItem(state: EditorState, range: SelectionRange) {
  if (!range.empty) return null

  const line = state.doc.lineAt(range.head)
  const match = LIST_PREFIX_RE.exec(line.text.slice(0, range.head - line.from))
  if (!match?.groups) return null

  const from = line.from + match.groups.lead.length
  const item = syntaxTree(state).resolveInner(from, 1).parent
  if (item?.name !== 'ListItem' || !item.prevSibling) return null

  return { from, number: match.groups.number }
}

// Tab right behind a list marker moves the item one level in, so nothing has to
// walk the cursor back in front of the marker first. Everything else stays with
// `insertTab`, including items that can't be nested.
const indentListItemOrInsertTab: Command = (view) => {
  const { state } = view
  if (state.readOnly) return false

  const items = state.selection.ranges.map((range) =>
    findNestableListItem(state, range),
  )
  if (items.some((item) => item === null)) return insertTab(view)

  const unit = state.facet(indentUnit)
  // Two cursors can sit behind the same marker; that line is indented once.
  const itemsByStart = new Map(items.map((item) => [item!.from, item!]))
  const changes = [...itemsByStart.values()].map(({ from, number }) =>
    // Only "1" may start a nested ordered list, any other number would stay
    // plain text of the item above.
    number
      ? { from, to: from + number.length, insert: `${unit}1` }
      : { from, insert: unit },
  )

  view.dispatch(
    state.update({ changes, scrollIntoView: true, userEvent: 'input.indent' }),
  )
  return true
}

// Tab inserts a single indentation character at the cursor, and only falls back
// to indenting whole lines when the selection actually spans them (that's what
// `insertTab` does). This replaces CodeMirror's `indentWithTab`, which always
// ran `indentMore` — indenting the entire current line even for a collapsed
// cursor, so pressing Tab mid-line shifted the whole line instead of inserting
// at the caret. List items right behind their marker are the one exception.
// Shift-Tab still dedents via `indentLess`.
export const tabIndentKeyBinding: KeyBinding = {
  key: 'Tab',
  run: indentListItemOrInsertTab,
  shift: indentLess,
}

// Use a real tab as the indent unit so `insertTab` (caret), `indentMore`
// (selection) and `indentLess` (Shift-Tab) all stay consistent — without this
// the selection/dedent paths would use CodeMirror's 2-space default while a
// collapsed-caret Tab inserts "\t".
export const tabIndentUnit: Extension = indentUnit.of('\t')

export function insertWrappedText(
  view: EditorView,
  before: string,
  after: string = before,
) {
  const { from, to } = view.state.selection.main
  const selected = view.state.doc.sliceString(from, to)
  const hasSelection = from !== to

  if (hasSelection) {
    // Case 1: The selection itself IS the formatted block (e.g. user selected "**bold**")
    if (
      selected.startsWith(before) &&
      selected.endsWith(after) &&
      selected.length > before.length + after.length
    ) {
      const unwrapped = selected.slice(
        before.length,
        selected.length - after.length,
      )
      view.dispatch({
        changes: { from, to, insert: unwrapped },
        selection: { anchor: from + unwrapped.length },
      })
      view.focus()
      return
    }

    // Case 2: Markers are directly outside the selection (e.g. user selected "bold" with ** around it)
    const docLen = view.state.doc.length
    const extBefore = view.state.doc.sliceString(
      Math.max(0, from - before.length),
      from,
    )
    const extAfter = view.state.doc.sliceString(
      to,
      Math.min(docLen, to + after.length),
    )
    if (extBefore === before && extAfter === after) {
      view.dispatch({
        changes: [
          { from: from - before.length, to: from, insert: '' },
          { from: to, to: to + after.length, insert: '' },
        ],
        selection: { anchor: from - before.length + selected.length },
      })
      view.focus()
      return
    }
  }

  const insertText = hasSelection
    ? `${before}${selected}${after}`
    : `${before}${after}`
  const cursorPos = hasSelection
    ? from + insertText.length
    : from + before.length

  view.dispatch({
    changes: { from, to, insert: insertText },
    selection: { anchor: cursorPos },
  })
  view.focus()
}

function escapeRegExp(value: string) {
  return value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}

// Replaces a filename in markdown link/image targets (`[text](.../old)` -> `[text](.../new)`).
// Also updates the link/alt text itself when it exactly matches the old filename, since
// inserted assets default to using the filename as their alt/link text.
export function replaceFilenameInText(
  docText: string,
  before: string,
  after: string,
) {
  const newFilename = after.startsWith('/') ? after.slice(1) : after
  const regex = new RegExp(
    `(!?\\[)([^\\]]*?)(\\]\\((?:(?!\\]\\().)*?\\/?)\\/${escapeRegExp(before)}(\\))`,
    'g',
  )

  return docText.replace(
    regex,
    (_match, bracket, altText, pathPrefix, closeParen) => {
      const newAltText = altText === before ? newFilename : altText
      return `${bracket}${newAltText}${pathPrefix}/${newFilename}${closeParen}`
    },
  )
}

// Inserts a heading of the specified level (1, 2, or 3) at the current line at the start position
export function insertHeadingAtStart(view: EditorView, level: 1 | 2 | 3) {
  const { from } = view.state.selection.main
  const line = view.state.doc.lineAt(from)
  const lineStart = line.from
  const prefix = '#'.repeat(level) + ' '

  const transaction = view.state.update({
    changes: { from: lineStart, insert: prefix },
    selection: { anchor: from + prefix.length - (from - lineStart) },
    scrollIntoView: true,
  })
  view.dispatch(transaction)
  view.focus()
}
