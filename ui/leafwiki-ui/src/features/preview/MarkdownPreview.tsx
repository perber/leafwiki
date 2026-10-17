import { useDesignModeStore } from '@/features/designtoggle/designmode'
import { normalizeAssetSrc } from '@/lib/assetSrc'
import {
  buildWikiTitleIndex,
  preprocessWikilinks,
} from '@/lib/preprocessWikilinks'
import { useTreeStore } from '@/stores/tree'
import 'katex/dist/katex.min.css'
import {
  AnchorHTMLAttributes,
  AudioHTMLAttributes,
  BlockquoteHTMLAttributes,
  Children,
  ClassAttributes,
  HTMLAttributes,
  ImgHTMLAttributes,
  VideoHTMLAttributes,
  isValidElement,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  useSyncExternalStore,
} from 'react'
import ReactMarkdown from 'react-markdown'
import { JSX } from 'react/jsx-runtime'
import rehypeHighlight from 'rehype-highlight'
import rehypeKatex from 'rehype-katex'
import rehypeRaw from 'rehype-raw'
import rehypeSanitize from 'rehype-sanitize'
import remarkFlexibleMarkers from 'remark-flexible-markers'
import remarkGfm from 'remark-gfm'
import remarkMath from 'remark-math'
import { extractTocEntries } from './extractTocEntries'
import Headline from './Headline'
import MarkdownCodeBlock from './MarkdownCodeBlock'
import MarkdownInlineCode from './MarkdownInlineCode'
import { MarkdownImage } from './MarkdownImage'
import { MarkdownLink } from './MarkdownLink'
import { MarkdownPdfEmbed } from './MarkdownPdfEmbed'
import './markdownPreviewCodeTheme.css'
import MermaidBlock from './MermaidBlock'
import { normalizeMarkdownListIndentation } from './normalizeMarkdownListIndentation'
import { normalizeMarkdownBlocks } from './normalizeMarkdownBlocks'
import { rehypeCodeFenceLineNumbers } from './rehypeCodeFenceLineNumbers'
import { rehypeLineNumber } from './rehypeLineNumber'
import { rehypeWhitelistStyles } from './rehypeWhitelistStyles'
import { syntaxHighlightLanguages } from './syntaxHighlightLanguages'
import { TocDropdownButton } from './TocDropdownButton'
import { remarkImageSize } from './remarkImageSize'
import { MarkdownPreviewErrorBoundary } from './MarkdownPreviewErrorBoundary'
import {
  findScrollParent,
  getSemanticShoutoutTitle,
  getShoutoutConfig,
  isPdfSrc,
  isPlainListParagraph,
  normalizeFootnoteHref,
  transformMarkdownUrl,
  type MarkdownNodeProp,
} from './previewHelpers'
import { previewSanitizeSchema } from './previewSanitizeSchema'

type Props = {
  content: string
  path?: string
  resolveAssetUrl?: (src: string) => string
  enableHeadlineLinks?: boolean
  showToc?: boolean
  tocClickable?: boolean
  onStickyTocChange?: (show: boolean) => void
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
        ClassAttributes<HTMLImageElement> &
        ImgHTMLAttributes<HTMLImageElement>) => {
        void node
        if (isPdfSrc(props.src)) {
          return (
            <MarkdownPdfEmbed {...props} resolveAssetUrl={resolveAssetUrl} />
          )
        }
        return <MarkdownImage {...props} resolveAssetUrl={resolveAssetUrl} />
      },
      audio: ({
        node,
        ...props
      }: MarkdownNodeProp & AudioHTMLAttributes<HTMLAudioElement>) => {
        void node
        const resolvedSrc = resolveAssetUrl?.(props.src ?? '') ?? props.src
        return (
          <audio
            {...props}
            src={resolvedSrc ? normalizeAssetSrc(resolvedSrc) : resolvedSrc}
          />
        )
      },
      video: ({
        node,
        ...props
      }: MarkdownNodeProp & VideoHTMLAttributes<HTMLVideoElement>) => {
        void node
        const resolvedSrc = resolveAssetUrl?.(props.src ?? '') ?? props.src
        return (
          <video
            {...props}
            src={resolvedSrc ? normalizeAssetSrc(resolvedSrc) : resolvedSrc}
          />
        )
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
        HTMLAttributes<HTMLElement> & {
          'data-line'?: string
          'data-line-numbers'?: string | boolean
        }) => {
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
          <MarkdownInlineCode data-line={dataLine} className={className}>
            {children}
          </MarkdownInlineCode>
        )
      },
    }),
    [enableHeadlineLinks, markdownLink, resolveAssetUrl, resolvedMode],
  )

  const wikiTitleIndex = useMemo(
    () => buildWikiTitleIndex(treeById),
    [treeById],
  )

  const normalizedContent = useMemo(
    () =>
      normalizeMarkdownListIndentation(
        normalizeMarkdownBlocks(
          preprocessWikilinks(
            content,
            (title) => wikiTitleIndex.get(title.toLowerCase()) ?? [],
          ),
        ),
      ),
    [content, wikiTitleIndex],
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

  // Memoized on the resolved markdown string: tree updates (e.g. drag reorder
  // in the sidebar) swap the store's byId identity on every change, and without
  // this the whole remark/rehype pipeline would re-parse and re-render the
  // article on each of those updates even though nothing in it changed.
  const markdownBody = useMemo(
    () => (
      <MarkdownPreviewErrorBoundary resetKey={`${path ?? ''}:${content}`}>
        <>
          <ReactMarkdown
            // singleDollarTextMath disabled: $var in bash/code prose would be parsed
            // as math delimiters and conflict with wikilink preprocessing. Use $$...$$ for math.
            remarkPlugins={[
              [remarkMath, { singleDollarTextMath: false }],
              remarkGfm,
              remarkFlexibleMarkers,
              remarkImageSize,
            ]}
            rehypePlugins={[
              rehypeRaw,
              rehypeLineNumber,
              rehypeCodeFenceLineNumbers,
              rehypeWhitelistStyles,
              [rehypeKatex, { output: 'html', strict: 'ignore' }],
              [rehypeSanitize, previewSanitizeSchema],
              [
                rehypeHighlight,
                {
                  languages: syntaxHighlightLanguages,
                },
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
    ),
    [normalizedContent, components, path, content],
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
