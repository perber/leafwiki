import { Paperclip } from 'lucide-react'
import clsx from 'clsx'
import { createElement, HTMLAttributes, isValidElement, ReactNode } from 'react'

function containsLink(node: ReactNode): boolean {
  if (node == null || typeof node === 'boolean') return false
  if (typeof node === 'string' || typeof node === 'number') return false
  if (Array.isArray(node)) {
    return node.some(containsLink)
  }
  if (!isValidElement(node)) return false
  const props = (node.props ?? {}) as { href?: string; children?: ReactNode }

  if (typeof node.type === 'string' && node.type === 'a') {
    return true
  }

  if (typeof props.href === 'string') {
    return true
  }

  return containsLink(props.children)
}

export type HeadlineProps = HTMLAttributes<HTMLHeadingElement> & {
  level: number
  children: ReactNode
  'data-line'?: string
  'data-leafwiki-generated-id'?: string
  node?: unknown
}

export default function Headline({
  level,
  children,
  className,
  id,
  'data-line': dataLine,
  'data-leafwiki-generated-id': hasGeneratedId,
  node,
  ...props
}: HeadlineProps) {
  void node
  const tagName = `h${level}` as keyof HTMLElementTagNameMap
  const shouldRenderAnchor = hasGeneratedId === 'true' && !!id
  const hasNestedLink = containsLink(children)
  const sectionLinkLabel = 'Link to section'

  return createElement(
    tagName,
    {
      id,
      className: clsx(className, shouldRenderAnchor && 'anchor'),
      'data-line': dataLine,
      ...props,
    },
    shouldRenderAnchor ? (
      hasNestedLink ? (
        <>
          {children}
          <a
            className="headline-anchor"
            href={`#${id}`}
            aria-label={sectionLinkLabel}
            title={sectionLinkLabel}
          >
            <span>
              <Paperclip size={18} />
            </span>
          </a>
        </>
      ) : (
        <a className="headline-anchor headline-anchor--full" href={`#${id}`}>
          {children}
          <span>
            <Paperclip size={18} />
          </span>
        </a>
      )
    ) : (
      children
    ),
  )
}
