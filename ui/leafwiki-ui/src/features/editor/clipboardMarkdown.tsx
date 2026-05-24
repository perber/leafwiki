import ReactMarkdown from 'react-markdown'
import { renderToStaticMarkup } from 'react-dom/server'
import remarkGfm from 'remark-gfm'
import TurndownService from 'turndown'
import { normalizeMarkdownListIndentation } from '../preview/normalizeMarkdownListIndentation'

const GOOGLE_DOCS_CLIPBOARD_TYPE =
  'application/x-vnd.google-docs-document-slice-clip+wrapped'

const turndown = new TurndownService({
  bulletListMarker: '-',
  codeBlockStyle: 'fenced',
  emDelimiter: '_',
  headingStyle: 'atx',
  hr: '---',
})

const unorderedListMarkerPattern = /^[\s\u00a0]*[•●◦▪■]\s*/
const orderedListMarkerPattern = /^[\s\u00a0]*(?:\d+|[A-Za-z])[.)]\s*/

function isPortableImageSource(src: string) {
  return !/^(?:data|blob|file):/i.test(src)
}

function parseInlineStyle(styleValue: string | null) {
  const styles = new Map<string, string>()

  for (const declaration of styleValue?.split(';') ?? []) {
    const [property, ...valueParts] = declaration.split(':')
    if (!property || valueParts.length === 0) continue

    styles.set(property.trim().toLowerCase(), valueParts.join(':').trim())
  }

  return styles
}

function isBoldFontWeight(value: string | undefined) {
  if (!value) return false
  if (value === 'bold' || value === 'bolder') return true

  const numeric = Number.parseInt(value, 10)
  return Number.isFinite(numeric) && numeric >= 600
}

function isMonospaceFontFamily(value: string | undefined) {
  if (!value) return false

  return /(monospace|menlo|consolas|monaco|courier|source code|fira code)/i.test(
    value,
  )
}

function unwrapElement(element: Element) {
  const parent = element.parentNode
  if (!parent) return

  const fragment = element.ownerDocument.createDocumentFragment()
  while (element.firstChild) {
    fragment.appendChild(element.firstChild)
  }

  parent.replaceChild(fragment, element)
}

function unwrapGoogleDocsWrappers(root: HTMLElement) {
  const wrappers = Array.from(
    root.querySelectorAll('[id^="docs-internal-guid"]'),
  )

  for (const wrapper of wrappers.reverse()) {
    unwrapElement(wrapper)
  }

  const inlineWrappersWithBlockChildren = Array.from(
    root.querySelectorAll('b, strong, i, em'),
  ).filter((element) =>
    Array.from(element.children).some((child) =>
      /^(P|DIV|UL|OL|LI|TABLE|BLOCKQUOTE|H[1-6])$/.test(child.tagName),
    ),
  )

  for (const wrapper of inlineWrappersWithBlockChildren.reverse()) {
    unwrapElement(wrapper)
  }
}

function stripLeafWikiHeadingAnchors(root: HTMLElement) {
  const headingLinks = Array.from(
    root.querySelectorAll('h1.anchor, h2.anchor, h3.anchor, h4.anchor, h5.anchor, h6.anchor'),
  )

  for (const heading of headingLinks) {
    const fullAnchor = heading.querySelector(
      ':scope > a.headline-anchor--full',
    ) as HTMLAnchorElement | null

    if (fullAnchor) {
      const fragment = heading.ownerDocument.createDocumentFragment()

      for (const child of Array.from(fullAnchor.childNodes)) {
        if (
          child instanceof HTMLElement &&
          child.tagName === 'SPAN' &&
          child.querySelector('svg')
        ) {
          continue
        }

        fragment.appendChild(child)
      }

      heading.replaceChildren(fragment)
    }

    heading
      .querySelectorAll(':scope > a.headline-anchor:not(.headline-anchor--full)')
      .forEach((anchor) => anchor.remove())
  }
}

function stripLeafWikiCodeBlockChrome(root: HTMLElement) {
  root
    .querySelectorAll('.markdown-code-block__actions')
    .forEach((actions) => actions.remove())
}

