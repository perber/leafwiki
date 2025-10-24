import { remarkLineNumber } from '@/lib/remarkLineNumber'
import 'highlight.js/styles/github-dark.css'
import ReactMarkdown from 'react-markdown'
import rehypeHighlight from 'rehype-highlight'
import remarkGfm from 'remark-gfm'
import { MarkdownImage } from './MarkdownImage'
import { MarkdownLink } from './MarkdownLink'

type Props = {
  content: string
}

export default function MarkdownPreview({ content }: Props) {
  return (
    <ReactMarkdown
      remarkPlugins={[remarkGfm, remarkLineNumber]}
      rehypePlugins={[rehypeHighlight]}
      components={{
        a: MarkdownLink,
        img: MarkdownImage,
        code: (props) => {
          const { className, children } = props
          if (className?.includes('language-')) {
            return (
              <code className={className} {...props}>
                {children}
              </code>
            )
          }
          if (children && typeof children === 'string' && children.includes('\n')) {
            return <code>{children}</code>
          } else {
              return <code className="inline-code">{children}</code>
          }         
        },
      }}
    >
      {content}
    </ReactMarkdown>
  )
}
