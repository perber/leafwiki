import { remarkLineNumber } from '@/features/preview/remarkLineNumber'
import 'highlight.js/styles/github-dark.css'
import { ClassAttributes, HTMLAttributes, useMemo } from 'react'
import ReactMarkdown from 'react-markdown'
import { JSX } from 'react/jsx-runtime'
import rehypeHighlight from 'rehype-highlight'
import remarkGfm from 'remark-gfm'
import Headline from './Headline'
import { HeadlineIdProvider } from './HeadlineIdContext'
import { MarkdownImage } from './MarkdownImage'
import { MarkdownLink } from './MarkdownLink'
import MermaidBlock from './MermaidBlock'

type Props = {
  content: string
}

export default function MarkdownPreview({ content }: Props) {
  const components = useMemo(
    () => ({
      a: MarkdownLink,
      img: MarkdownImage,
      h1: ({
        children,
        ...props
      }: ClassAttributes<HTMLHeadingElement> &
        HTMLAttributes<HTMLHeadingElement>) => (
        <Headline level={1} {...props}>
          {children}
        </Headline>
      ),
      h2: ({
        children,
        ...props
      }: ClassAttributes<HTMLHeadingElement> &
        HTMLAttributes<HTMLHeadingElement>) => (
        <Headline level={2} {...props}>
          {children}
        </Headline>
      ),
      h3: ({
        children,
        ...props
      }: ClassAttributes<HTMLHeadingElement> &
        HTMLAttributes<HTMLHeadingElement>) => (
        <Headline level={3} {...props}>
          {children}
        </Headline>
      ),
      h4: ({
        children,
        ...props
      }: ClassAttributes<HTMLHeadingElement> &
        HTMLAttributes<HTMLHeadingElement>) => (
        <Headline level={4} {...props}>
          {children}
        </Headline>
      ),
      h5: ({
        children,
        ...props
      }: ClassAttributes<HTMLHeadingElement> &
        HTMLAttributes<HTMLHeadingElement>) => (
        <Headline level={5} {...props}>
          {children}
        </Headline>
      ),
      h6: ({
        children,
        ...props
      }: ClassAttributes<HTMLHeadingElement> &
        HTMLAttributes<HTMLHeadingElement>) => (
        <Headline level={6} {...props}>
          {children}
        </Headline>
      ),
      code: (
        props: JSX.IntrinsicAttributes &
          ClassAttributes<HTMLElement> &
          HTMLAttributes<HTMLElement> & { 'data-line'?: string },
      ) => {
        const { className, children, 'data-line': dataLine } = props
        if (className?.includes('language-mermaid')) {
          const code = String(children ?? '').trim()
          return <MermaidBlock code={code} dataLine={dataLine} />
        }

        if (className?.includes('language-')) {
          return (
            <code data-line={dataLine} className={className} {...props}>
              {children}
            </code>
          )
        }
        if (
          children &&
          typeof children === 'string' &&
          children.includes('\n')
        ) {
          return <code data-line={dataLine}>{children}</code>
        }
        return (
          <code data-line={dataLine} className="inline-code">
            {children}
          </code>
        )
      },
    }),
    [],
  )

  return (
    <HeadlineIdProvider>
      <ReactMarkdown
        remarkPlugins={[remarkGfm, remarkLineNumber]}
        rehypePlugins={[rehypeHighlight]}
        components={components}
      >
        {content}
      </ReactMarkdown>
      <div id="mermaid-renderer"></div>
    </HeadlineIdProvider>
  )
}
