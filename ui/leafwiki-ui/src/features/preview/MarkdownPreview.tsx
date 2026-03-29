import 'highlight.js/styles/github-dark.css'
import { ClassAttributes, HTMLAttributes, useCallback, useMemo } from 'react'
import ReactMarkdown from 'react-markdown'
import { JSX } from 'react/jsx-runtime'
import rehypeHighlight from 'rehype-highlight'
import rehypeRaw from 'rehype-raw'
import rehypeSanitize, { defaultSchema } from 'rehype-sanitize'
import remarkGfm from 'remark-gfm'
import Headline from './Headline'
import { MarkdownImage } from './MarkdownImage'
import { MarkdownLink } from './MarkdownLink'
import MermaidBlock from './MermaidBlock'
import { rehypeLineNumber } from './rehypeLineNumber'
import { rehypeWhitelistStyles } from './rehypeWhitelistStyles'

const schema = {
  ...defaultSchema,
  attributes: {
    ...defaultSchema.attributes,
    '*': [...(defaultSchema.attributes?.['*'] || []), 'data-line', 'style'],
  },
}

type Props = {
  content: string
  path?: string
  resolveAssetUrl?: (src: string) => string
  enableHeadlineLinks?: boolean
}

export default function MarkdownPreview({
  content,
  path,
  resolveAssetUrl,
  enableHeadlineLinks = true,
}: Props) {
  const markdownLink = useCallback(
    (
      props: ClassAttributes<HTMLAnchorElement> &
        HTMLAttributes<HTMLAnchorElement>,
    ) => (
      <MarkdownLink path={path} resolveAssetUrl={resolveAssetUrl} {...props} />
    ),
    [path, resolveAssetUrl],
  )

  const components = useMemo(
    () => ({
      a: markdownLink,
      img: (
        props: JSX.IntrinsicAttributes &
          ClassAttributes<HTMLImageElement> &
          HTMLAttributes<HTMLImageElement>,
      ) => <MarkdownImage resolveAssetUrl={resolveAssetUrl} {...props} />,
      h1: ({
        children,
        ...props
      }: ClassAttributes<HTMLHeadingElement> &
        HTMLAttributes<HTMLHeadingElement>) =>
        enableHeadlineLinks ? (
          <Headline level={1} {...props}>
            {children}
          </Headline>
        ) : (
          <h1 {...props}>{children}</h1>
        ),
      h2: ({
        children,
        ...props
      }: ClassAttributes<HTMLHeadingElement> &
        HTMLAttributes<HTMLHeadingElement>) =>
        enableHeadlineLinks ? (
          <Headline level={2} {...props}>
            {children}
          </Headline>
        ) : (
          <h2 {...props}>{children}</h2>
        ),
      h3: ({
        children,
        ...props
      }: ClassAttributes<HTMLHeadingElement> &
        HTMLAttributes<HTMLHeadingElement>) =>
        enableHeadlineLinks ? (
          <Headline level={3} {...props}>
            {children}
          </Headline>
        ) : (
          <h3 {...props}>{children}</h3>
        ),
      h4: ({
        children,
        ...props
      }: ClassAttributes<HTMLHeadingElement> &
        HTMLAttributes<HTMLHeadingElement>) =>
        enableHeadlineLinks ? (
          <Headline level={4} {...props}>
            {children}
          </Headline>
        ) : (
          <h4 {...props}>{children}</h4>
        ),
      h5: ({
        children,
        ...props
      }: ClassAttributes<HTMLHeadingElement> &
        HTMLAttributes<HTMLHeadingElement>) =>
        enableHeadlineLinks ? (
          <Headline level={5} {...props}>
            {children}
          </Headline>
        ) : (
          <h5 {...props}>{children}</h5>
        ),
      h6: ({
        children,
        ...props
      }: ClassAttributes<HTMLHeadingElement> &
        HTMLAttributes<HTMLHeadingElement>) =>
        enableHeadlineLinks ? (
          <Headline level={6} {...props}>
            {children}
          </Headline>
        ) : (
          <h6 {...props}>{children}</h6>
        ),
      table: (
        props: ClassAttributes<HTMLTableElement> &
          HTMLAttributes<HTMLTableElement>,
      ) => (
        <div className="table-wrapper custom-scrollbar">
          <table {...props} />
        </div>
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
    [enableHeadlineLinks, markdownLink, resolveAssetUrl],
  )

  return (
    <>
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        rehypePlugins={[
          rehypeRaw,
          rehypeLineNumber,
          rehypeWhitelistStyles,
          [rehypeSanitize, schema],
          rehypeHighlight,
        ]}
        components={components}
      >
        {content}
      </ReactMarkdown>
      <div id="mermaid-renderer"></div>
    </>
  )
}
