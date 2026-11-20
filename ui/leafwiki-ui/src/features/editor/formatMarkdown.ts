// Cleans up Markdown on save so pages pass `rumdl check` for MD009, MD012,
// MD019, MD022 and MD047. Like rumdl, it leaves indented code blocks as they are.
// Fenced code gets only its trailing whitespace stripped.

// A fence may sit behind list markers or blockquote markers (CommonMark containers).
const FENCE_RE = /^(?:[ \t]|>|(?:[-*+]|\d{1,9}[.)])[ \t])*(`{3,}|~{3,})/
const FENCE_CLOSE_RE = /^(?:[ \t]|>)*(`{3,}|~{3,})[ \t]*$/
const HEADING_RE = /^(#{1,6})[ \t]+/
const INDENTED_RE = /^(?: {4}|\t)/
const TRAILING_WS_RE = /[ \t]+$/

function trimTrailing(line: string): string {
  const trimmed = line.replace(TRAILING_WS_RE, '')
  // Two or more trailing spaces are a hard line break; rumdl wants exactly two.
  return / {2,}$/.test(line) ? trimmed + '  ' : trimmed
}

export function formatMarkdown(text: string): string {
  const out: string[] = []
  let blanks: string[] = []
  let fence = ''
  let inIndentedCode = false
  let afterHeading = false

  for (const line of text.replace(/\r\n?/g, '\n').split('\n')) {
    if (fence) {
      out.push(line.replace(TRAILING_WS_RE, ''))
      const close = FENCE_CLOSE_RE.exec(line)
      if (
        close &&
        close[1][0] === fence[0] &&
        close[1].length >= fence.length
      ) {
        fence = ''
      }
      continue
    }

    if (line.trim() === '') {
      blanks.push(line)
      continue
    }

    // shortcut: heuristic, not CommonMark — list continuations may count as code,
    // which only leaves them unformatted.
    const isCode: boolean =
      INDENTED_RE.test(line) &&
      (inIndentedCode || blanks.length > 0 || out.length === 0 || afterHeading)
    if (isCode && inIndentedCode) out.push(...blanks)
    else if (blanks.length > 0 && out.length > 0) out.push('')
    blanks = []
    inIndentedCode = isCode

    const isHeading: boolean = !isCode && HEADING_RE.test(line)
    if ((isHeading || afterHeading) && out.length > 0 && out.at(-1) !== '') {
      out.push('')
    }
    afterHeading = isHeading

    if (isCode) {
      out.push(line)
    } else if (isHeading) {
      out.push(line.replace(HEADING_RE, '$1 ').replace(TRAILING_WS_RE, ''))
    } else {
      const open = FENCE_RE.exec(line)
      if (open) fence = open[1]
      out.push(open ? line.replace(TRAILING_WS_RE, '') : trimTrailing(line))
    }
  }

  return out.length > 0 ? out.join('\n') + '\n' : ''
}
