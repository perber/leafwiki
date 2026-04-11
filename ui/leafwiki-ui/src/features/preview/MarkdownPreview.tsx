import './markdownPreviewCodeTheme.css'
import { useDesignModeStore } from '@/features/designtoggle/designmode'
import { withBasePath } from '@/lib/routePath'
import {
  Children,
  AnchorHTMLAttributes,
  AudioHTMLAttributes,
  BlockquoteHTMLAttributes,
  ClassAttributes,
  Component,
  ErrorInfo,
  HTMLAttributes,
  ReactElement,
  isValidElement,
  ReactNode,
  useCallback,
  useMemo,
  useSyncExternalStore,
  VideoHTMLAttributes,
} from 'react'
import ReactMarkdown from 'react-markdown'
import { JSX } from 'react/jsx-runtime'
import rehypeHighlight from 'rehype-highlight'
import rehypeRaw from 'rehype-raw'
import rehypeSanitize, { defaultSchema } from 'rehype-sanitize'
import remarkGfm from 'remark-gfm'
import Headline from './Headline'
import MarkdownCodeBlock from './MarkdownCodeBlock'
import { MarkdownImage } from './MarkdownImage'
import { MarkdownLink } from './MarkdownLink'
import MermaidBlock from './MermaidBlock'
import { normalizeMarkdownListIndentation } from './normalizeMarkdownListIndentation'
import { normalizeMarkdownShoutouts } from './normalizeMarkdownShoutouts'
import { rehypeLineNumber } from './rehypeLineNumber'
import { rehypeWhitelistStyles } from './rehypeWhitelistStyles'

const schema = {
  ...defaultSchema,
  clobberPrefix: '',
  tagNames: [...(defaultSchema.tagNames || []), 'audio', 'video'],
  attributes: {
    ...defaultSchema.attributes,
    '*': [
      ...(defaultSchema.attributes?.['*'] || []),
      'data-leafwiki-generated-id',
      'data-line',
      'style',
    ],
    audio: [...(defaultSchema.attributes?.audio || []), 'controls', 'src'],
    video: [
      ...(defaultSchema.attributes?.video || []),
      'controls',
      'src',
      'preload',
    ],
  },
}

type Props = {
  content: string
  path?: string
}

type MarkdownPreviewErrorBoundaryState = {
  hasError: boolean
}

const CLOBBER_PREFIX = ''
const FOOTNOTE_TARGET_PREFIX = '#user-content-fn'

type MarkdownNodeProp = {
  node?: unknown
}

type AlertKind = 'info' | 'success' | 'warning' | 'error'

function getTextContent(node: ReactNode): string {
  if (typeof node === 'string' || typeof node === 'number') {
    return String(node)
  }

  if (Array.isArray(node)) {
    return node.map(getTextContent).join('')
  }

  if (isValidElement<{ children?: ReactNode }>(node)) {
    return getTextContent(node.props.children)
  }

  return ''
}

function getAlertKind(children: ReactNode): AlertKind | null {
  const childArray = Children.toArray(children)
  // HAST includes whitespace text nodes between block elements; skip them
  const firstChild = childArray.find(
    (child) => typeof child !== 'string' || child.trim() !== '',
  )

  if (!isValidElement<{ children?: ReactNode }>(firstChild)) {
    return null
  }

  if (firstChild.type !== 'p') {
    return null
  }

  const marker = getTextContent(firstChild.props.children).trim()
  if (marker === '[!INFO]') return 'info'
  if (marker === '[!SUCCESS]') return 'success'
  if (marker === '[!WARNING]') return 'warning'
  if (marker === '[!ERROR]') return 'error'
  return null
}

function getAlertLabel(kind: AlertKind) {
  if (kind === 'info') return 'Info'
  if (kind === 'success') return 'Success'
  if (kind === 'warning') return 'Warning'
  return 'Error'
}

class MarkdownPreviewErrorBoundary extends Component<
  { children: ReactNode; resetKey: string },
  MarkdownPreviewErrorBoundaryState
> {
  state: MarkdownPreviewErrorBoundaryState = { hasError: false }

  static getDerivedStateFromError(): MarkdownPreviewErrorBoundaryState {
    return { hasError: true }
  }

  componentDidCatch(error: Error, errorInfo: ErrorInfo) {
    console.error('Markdown preview failed to render', error, errorInfo)
  }

  componentDidUpdate(prevProps: { children: ReactNode; resetKey: string }) {
    if (this.state.hasError && prevProps.resetKey !== this.props.resetKey) {
      this.setState({ hasError: false })
    }
  }

  render() {
    if (this.state.hasError) {
      return (
        <div className="border-destructive/40 bg-destructive/5 text-destructive rounded-md border p-4 text-sm">
          This page contains Markdown that could not be rendered safely.
        </div>
      )
    }

    return this.props.children
  }
}

