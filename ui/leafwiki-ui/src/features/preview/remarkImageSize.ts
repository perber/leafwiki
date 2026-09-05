import type { Image, Parent, Root, Text } from 'mdast'
import { visit } from 'unist-util-visit'

const IMAGE_WIDTH_PATTERN = /^\s*\{width=([^}]*)\}/
const IMAGE_WIDTH_VALUE_PATTERN = /^(\d+(?:\.\d+)?)%$/

function consumeTextNode(
  parent: Parent,
  index: number,
  textNode: Text,
  length: number,
) {
  textNode.value = textNode.value.slice(length)

  if (textNode.value.length === 0) {
    parent.children.splice(index, 1)
  }
}

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
      const match = textNode.value.match(IMAGE_WIDTH_PATTERN)

      if (!match) {
        return
      }

      consumeTextNode(parent, index + 1, textNode, match[0].length)

      const widthMatch = match[1].match(IMAGE_WIDTH_VALUE_PATTERN)

      if (!widthMatch) {
        return
      }

      const width = Number(widthMatch[1])

      if (!Number.isFinite(width) || width <= 0 || width > 100) {
        return
      }

      node.data ??= {}
      node.data.hProperties = {
        ...(node.data.hProperties ?? {}),
        width: `${width}%`,
      }
    })
  }
}
