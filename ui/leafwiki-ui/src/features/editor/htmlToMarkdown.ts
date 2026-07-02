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

  return td
}

const converter = createConverter()

export function htmlToMarkdown(html: string): string {
  if (!html || html.trim() === '') return ''
  return converter.turndown(html).trim()
}
