import { Paperclip } from 'lucide-react'
import { ReactNode, useEffect } from 'react'
import { JSX } from 'react/jsx-runtime'
import { useHeadlinesStore } from './headlines'

function getText(node: ReactNode): string {
  if (typeof node === 'string' || typeof node === 'number') {
    return String(node)
  }
  if (Array.isArray(node)) {
    return node.map(getText).join('')
  }
  if (node && typeof node === 'object' && 'props' in node) {
    // @ts-expect-error -- props exist
    return getText(node.props.children)
  }
  return ''
}

export type HeadlineProps = {
  level: number
  children: ReactNode
  'data-line'?: string
}

export default function Headline({
  level,
  children,
  'data-line': dataLine,
}: HeadlineProps) {
  const text = getText(children)
  const Tag = `h${level}` as keyof JSX.IntrinsicElements

  const registerHeadline = useHeadlinesStore((s) => s.registerHeadline)
  const unregisterHeadline = useHeadlinesStore((s) => s.unregisterHeadline)

  // Register headline synchronously before reading slug
  registerHeadline(level, text, dataLine ? dataLine : '')

  const slug =
    useHeadlinesStore((s) =>
      s.getSlug(level, text, dataLine ? dataLine : ''),
    ) || ''

  useEffect(() => {
    return () => {
      unregisterHeadline(level, text, dataLine ? dataLine : '')
    }
  }, [level, text, dataLine, unregisterHeadline])

  return (
    <Tag id={slug} className="anchor" data-line={dataLine}>
      <a className="" href={`#${slug}`}>
        {children}
        <span>
          <Paperclip size={18} />
        </span>
      </a>
    </Tag>
  )
}
