import type { Image, Parent, Root, Text } from 'mdast'
import { visit } from 'unist-util-visit'

const IMAGE_SIZE_PATTERN = /^\s*\{size=(\d+(?:\.\d+)?)%\}/

export function remarkImageSize() {
  return (tree: Root) => {
    visit(tree, 'image', (node: Image, index, parent: Parent | undefined) => {
      if (index === undefined || !parent) {
        return
      }

      const nextNode = parent.children[index + 1]

      if (!nextNode || nextNode.type !== 'text') {
        return
      }

      const textNode = nextNode as Text
      const match = textNode.value.match(IMAGE_SIZE_PATTERN)

      if (!match) {
        return
      }

      const size = Number(match[1])

      if (!Number.isFinite(size) || size <= 0 || size > 100) {
        return
      }

      node.data ??= {}
      node.data.hProperties = {
        ...(node.data.hProperties ?? {}),
        width: `${size}%`,
      }

      textNode.value = textNode.value.slice(match[0].length)

      if (textNode.value.length === 0) {
        parent.children.splice(index + 1, 1)
      }
    })
  }
}