function normalizeInlineFormatting(root: HTMLElement) {
  const candidates = Array.from(root.querySelectorAll('span, font, b, i'))

  for (const element of candidates.reverse()) {
    const styles = parseInlineStyle(element.getAttribute('style'))
    const tagName = element.tagName.toLowerCase()

    const wantsStrong =
      tagName === 'strong' ||
      (tagName === 'b' && styles.get('font-weight') !== 'normal') ||
      isBoldFontWeight(styles.get('font-weight'))
    const wantsEm =
      tagName === 'em' ||
      (tagName === 'i' && styles.get('font-style') !== 'normal') ||
      styles.get('font-style') === 'italic'
    const wantsDel =
      tagName === 'del' ||
      tagName === 'strike' ||
      tagName === 's' ||
      styles.get('text-decoration')?.includes('line-through') === true
    const wantsCode =
      tagName === 'code' || isMonospaceFontFamily(styles.get('font-family'))

    if (!wantsStrong && !wantsEm && !wantsDel && !wantsCode) {
      unwrapElement(element)
      continue
    }

    const fragment = element.ownerDocument.createDocumentFragment()
    let topWrapper: HTMLElement | null = null
    let currentWrapper: HTMLElement | null = null

    for (const wrapperTag of [
      wantsStrong ? 'strong' : null,
      wantsEm ? 'em' : null,
      wantsDel ? 'del' : null,
      wantsCode ? 'code' : null,
    ]) {
      if (!wrapperTag) continue

      const wrapper = element.ownerDocument.createElement(wrapperTag)
      if (!topWrapper) {
        topWrapper = wrapper
      }
      if (currentWrapper) {
        currentWrapper.appendChild(wrapper)
      }
      currentWrapper = wrapper
    }

    while (element.firstChild) {
      ;(currentWrapper ?? fragment).appendChild(element.firstChild)
    }

    if (topWrapper) {
      fragment.appendChild(topWrapper)
    }

    element.parentNode?.replaceChild(fragment, element)
  }
}

function getListMarkerInfo(element: Element) {
  if (element.tagName !== 'P' && element.tagName !== 'DIV') {
    return null
  }

  const text = element.textContent?.replace(/\u00a0/g, ' ') ?? ''
  if (unorderedListMarkerPattern.test(text)) {
    return { type: 'ul' as const }
  }
  if (orderedListMarkerPattern.test(text)) {
    return { type: 'ol' as const }
  }

  return null
}

function stripLeadingListMarker(element: Element) {
  const walker = element.ownerDocument.createTreeWalker(
    element,
    NodeFilter.SHOW_TEXT,
  )

  let removeWhitespaceOnly = false
  let currentNode = walker.nextNode()

  while (currentNode) {
    const textNode = currentNode as Text
    const original = textNode.textContent ?? ''
    const normalized = original.replace(/\u00a0/g, ' ')

    if (!removeWhitespaceOnly) {
      const withoutUnordered = normalized.replace(
        unorderedListMarkerPattern,
        '',
      )
      if (withoutUnordered !== normalized) {
        textNode.textContent = withoutUnordered
        return
      }

      const withoutOrdered = normalized.replace(orderedListMarkerPattern, '')
      if (withoutOrdered !== normalized) {
        textNode.textContent = withoutOrdered
        return
      }

      if (
        /^[\s\u00a0]*[•●◦▪■][\s\u00a0]*$/.test(normalized) ||
        /^[\s\u00a0]*(?:\d+|[A-Za-z])[.)][\s\u00a0]*$/.test(normalized)
      ) {
        textNode.textContent = ''
        removeWhitespaceOnly = true
      }
    } else if (/\S/.test(normalized)) {
      textNode.textContent = normalized.replace(/^[\s\u00a0]+/, '')
      return
    }

    currentNode = walker.nextNode()
  }
}

function convertGoogleDocsParagraphLists(root: HTMLElement) {
  const processContainer = (container: HTMLElement) => {
    const blocks = Array.from(container.children)

    for (let index = 0; index < blocks.length; index++) {
      const first = blocks[index]
      const firstInfo = getListMarkerInfo(first)

      if (!firstInfo) {
        processContainer(first as HTMLElement)
        continue
      }

      const list = root.ownerDocument.createElement(firstInfo.type)
      first.before(list)

      let cursor = index
      while (cursor < blocks.length) {
        const candidate = blocks[cursor]
        const info = getListMarkerInfo(candidate)
        if (!info || info.type !== firstInfo.type) break

        const listItem = root.ownerDocument.createElement('li')
        const clone = candidate.cloneNode(true) as Element
        stripLeadingListMarker(clone)

        while (clone.firstChild) {
          listItem.appendChild(clone.firstChild)
        }

        list.appendChild(listItem)
        candidate.remove()
        cursor += 1
      }

      index = cursor - 1
    }
  }

  processContainer(root)
}

function normalizeGoogleDocsHtml(root: HTMLElement) {
  unwrapGoogleDocsWrappers(root)
  stripLeafWikiHeadingAnchors(root)
  stripLeafWikiCodeBlockChrome(root)
  normalizeInlineFormatting(root)
  convertGoogleDocsParagraphLists(root)
}

function escapeTableCell(value: string) {
  return value
    .replace(/\r\n/g, '\n')
    .replace(/\n+/g, '<br>')
    .replace(/\|/g, '\\|')
    .trim()
}

