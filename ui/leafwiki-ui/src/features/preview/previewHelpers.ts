import { PDF_EXTENSIONS } from '@/lib/config'
import { Children, ReactElement, ReactNode, isValidElement } from 'react'
import { defaultUrlTransform, type ExtraProps } from 'react-markdown'

// Helpers used by MarkdownPreview's renderers.

export const CLOBBER_PREFIX = ''
export const FOOTNOTE_TARGET_PREFIX = '#user-content-fn'
export const WIKILINK_PROTOCOLS = [
  'wikilink-notfound:',
  'wikilink-ambiguous:',
] as const

export type MarkdownNodeProp = ExtraProps

export type SemanticAlertKind = 'info' | 'success' | 'warning' | 'error'

export type ShoutoutConfig = {
  kind: string
}

function getTextContent(node: ReactNode): string {
  if (typeof node === 'string' || typeof node === 'number') {
    return String(node)
  }

  if (Array.isArray(node)) {
    return node.map(getTextContent).join('')
  }

  if (isValidElement<{ children?: ReactNode }>(node)) {
    return getTextContent(node.props.children)
  }

  return ''
}

export function getShoutoutConfig(children: ReactNode): ShoutoutConfig | null {
  const childArray = Children.toArray(children)
  const firstChild = childArray.find(
    (child) => typeof child !== 'string' || child.trim() !== '',
  )

  if (!isValidElement<{ children?: ReactNode }>(firstChild)) {
    return null
  }

  if (firstChild.type !== 'p') {
    return null
  }

  const marker = getTextContent(firstChild.props.children).trim()
  const match = marker.match(/^\[!(?<kind>[A-Z][A-Z0-9_-]*)\]$/)
  if (!match?.groups?.kind) {
    return null
  }

  const kind = match.groups.kind.toLowerCase()

  return { kind }
}

function getAlertLabel(kind: SemanticAlertKind) {
  if (kind === 'info') return 'Info'
  if (kind === 'success') return 'Success'
  if (kind === 'warning') return 'Warning'
  return 'Error'
}

export function getSemanticShoutoutTitle(kind: string) {
  if (
    kind === 'info' ||
    kind === 'success' ||
    kind === 'warning' ||
    kind === 'error'
  ) {
    return getAlertLabel(kind)
  }

  return null
}

export function isPdfSrc(src?: string) {
  if (!src) return false
  const withoutQuery = src.split(/[?#]/)[0]
  const ext = withoutQuery.split('.').pop()?.toLowerCase()
  return !!ext && PDF_EXTENSIONS.includes(ext)
}

export function normalizeFootnoteHref(href?: string) {
  if (!href?.startsWith(FOOTNOTE_TARGET_PREFIX)) {
    return href
  }

  return `#${CLOBBER_PREFIX}${href.slice(1)}`
}

export function transformMarkdownUrl(url: string) {
  if (WIKILINK_PROTOCOLS.some((protocol) => url.startsWith(protocol))) {
    return url
  }

  return defaultUrlTransform(url)
}

export function isPlainListParagraph(
  child: ReactNode,
): child is ReactElement<{ children?: ReactNode; 'data-line'?: string }> {
  if (
    !isValidElement<{ children?: ReactNode; 'data-line'?: string }>(child) ||
    child.type !== 'p'
  ) {
    return false
  }

  const propKeys = Object.keys(child.props)
  return propKeys.every((key) => key === 'children' || key === 'data-line')
}

export function findScrollParent(el: HTMLElement | null): HTMLElement | null {
  let node: HTMLElement | null = el?.parentElement ?? null
  while (node) {
    const { overflow, overflowY } = getComputedStyle(node)
    if (/auto|scroll/.test(overflow + overflowY)) return node
    node = node.parentElement
  }
  return null
}
