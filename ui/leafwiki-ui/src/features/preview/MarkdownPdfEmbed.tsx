import { versionAssetSrc } from '@/lib/assetSrc'
import i18next from '@/lib/i18n'
import { ExternalLink } from 'lucide-react'
import { useMemo } from 'react'

type MarkdownPdfEmbedProps = React.ImgHTMLAttributes<HTMLImageElement> & {
  resolveAssetUrl?: (src: string) => string
}

// A `#page=N` fragment on the source (e.g. `manual.pdf#page=3`) is a standard
// PDF "open parameter" that Chrome/Firefox/Edge's built-in viewer honors to
// open on that page — kept intact below through URL/searchParams handling
// since the fragment is never part of `search`.
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
        {i18next.t('imagePreview.openInNewTab', { ns: 'viewer' })}
      </a>
    </span>
  )
}
