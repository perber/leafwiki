import { useDesignModeStore } from '@/features/designtoggle/designmode'
import { withBasePath } from '@/lib/routePath'
import { useTreeStore } from '@/stores/tree'
import {
  AnchorHTMLAttributes,
  AudioHTMLAttributes,
  BlockquoteHTMLAttributes,
  Children,
  ClassAttributes,
  Component,
  ErrorInfo,
  HTMLAttributes,
  ReactElement,
  ReactNode,
  VideoHTMLAttributes,
  isValidElement,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  useSyncExternalStore,
} from 'react'
import ReactMarkdown, { defaultUrlTransform } from 'react-markdown'
import { JSX } from 'react/jsx-runtime'
import rehypeHighlight from 'rehype-highlight'
import dockerfile from 'highlight.js/lib/languages/dockerfile'
import http from 'highlight.js/lib/languages/http'
import nginx from 'highlight.js/lib/languages/nginx'
import nix from 'highlight.js/lib/languages/nix'
import protobuf from 'highlight.js/lib/languages/protobuf'
import rehypeKatex from 'rehype-katex'
import rehypeRaw from 'rehype-raw'
import rehypeSanitize, { defaultSchema } from 'rehype-sanitize'
import remarkGfm from 'remark-gfm'
import remarkMath from 'remark-math'
import { extractTocEntries } from './extractTocEntries'
import { TocDropdownButton } from './TocDropdownButton'
import Headline from './Headline'
import MarkdownCodeBlock from './MarkdownCodeBlock'
import { MarkdownImage } from './MarkdownImage'
import { MarkdownLink } from './MarkdownLink'
import './markdownPreviewCodeTheme.css'
import MermaidBlock from './MermaidBlock'
import { normalizeMarkdownListIndentation } from './normalizeMarkdownListIndentation'
import { normalizeMarkdownShoutouts } from './normalizeMarkdownShoutouts'
import { preprocessWikilinks } from '@/lib/preprocessWikilinks'
import { rehypeLineNumber } from './rehypeLineNumber'
import { rehypeWhitelistStyles } from './rehypeWhitelistStyles'
import 'katex/dist/katex.min.css'

