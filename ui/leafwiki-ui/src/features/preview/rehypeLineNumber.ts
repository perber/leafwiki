import { Element } from 'hast'
import { Plugin } from 'unified'
import { visit } from 'unist-util-visit'

export const rehypeLineNumber: Plugin = () => {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  return (tree: any) => {
    visit(tree, 'element', (node: Element) => {
      const line = node.position?.start?.line
      if (!line) return

      node.properties = node.properties || {}

      // Only set data-line if it is not already set
      if (!('data-line' in node.properties)) {
        node.properties['data-line'] = String(line)
      }
    })
  }
}
