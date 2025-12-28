import type { Root } from 'hast'
import { visit } from 'unist-util-visit'

const ALLOWED_STYLE_PROPS = new Set([
  'margin',
  'margin-top',
  'margin-bottom',
  'margin-left',
  'margin-right',
  'padding',
  'padding-top',
  'padding-bottom',
  'padding-left',
  'padding-right',
  'text-align',
  'font-weight',
  'font-style',
  'text-decoration',
  'color',
  'background-color',
  'font-size',
  'border',
  'border-width',
  'border-style',
  'border-color',
  'border-radius',
  'width',
  'height',
  'max-width',
  'max-height',
  'min-width',
  'min-height',
  'line-height',
  'letter-spacing',
  'word-spacing',
  'float',
  'clear',
  'display',
  'vertical-align',
])

function sanitizeStyle(style: string): string | undefined {
  const declarations = style
    .split(';')
    .map((d) => d.trim())
    .filter(Boolean)

  const safe: string[] = []

  for (const decl of declarations) {
    const [propRaw, ...rest] = decl.split(':')
    if (!propRaw || rest.length === 0) continue

    const prop = propRaw.trim().toLowerCase()
    const value = rest.join(':').trim()

    // Only keep whitelisted properties
    if (!ALLOWED_STYLE_PROPS.has(prop)) continue

    // Optional extra paranoia: reject obviously sketchy values
    const lowerVal = value.toLowerCase()
    if (lowerVal.includes('url(') || lowerVal.includes('expression(')) continue

    safe.push(`${prop}: ${value}`)
  }

  if (safe.length === 0) return undefined
  return safe.join('; ')
}

export function rehypeWhitelistStyles() {
  return (tree: Root) => {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    visit(tree, 'element', (node: any) => {
      const style = node.properties?.style
      if (!style || typeof style !== 'string') return

      const sanitized = sanitizeStyle(style)
      if (sanitized) {
        node.properties.style = sanitized
      } else {
        delete node.properties.style
      }
    })
  }
}