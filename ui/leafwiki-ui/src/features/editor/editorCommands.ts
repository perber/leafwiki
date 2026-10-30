import { indentLess, indentMore, insertTab } from '@codemirror/commands'
import { indentUnit } from '@codemirror/language'
import { Extension, StateCommand } from '@codemirror/state'
import { EditorView, KeyBinding } from '@codemirror/view'

// Matches leading whitespace + list marker + checkbox (optional)
// e.g. "" || " " && "-" || "*" || "+" || "<#>. " || "<#>) " && "" || "[ ] " || "[x] " || "[X] "
const LIST_PREFIX_RE = /^\s*(?:[-*+]|\d+[.)])\s+(?:\[[ xX]\]\s+)?$/

//Changes indentation rules if operating in a list
//If the line matches, use the "insertMore" function to move entire line,
//otherwise default back to "insertTab".
//Without the check, one would have to move the caret before the bullet,
//which makes list making quite tedious.
export const smartTab: StateCommand = ({ state, dispatch }) => {
  if (
    state.selection.ranges.every((range) => {
      if (!range.empty) return false
      const line = state.doc.lineAt(range.from)
      const textBeforeCursor = line.text.slice(0, range.from - line.from)
      return LIST_PREFIX_RE.test(textBeforeCursor)
    })
  ) {
    return indentMore({ state, dispatch })
  }

  return insertTab({ state, dispatch })
}

export const tabIndentKeyBinding: KeyBinding = {
  key: 'Tab',
  run: smartTab,
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