function normalizeAssetMediaSrc(src?: string) {
  if (!src) return src
  if (src.startsWith('/assets/')) {
    return withBasePath(src)
  }
  if (src.startsWith('assets/')) {
    return withBasePath(`/${src}`)
  }
  return src
}

function normalizeFootnoteHref(href?: string) {
  if (!href?.startsWith(FOOTNOTE_TARGET_PREFIX)) {
    return href
  }

  return `#${CLOBBER_PREFIX}${href.slice(1)}`
}

function isPlainListParagraph(
  child: ReactNode,
): child is ReactElement<{ children?: ReactNode; 'data-line'?: string }> {
  if (
    !isValidElement<{ children?: ReactNode; 'data-line'?: string }>(child) ||
    child.type !== 'p'
  ) {
    return false
  }

  const propKeys = Object.keys(child.props)
  return propKeys.every((key) => key === 'children' || key === 'data-line')
}

export default function MarkdownPreview({ content, path }: Props) {
  const designMode = useDesignModeStore((state) => state.mode)
  const prefersLight = useSyncExternalStore(
    (onStoreChange) => {
      if (typeof window === 'undefined' || designMode !== 'system') {
        return () => {}
      }

      const mediaQuery = window.matchMedia('(prefers-color-scheme: light)')
      mediaQuery.addEventListener('change', onStoreChange)
      return () => {
        mediaQuery.removeEventListener('change', onStoreChange)
      }
    },
    () => {
      if (typeof window === 'undefined') return true
      return window.matchMedia('(prefers-color-scheme: light)').matches
    },
    () => true,
  )

  const resolvedMode =
    designMode === 'system' ? (prefersLight ? 'light' : 'dark') : designMode

  const markdownLink = useCallback(
    ({
      node,
      ...props
    }: MarkdownNodeProp &
      ClassAttributes<HTMLAnchorElement> &
      AnchorHTMLAttributes<HTMLAnchorElement>) => {
      void node
      return (
        <MarkdownLink
          path={path}
          {...props}
          href={normalizeFootnoteHref(props.href)}
        />
      )
    },
    [path],
  )

  const components = useMemo(
    () => ({
      a: markdownLink,
      img: MarkdownImage,
      audio: ({
        node,
        ...props
      }: MarkdownNodeProp & AudioHTMLAttributes<HTMLAudioElement>) => {
        void node
        return <audio {...props} src={normalizeAssetMediaSrc(props.src)} />
      },
      video: ({
        node,
        ...props
      }: MarkdownNodeProp & VideoHTMLAttributes<HTMLVideoElement>) => {
        void node
        return <video {...props} src={normalizeAssetMediaSrc(props.src)} />
      },
      section: ({
        children,
        node,
        className,
        ...props
      }: MarkdownNodeProp &
        HTMLAttributes<HTMLElement> & {
          'data-footnotes'?: boolean | string
        }) => {
        void node
        if ('data-footnotes' in props) {
          return (
            <div
              {...props}
              className={`markdown-footnotes ${className ?? ''}`.trim()}
            >
              {children}
            </div>
          )
        }

        return (
          <section {...props} className={className}>
            {children}
          </section>
        )
      },
      li: ({
        children,
        node,
        ...props
      }: MarkdownNodeProp &
        ClassAttributes<HTMLLIElement> &
        HTMLAttributes<HTMLLIElement>) => {
        void node
        const childArray = Array.isArray(children) ? children : [children]
        const meaningfulChildren = childArray.filter(
          (child) => child !== null && child !== undefined && child !== false,
        )
        const onlyChild = meaningfulChildren[0]

        if (
          meaningfulChildren.length === 1 &&
          isPlainListParagraph(onlyChild)
        ) {
          return <li {...props}>{onlyChild.props.children}</li>
        }

        return <li {...props}>{children}</li>
      },
      blockquote: ({
        children,
        node,
        className,
        'data-line': dataLine,
        ...props
      }: MarkdownNodeProp &
        ClassAttributes<HTMLQuoteElement> &
        BlockquoteHTMLAttributes<HTMLQuoteElement> & {
          'data-line'?: string
        }) => {
        void node
        const alertKind = getAlertKind(children)

        if (!alertKind) {
          return (
            <blockquote {...props} data-line={dataLine} className={className}>
              {children}
            </blockquote>
          )
        }

        const childArray = Children.toArray(children)
        // Find the marker paragraph index so content starts after it,
        // accounting for leading whitespace text nodes
        const markerIndex = childArray.findIndex(
          (child) => isValidElement(child) && child.type === 'p',
        )
        const contentChildren = (
          markerIndex >= 0 ? childArray.slice(markerIndex + 1) : []
        ).filter((child) => typeof child !== 'string' || child.trim() !== '')

        return (
          <aside
            {...props}
            data-line={dataLine}
            className={`markdown-shoutout markdown-shoutout--${alertKind} ${className ?? ''}`.trim()}
          >
            <p className="markdown-shoutout__title">
              {getAlertLabel(alertKind)}
            </p>
            <div className="markdown-shoutout__content">{contentChildren}</div>
          </aside>
        )
      },
      h1: ({
        children,
        node,
        ...props
      }: MarkdownNodeProp &
        ClassAttributes<HTMLHeadingElement> &
        HTMLAttributes<HTMLHeadingElement>) => {
        void node
        return (
          <Headline level={1} {...props}>
            {children}
          </Headline>
        )
      },
      h2: ({
        children,
        node,
        ...props
      }: MarkdownNodeProp &
        ClassAttributes<HTMLHeadingElement> &
        HTMLAttributes<HTMLHeadingElement>) => {
        void node
        return (
          <Headline level={2} {...props}>
            {children}
          </Headline>
        )
      },
      h3: ({
        children,
        node,
        ...props
      }: MarkdownNodeProp &
        ClassAttributes<HTMLHeadingElement> &
        HTMLAttributes<HTMLHeadingElement>) => {
        void node
        return (
          <Headline level={3} {...props}>
            {children}
          </Headline>
        )
      },
      h4: ({
        children,
        node,
        ...props
      }: MarkdownNodeProp &
        ClassAttributes<HTMLHeadingElement> &
        HTMLAttributes<HTMLHeadingElement>) => {
        void node
        return (
          <Headline level={4} {...props}>
            {children}
          </Headline>
        )
      },
      h5: ({
        children,
        node,
        ...props
      }: MarkdownNodeProp &
        ClassAttributes<HTMLHeadingElement> &
        HTMLAttributes<HTMLHeadingElement>) => {
        void node
        return (
          <Headline level={5} {...props}>
            {children}
          </Headline>
        )
      },
      h6: ({
        children,
        node,
        ...props
      }: MarkdownNodeProp &
        ClassAttributes<HTMLHeadingElement> &
        HTMLAttributes<HTMLHeadingElement>) => {
        void node
        return (
          <Headline level={6} {...props}>
            {children}
          </Headline>
        )
      },
      table: ({
        node,
        ...props
      }: MarkdownNodeProp &
        ClassAttributes<HTMLTableElement> &
        HTMLAttributes<HTMLTableElement>) => {
        void node
        return (
          <div className="table-wrapper custom-scrollbar">
            <table
              {...props}
              className={`custom-scrollbar ${props.className ?? ''}`.trim()}
            />
          </div>
        )
      },
      pre: MarkdownCodeBlock,
      code: ({
        node,
        ...props
      }: MarkdownNodeProp &
        JSX.IntrinsicAttributes &
        ClassAttributes<HTMLElement> &
        HTMLAttributes<HTMLElement> & { 'data-line'?: string }) => {
        void node
        const { className, children, 'data-line': dataLine } = props
        if (className?.includes('language-mermaid')) {
          const code = String(children ?? '').trim()
          return (
            <MermaidBlock
              code={code}
              dataLine={dataLine}
              theme={resolvedMode === 'dark' ? 'dark' : 'default'}
            />
          )
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
    [markdownLink, resolvedMode],
  )

  const normalizedContent = useMemo(
    () => normalizeMarkdownListIndentation(normalizeMarkdownShoutouts(content)),
    [content],
  )

  return (
    <MarkdownPreviewErrorBoundary resetKey={`${path ?? ''}:${content}`}>
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
          {normalizedContent}
        </ReactMarkdown>
        <div id="mermaid-renderer"></div>
      </>
    </MarkdownPreviewErrorBoundary>
  )
}
