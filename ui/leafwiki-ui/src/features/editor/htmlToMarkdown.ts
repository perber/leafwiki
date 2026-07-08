import TurndownService from 'turndown'
import { gfm } from 'turndown-plugin-gfm'

function createConverter(): TurndownService {
  const td = new TurndownService({
    headingStyle: 'atx',
    hr: '---',
    bulletListMarker: '-',
    codeBlockStyle: 'fenced',
    fence: '```',
    emDelimiter: '_',
    strongDelimiter: '**',
    linkStyle: 'inlined',
  })

  td.use(gfm)

  td.remove(['style', 'script'])

  // GFM plugin uses single tilde; override to use double tilde (standard GFM)
  td.addRule('strikethrough', {
    filter: (node) => ['DEL', 'S', 'STRIKE'].includes(node.nodeName),
    replacement: (content) => `~~${content}~~`,
  })

  // Turndown default adds 3 spaces after the bullet/number; override to 1 space
  td.addRule('listItem', {
    filter: 'li',
    replacement(content, node, options) {
      content = content
        .replace(/^\n+/, '')
        .replace(/\n+$/, '\n')
        .replace(/\n/gm, '\n    ')
        // Collapse double space after task list checkbox marker ([ ]  → [ ] )
        .replace(/^(\[[ x]\]) {2}/, '$1 ')

      const parent = node.parentNode as Element
      const isOrdered = parent.nodeName === 'OL'
      const prefix = isOrdered
        ? `${Array.from(parent.children).indexOf(node as Element) + 1}. `
        : `${options.bulletListMarker} `

      return `${prefix}${content}${node.nextSibling ? '\n' : ''}`
    },
  })

  // Google Docs renders headings as <ol><li style="list-style-type:none"> to preserve
  // document structure. Strip the ordered-list prefix for these invisible list items.
  td.addRule('listStyleNone', {
    filter: (node) => {
      if (node.nodeName !== 'LI') return false
      return (node as HTMLElement).style?.listStyleType === 'none'
    },
    replacement: (content) => `\n\n${content.trim()}\n\n`,
  })

  // Google Docs wraps all clipboard content in <b style="font-weight:normal;">
  // to preserve structure without actually making text bold. Strip it transparently.
  td.addRule('googleDocsBoldWrapper', {
    filter: (node) => {
      if (node.nodeName !== 'B') return false
      const fw = (node as HTMLElement).style?.fontWeight
      return fw === 'normal' || fw === '400'
    },
    replacement: (content) => content,
  })

  // Google Docs wraps links in a redirect URL: https://www.google.com/url?q=ACTUAL_URL
  // Extract the real destination from the q parameter.
  td.addRule('googleDocsLink', {
    filter: (node) => {
      if (node.nodeName !== 'A') return false
      const href = (node as HTMLElement).getAttribute('href') ?? ''
      return /^https?:\/\/(www\.)?google\.com\/url[?]/.test(href)
    },
    replacement: (content, node) => {
      const href = (node as HTMLElement).getAttribute('href') ?? ''
      try {
        const realUrl = new URL(href).searchParams.get('q') ?? href
        return `[${content}](${realUrl})`
      } catch {
        return `[${content}](${href})`
      }
    },
  })

  // GFM tables require a header row, but turndown-plugin-gfm only recognizes
  // one when its cells are literal <th> elements — otherwise it keeps the
  // whole table as raw, unconverted HTML. Many real paste sources (Google
  // Sheets, plain web tables, Excel-to-HTML) render every cell as <td> with
  // no <th> at all, so their tables would silently survive as raw HTML.
  // Override the table rules to always treat a table's first row as the
  // header, regardless of whether its cells are <th> or <td>.
  td.addRule('table', {
    filter: 'table',
    replacement: (content) => `\n\n${content.replace('\n\n', '\n')}\n\n`,
  })

  td.addRule('tableSection', {
    filter: ['thead', 'tbody', 'tfoot'],
    replacement: (content) => content,
  })

  td.addRule('tableRow', {
    filter: 'tr',
    replacement: (content, node) => {
      const tr = node as HTMLTableRowElement
      let borderCells = ''
      if (isFirstTableRow(tr)) {
        const alignMap: Record<string, string> = {
          left: ':--',
          right: '--:',
          center: ':-:',
        }
        for (const cell of Array.from(tr.cells)) {
          const align = (cell.getAttribute('align') || '').toLowerCase()
          borderCells += tableCell(alignMap[align] || '---', cell)
        }
      }
      return `\n${content}${borderCells ? `\n${borderCells}` : ''}`
    },
  })

  td.addRule('tableCell', {
    filter: ['th', 'td'],
    replacement: (content, node) =>
      tableCell(content, node as HTMLTableCellElement),
  })

  return td
}

function isFirstTableRow(tr: HTMLTableRowElement): boolean {
  const table = tr.closest('table')
  return !!table && table.rows[0] === tr
}

function tableCell(content: string, node: HTMLTableCellElement): string {
  const index = Array.from(node.parentNode?.childNodes ?? []).indexOf(node)
  const prefix = index === 0 ? '| ' : ' '
  return `${prefix}${content} |`
}

const converter = createConverter()

// Turndown treats U+00A0 (nbsp) as significant whitespace and does not collapse
// runs of it the way it collapses regular spaces, so converting nbsp→space
// before running turndown would change consecutive-nbsp output (e.g. two nbsp
// would collapse to one space instead of staying two). So normalize nbsp→space
// on the *converted* markdown instead — but skip fenced/inline code spans, since
// code content must stay byte-for-byte as pasted (nbsp can be meaningful there).
const CODE_SPAN_RE = /(```[\s\S]*?```|`[^`\n]*`)/g

function normalizeNbspOutsideCode(markdown: string): string {
  return markdown
    .split(CODE_SPAN_RE)
    .map((part, i) => (i % 2 === 0 ? part.replace(/\u00A0/g, ' ') : part))
    .join('')
}

export function htmlToMarkdown(html: string): string {
  if (!html || html.trim() === '') return ''
  // Strip Word/Outlook <o:p> tags before DOM parsing; they survive as unknown
  // elements and produce empty noise in the output.
  const cleaned = html.replace(/<\/?o:p[^>]*>/gi, '')
  const converted = converter.turndown(cleaned).trim()
  return normalizeNbspOutsideCode(converted)
}
