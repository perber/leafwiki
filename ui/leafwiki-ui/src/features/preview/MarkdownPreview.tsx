import './markdownPreviewCodeTheme.css'
import { useDesignModeStore } from '@/features/designtoggle/designmode'
import { withBasePath } from '@/lib/routePath'
import {
  AudioHTMLAttributes,
  ClassAttributes,
  Component,
  ErrorInfo,
  HTMLAttributes,
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
import { rehypeLineNumber } from './rehypeLineNumber'
import { rehypeWhitelistStyles } from './rehypeWhitelistStyles'

const schema = {
  ...defaultSchema,
  clobberPrefix: 'leafwiki-',
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

function isPlainListParagraph(child: ReactNode) {
  if (!isValidElement<{ children?: ReactNode }>(child) || child.type !== 'p') {
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
    (
      props: ClassAttributes<HTMLAnchorElement> &
        HTMLAttributes<HTMLAnchorElement>,
    ) => <MarkdownLink path={path} {...props} />,
    [path],
  )

  const components = useMemo(
    () => ({
      a: markdownLink,
      img: MarkdownImage,
      audio: (props: AudioHTMLAttributes<HTMLAudioElement>) => (
        <audio {...props} src={normalizeAssetMediaSrc(props.src)} />
      ),
      video: (props: VideoHTMLAttributes<HTMLVideoElement>) => (
        <video {...props} src={normalizeAssetMediaSrc(props.src)} />
      ),
      li: ({
        children,
        ...props
      }: ClassAttributes<HTMLLIElement> & HTMLAttributes<HTMLLIElement>) => {
        const childArray = Array.isArray(children) ? children : [children]
        const meaningfulChildren = childArray.filter(
          (child) => child !== null && child !== undefined && child !== false,
        )
        const onlyChild = meaningfulChildren[0]

        if (meaningfulChildren.length === 1 && isPlainListParagraph(onlyChild)) {
          return <li {...props}>{onlyChild.props.children}</li>
        }

        return <li {...props}>{children}</li>
      },
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
      table: (
        props: ClassAttributes<HTMLTableElement> &
          HTMLAttributes<HTMLTableElement>,
      ) => (
        <div className="table-wrapper custom-scrollbar">
          <table
            {...props}
            className={`custom-scrollbar ${props.className ?? ''}`.trim()}
          />
        </div>
      ),
      pre: MarkdownCodeBlock,
      code: (
        props: JSX.IntrinsicAttributes &
          ClassAttributes<HTMLElement> &
          HTMLAttributes<HTMLElement> & { 'data-line'?: string },
      ) => {
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
          {content}
        </ReactMarkdown>
        <div id="mermaid-renderer"></div>
      </>
    </MarkdownPreviewErrorBoundary>
  )
}
