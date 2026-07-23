import { Element, Root } from 'hast'
import { Plugin } from 'unified'
import { visit } from 'unist-util-visit'

export const LINE_NUMBERS_CLASS = 'md-line-numbers'

/**
 * Enables line numbers on fenced code blocks whose language identifier ends
 * with an equal sign, e.g.
 *
 * ```js=
 * const answer = 42
 * ```
 *
 * This follows the Otter Wiki convention adopted for leafwiki in #1264/#1313:
 * append `=` to turn line numbers on, leave it off to keep them off.
 *
 * react-markdown lowers ```js= to `<code class="language-js=">`, which
 * rehype-highlight cannot tokenize (there is no "js=" grammar). This plugin
 * must therefore run BEFORE rehype-highlight: it strips the trailing `=` so the
 * real language is highlighted as usual, and tags the `<code>` element with the
 * {@link LINE_NUMBERS_CLASS} marker so the renderer knows to draw a gutter.
 */
export const rehypeCodeLineNumbers: Plugin<[], Root> = () => {
  return (tree) => {
    visit(tree, 'element', (node: Element) => {
      if (node.tagName !== 'code') return

      const properties = node.properties ?? (node.properties = {})
      const className = properties.className
      const classes = Array.isArray(className)
        ? [...className]
        : typeof className === 'string'
          ? className.split(/\s+/).filter(Boolean)
          : []

      let enabled = false
      const rewritten = classes.map((cls) => {
        if (typeof cls === 'string' && /^language-.*=$/.test(cls)) {
          enabled = true
          return cls.slice(0, -1)
        }
        return cls
      })

      if (!enabled) return

      if (!rewritten.includes(LINE_NUMBERS_CLASS)) {
        rewritten.push(LINE_NUMBERS_CLASS)
      }
      properties.className = rewritten
    })
  }
}
