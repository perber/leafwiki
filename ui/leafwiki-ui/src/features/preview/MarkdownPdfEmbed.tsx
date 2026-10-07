import { versionAssetSrc } from '@/lib/assetSrc'
import i18next from '@/lib/i18n'
import { useIsMobile } from '@/lib/useIsMobile'
import { ExternalLink, FileText } from 'lucide-react'
import { useMemo } from 'react'

type MarkdownPdfEmbedProps = React.ImgHTMLAttributes<HTMLImageElement> & {
  resolveAssetUrl?: (src: string) => string
}

// A `#page=N` fragment on the source (e.g. `manual.pdf#page=3`) is a standard
// PDF "open parameter" that Chrome/Firefox/Edge's built-in viewer honors to
// open on that page — kept intact below through URL/searchParams handling
// since the fragment is never part of `search`.
function isSameOrigin(src: string): boolean {
  try {
    return new URL(src, location.origin).origin === location.origin
  } catch {
    return false
  }
}

// fileNameOf returns the decoded last path segment of src (without query or
// fragment), e.g. "Benutzer Handbuch.pdf" for "/assets/Benutzer%20Handbuch.pdf#page=2".
function fileNameOf(src: string): string {
  const path = src.split(/[?#]/)[0]
  const name = path.slice(path.lastIndexOf('/') + 1)
  try {
    return decodeURIComponent(name)
  } catch {
    return name
  }
}

export function MarkdownPdfEmbed({
  src = '',
  alt,
  width,
  resolveAssetUrl,
}: MarkdownPdfEmbedProps) {
  const resolvedSrc = useMemo(
    () => resolveAssetUrl?.(src) ?? src,
    [resolveAssetUrl, src],
  )
  const versionedSrc = useMemo(
    () => versionAssetSrc(resolvedSrc),
    [resolvedSrc],
  )
  const isMobile = useIsMobile()
  const openLabel = i18next.t('imagePreview.openInNewTab', { ns: 'viewer' })

  // In-page PDF iframes aren't usable on mobile (no pinch-zoom in most
  // mobile browsers, fixed embed height), so skip the embed there and only
  // offer the "open" action. A PDF from another origin is never framed
  // either: the page would embed whatever that site serves.
  if (isMobile || !isSameOrigin(versionedSrc)) {
    const fileName = fileNameOf(resolvedSrc)
    const title = alt || fileName
    return (
      <span
        className="markdown-pdf-embed"
        style={width ? { width } : undefined}
      >
        <a
          href={versionedSrc}
          target="_blank"
          rel="noopener noreferrer"
          className="markdown-pdf-card"
        >
          <FileText size={20} className="markdown-pdf-card__icon" />
          <span className="markdown-pdf-card__body">
            <span className="markdown-pdf-card__title">{title}</span>
            <span className="markdown-pdf-card__meta">
              {alt && fileName ? `${fileName} · ` : ''}
              {openLabel}
            </span>
          </span>
          <ExternalLink
            size={14}
            className="markdown-pdf-card__arrow ml-auto"
          />
        </a>
      </span>
    )
  }

  return (
    <span className="markdown-pdf-embed" style={{ width: width || '100%' }}>
      <iframe
        src={versionedSrc}
        title={alt || i18next.t('pdfEmbed.defaultTitle', { ns: 'viewer' })}
        className="markdown-pdf-embed__frame"
        loading="lazy"
      />
      <a
        href={versionedSrc}
        target="_blank"
        rel="noopener noreferrer"
        className="markdown-pdf-embed__open-link"
      >
        <ExternalLink size={14} />
        {openLabel}
      </a>
    </span>
  )
}