function readTableCell(cell: Element) {
  return escapeTableCell(cell.textContent ?? '')
}

turndown.addRule('removeOfficeAnchors', {
  filter: (node) =>
    node.nodeName === 'A' &&
    (node as HTMLAnchorElement).getAttribute('href')?.startsWith('#_') === true,
  replacement: (content) => content,
})

turndown.addRule('preserveImageAlt', {
  filter: 'img',
  replacement: (_, node) => {
    const element = node as HTMLImageElement
    const src = element.getAttribute('src')?.trim()
    if (!src) return ''
    if (!isPortableImageSource(src)) return ''

    const alt = (element.getAttribute('alt') ?? '').replace(/\n+/g, ' ').trim()
    const title = (element.getAttribute('title') ?? '')
      .replace(/\n+/g, ' ')
      .trim()

    return title ? `![${alt}](${src} "${title}")` : `![${alt}](${src})`
  },
})

turndown.addRule('markdownTables', {
  filter: 'table',
  replacement: (_, node) => {
    const table = node as HTMLTableElement
    const rows = Array.from(table.querySelectorAll('tr')).map((row) =>
      Array.from(row.querySelectorAll('th, td')).map(readTableCell),
    )

    const nonEmptyRows = rows.filter((row) =>
      row.some((cell) => cell.length > 0),
    )
    if (nonEmptyRows.length === 0) {
      return '\n\n'
    }

    const columnCount = Math.max(...nonEmptyRows.map((row) => row.length))
    const normalizedRows = nonEmptyRows.map((row) =>
      Array.from({ length: columnCount }, (_, index) => row[index] ?? ''),
    )

    const hasHeader = table.querySelector('th') !== null
    const header = hasHeader
      ? normalizedRows[0]
      : normalizedRows[0].map((_, index) => `Column ${index + 1}`)
    const bodyRows = hasHeader ? normalizedRows.slice(1) : normalizedRows
    const separator = header.map(() => '---')

    const markdownRows = [header, separator, ...bodyRows].map(
      (row) => `| ${row.join(' | ')} |`,
    )

    return `\n\n${markdownRows.join('\n')}\n\n`
  },
})

function normalizeMarkdownClipboardContent(markdown: string) {
  return normalizeMarkdownListIndentation(markdown)
    .replace(/\u00a0/g, ' ')
    .replace(/\r\n/g, '\n')
    .replace(/\n{3,}/g, '\n\n')
    .trim()
}

export function clipboardHtmlToMarkdown(html: string) {
  const parsed = new DOMParser().parseFromString(html, 'text/html')

  parsed.querySelectorAll('style, script, meta, link').forEach((node) => {
    node.remove()
  })

  normalizeGoogleDocsHtml(parsed.body)

  return normalizeMarkdownClipboardContent(turndown.turndown(parsed.body))
}

function findBestHtmlString(value: unknown): string {
  if (typeof value === 'string') {
    return /<(?:p|div|span|b|strong|i|em|ul|ol|li|h[1-6]|table|img)\b/i.test(
      value,
    )
      ? value
      : ''
  }

  if (!value || typeof value !== 'object') {
    return ''
  }

  if (Array.isArray(value)) {
    for (const item of value) {
      const found = findBestHtmlString(item)
      if (found) return found
    }
    return ''
  }

  for (const nested of Object.values(value)) {
    const found = findBestHtmlString(nested)
    if (found) return found
  }

  return ''
}

function findBestPlainString(value: unknown): string {
  if (typeof value === 'string') {
    return value.trim()
  }

  if (!value || typeof value !== 'object') {
    return ''
  }

  if (Array.isArray(value)) {
    for (const item of value) {
      const found = findBestPlainString(item)
      if (found) return found
    }
    return ''
  }

  for (const nested of Object.values(value)) {
    const found = findBestPlainString(nested)
    if (found) return found
  }

  return ''
}

export function googleDocsClipboardToMarkdown(raw: string) {
  if (!raw.trim()) return ''

  try {
    const parsed = JSON.parse(raw) as unknown
    const html = findBestHtmlString(parsed)
    if (html) {
      return clipboardHtmlToMarkdown(html)
    }

    return normalizeMarkdownClipboardContent(findBestPlainString(parsed))
  } catch {
    return ''
  }
}

export { GOOGLE_DOCS_CLIPBOARD_TYPE }

export function markdownToClipboardHtml(markdown: string) {
  const normalized = normalizeMarkdownClipboardContent(markdown)
  if (!normalized) return ''

  return renderToStaticMarkup(
    <ReactMarkdown remarkPlugins={[remarkGfm]}>{normalized}</ReactMarkdown>,
  )
}
