import { Children, ReactNode, cloneElement, isValidElement } from 'react'

/**
 * Split highlighted React children into lines, preserving hljs spans.
 */
export function splitHighlightedLines(children: ReactNode): ReactNode[][] {
  const lines: ReactNode[][] = [[]]

  const appendToCurrent = (node: ReactNode) => {
    lines[lines.length - 1].push(node)
  }

  const walk = (node: ReactNode): void => {
    if (node == null || typeof node === 'boolean') {
      return
    }

    if (typeof node === 'string' || typeof node === 'number') {
      const parts = String(node).split('\n')
      parts.forEach((part, index) => {
        if (index > 0) {
          lines.push([])
        }
        if (part !== '') {
          appendToCurrent(part)
        }
      })
      return
    }

    if (Array.isArray(node) || Children.count(node) > 1) {
      Children.forEach(node, walk)
      return
    }

    if (isValidElement<{ children?: ReactNode }>(node)) {
      const child = node.props.children
      if (
        (typeof child === 'string' || typeof child === 'number') &&
        String(child).includes('\n')
      ) {
        const parts = String(child).split('\n')
        parts.forEach((part, index) => {
          if (index > 0) {
            lines.push([])
          }
          if (part !== '') {
            appendToCurrent(
              cloneElement(node, { key: `ln-${lines.length}-${index}` }, part),
            )
          }
        })
        return
      }

      appendToCurrent(node)
      return
    }

    appendToCurrent(node)
  }

  walk(children)

  // Code fences usually end with a trailing newline; drop the empty last line.
  if (lines.length > 1 && lines[lines.length - 1].length === 0) {
    lines.pop()
  }

  return lines
}