const schema = {
  ...defaultSchema,
  clobberPrefix: '',
  protocols: {
    ...defaultSchema.protocols,
    href: [
      ...(defaultSchema.protocols?.href ?? []),
      'wikilink-notfound',
      'wikilink-ambiguous',
    ],
  },
  tagNames: [...(defaultSchema.tagNames || []), 'audio', 'video'],
  attributes: {
    ...defaultSchema.attributes,
    '*': [
      ...(defaultSchema.attributes?.['*'] || []),
      'class',
      'className',
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
  resolveAssetUrl?: (src: string) => string
  enableHeadlineLinks?: boolean
  showToc?: boolean
  tocClickable?: boolean
  onStickyTocChange?: (show: boolean) => void
}

type MarkdownPreviewErrorBoundaryState = {
  hasError: boolean
}

const CLOBBER_PREFIX = ''
const FOOTNOTE_TARGET_PREFIX = '#user-content-fn'
const WIKILINK_PROTOCOLS = [
  'wikilink-notfound:',
  'wikilink-ambiguous:',
] as const

type MarkdownNodeProp = {
  node?: unknown
}

type SemanticAlertKind = 'info' | 'success' | 'warning' | 'error'

type ShoutoutConfig = {
  kind: string
}

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

function getShoutoutConfig(children: ReactNode): ShoutoutConfig | null {
  const childArray = Children.toArray(children)
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
  const match = marker.match(/^\[!(?<kind>[A-Z][A-Z0-9_-]*)\]$/)
  if (!match?.groups?.kind) {
    return null
  }

  const kind = match.groups.kind.toLowerCase()

  return { kind }
}

function getAlertLabel(kind: SemanticAlertKind) {
  if (kind === 'info') return 'Info'
  if (kind === 'success') return 'Success'
  if (kind === 'warning') return 'Warning'
  return 'Error'
}

function getSemanticShoutoutTitle(kind: string) {
  if (
    kind === 'info' ||
    kind === 'success' ||
    kind === 'warning' ||
    kind === 'error'
  ) {
    return getAlertLabel(kind)
  }

  return null
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

function transformMarkdownUrl(url: string) {
  if (WIKILINK_PROTOCOLS.some((protocol) => url.startsWith(protocol))) {
    return url
  }

  return defaultUrlTransform(url)
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

function findScrollParent(el: HTMLElement | null): HTMLElement | null {
  let node: HTMLElement | null = el?.parentElement ?? null
  while (node) {
    const { overflow, overflowY } = getComputedStyle(node)
    if (/auto|scroll/.test(overflow + overflowY)) return node
    node = node.parentElement
  }
  return null
}

export default function MarkdownPreview({
  content,
  path,
  resolveAssetUrl,
  enableHeadlineLinks = true,
  showToc = false,
  tocClickable = true,
  onStickyTocChange,
}: Props) {
  const treeById = useTreeStore((s) => s.byId)
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
          resolveAssetUrl={resolveAssetUrl}
          {...props}
          href={normalizeFootnoteHref(props.href)}
        />
      )
    },
    [path, resolveAssetUrl],
  )

  const components = useMemo(
    () => ({
      a: markdownLink,
      img: ({
        node,
        ...props
      }: MarkdownNodeProp &
        JSX.IntrinsicAttributes &
        ClassAttributes<HTMLImageElement> &
        HTMLAttributes<HTMLImageElement>) => {
        void node
        return <MarkdownImage resolveAssetUrl={resolveAssetUrl} {...props} />
      },
      audio: ({
        node,
        ...props
      }: MarkdownNodeProp & AudioHTMLAttributes<HTMLAudioElement>) => {
        void node
        const resolvedSrc = resolveAssetUrl?.(props.src ?? '') ?? props.src
        return <audio {...props} src={normalizeAssetMediaSrc(resolvedSrc)} />
      },
      video: ({
        node,
        ...props
      }: MarkdownNodeProp & VideoHTMLAttributes<HTMLVideoElement>) => {
        void node
        const resolvedSrc = resolveAssetUrl?.(props.src ?? '') ?? props.src
        return <video {...props} src={normalizeAssetMediaSrc(resolvedSrc)} />
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
        const shoutoutConfig = getShoutoutConfig(children)

        if (!shoutoutConfig) {
          return (
            <blockquote {...props} data-line={dataLine} className={className}>
              {children}
            </blockquote>
          )
        }

        const childArray = Children.toArray(children)
        const markerIndex = childArray.findIndex(
          (child) => isValidElement(child) && child.type === 'p',
        )
        const contentChildren = (
          markerIndex >= 0 ? childArray.slice(markerIndex + 1) : []
        ).filter((child) => typeof child !== 'string' || child.trim() !== '')

        const title = getSemanticShoutoutTitle(shoutoutConfig.kind)

        return (
          <aside
            {...props}
            data-line={dataLine}
            className={`markdown-shoutout markdown-shoutout--${shoutoutConfig.kind} ${className ?? ''}`.trim()}
          >
            {title ? <p className="markdown-shoutout__title">{title}</p> : null}
            <div className="markdown-shoutout__content">{contentChildren}</div>
          </aside>
        )
      },
      h1: ({
        children,
        ...props
      }: MarkdownNodeProp &
        ClassAttributes<HTMLHeadingElement> &
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
      }: MarkdownNodeProp &
        ClassAttributes<HTMLHeadingElement> &
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
      }: MarkdownNodeProp &
        ClassAttributes<HTMLHeadingElement> &
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
      }: MarkdownNodeProp &
        ClassAttributes<HTMLHeadingElement> &
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
      }: MarkdownNodeProp &
        ClassAttributes<HTMLHeadingElement> &
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
      }: MarkdownNodeProp &
        ClassAttributes<HTMLHeadingElement> &
        HTMLAttributes<HTMLHeadingElement>) =>
        enableHeadlineLinks ? (
          <Headline level={6} {...props}>
            {children}
          </Headline>
        ) : (
          <h6 {...props}>{children}</h6>
        ),
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
    [enableHeadlineLinks, markdownLink, resolveAssetUrl, resolvedMode],
  )

  const normalizedContent = useMemo(
    () =>
      normalizeMarkdownListIndentation(
        normalizeMarkdownShoutouts(
          preprocessWikilinks(content, (title) => {
            const lower = title.toLowerCase()
            return Object.values(treeById).filter(
              (n) => n.title.toLowerCase() === lower,
            )
          }),
        ),
      ),
    [content, treeById],
  )

  const tocEntries = useMemo(
    () => (showToc ? extractTocEntries(normalizedContent) : []),
    [showToc, normalizedContent],
  )

  const inFlowRef = useRef<HTMLDivElement>(null)
  const [showStickyToc, setShowStickyToc] = useState(false)

  useEffect(() => {
    if (!showToc || tocEntries.length <= 3) return
    const el = inFlowRef.current
    if (!el) return
    const root = findScrollParent(el)
    const observer = new IntersectionObserver(
      ([entry]) => {
        const sticky = !entry.isIntersecting
        setShowStickyToc(sticky)
        onStickyTocChange?.(sticky)
      },
      { root, threshold: 0 },
    )
    observer.observe(el)
    return () => observer.disconnect()
  }, [showToc, tocEntries.length, onStickyTocChange])

  const markdownBody = (
    <MarkdownPreviewErrorBoundary resetKey={`${path ?? ''}:${content}`}>
      <>
        <ReactMarkdown
          remarkPlugins={[remarkMath, remarkGfm]}
          rehypePlugins={[
            rehypeRaw,
            rehypeLineNumber,
            rehypeWhitelistStyles,
            [rehypeKatex, { output: 'html', strict: 'ignore' }],
            [rehypeSanitize, schema],
            [
              rehypeHighlight,
              { languages: { dockerfile, http, nginx, nix, protobuf } },
            ],
          ]}
          components={components}
          urlTransform={transformMarkdownUrl}
        >
          {normalizedContent}
        </ReactMarkdown>
        <div id="mermaid-renderer"></div>
      </>
    </MarkdownPreviewErrorBoundary>
  )

  if (!showToc || tocEntries.length <= 3) {
    return markdownBody
  }

  return (
    <>
      <div ref={inFlowRef} className="print:hidden">
        {!showStickyToc && (
          <div className="markdown-preview__toc-inline mb-2 flex sm:mb-4">
            <TocDropdownButton entries={tocEntries} clickable={tocClickable} />
          </div>
        )}
      </div>
      {markdownBody}
    </>
  )
}
