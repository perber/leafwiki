import { EditorView } from '@codemirror/view'

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
