import type { Element, Root } from 'hast'
import type { Plugin } from 'unified'
import { visit } from 'unist-util-visit'

function asClassList(
  className: string | number | boolean | (string | number)[] | null | undefined,
): string[] {
  if (Array.isArray(className)) {
    return className.map(String)
  }
  if (typeof className === 'string') {
    return className.split(/\s+/).filter(Boolean)
  }
  return []
}

/**
 * Opt-in code-block line numbers via a trailing `=` on the fence language,
 * e.g. ```go= or ```=. Strips the marker so highlighting still works and sets
 * data-line-numbers for the preview renderer.
 */
export const rehypeCodeFenceLineNumbers: Plugin<[], Root> = () => {
  return (tree) => {
    visit(tree, 'element', (node: Element) => {
      if (node.tagName !== 'code') return

      const classes = asClassList(node.properties?.className)
      let enabled = false

      const nextClasses = classes.map((cls) => {
        if (!cls.startsWith('language-') || !cls.endsWith('=')) {
          return cls
        }
        enabled = true
        const withoutMarker = cls.slice(0, -1)
        return withoutMarker === 'language-'
          ? 'language-plaintext'
          : withoutMarker
      })

      if (!enabled) return

      node.properties = node.properties || {}
      node.properties.className = nextClasses
      node.properties['data-line-numbers'] = 'true'
    })
  }
}
