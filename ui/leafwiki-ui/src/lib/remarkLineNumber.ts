import { Plugin } from 'unified'
import { visit } from 'unist-util-visit'

export const remarkLineNumber: Plugin = () => {
  return (tree: any) => {
    visit(tree, (node: any) => {
      if (node.position && node.position.start) {
        node.data = {
          ...node.data,
          hProperties: {
            ...(node.data?.hProperties || {}),
            'data-line': node.position.start.line,
          },
        }
      }
    })
  }
}
