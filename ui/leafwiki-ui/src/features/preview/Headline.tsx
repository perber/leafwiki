import { Paperclip } from 'lucide-react'
import { ReactNode, useMemo } from 'react'
import { JSX } from 'react/jsx-runtime'
import { useHeadlineId } from './useHeadlineId'

// Utility function to generate slugs from headline text
const slugify = (text: string) => {
  // replace special characters like ä, ö, ü, ß etc. with ae, oe, ue, ss
  const specialChars = {
    ö: 'o',
    ü: 'u',
    ß: 's',
    ä: 'a',
  }

  return text
    .toLowerCase()
    .replace(
      /[öüßä]/g,
      (char) => specialChars[char as keyof typeof specialChars],
    )
    .trim()
    .replace(/[^\w\s-]/g, '')
    .replace(/[\s_-]+/g, '-')
    .replace(/^-+|-+$/g, '')
}

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
  const { getUniqueId } = useHeadlineId()

  // Use useMemo to ensure we only generate the unique ID once per render
  const slug = useMemo(() => {
    const baseSlug = slugify(text)
    return getUniqueId(baseSlug)
  }, [text, getUniqueId])

  return (
    <Tag id={slug} className="anchor" data-line={dataLine}>
      <a className="no-underline hover:underline" href={`#${slug}`}>
        {children}
        <span className="absolute top-1/2 -left-5 -translate-y-1/2 text-gray-600">
          <Paperclip size={18} />
        </span>
      </a>
    </Tag>
  )
}
