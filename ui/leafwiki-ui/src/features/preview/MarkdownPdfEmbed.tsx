import i18next from '@/lib/i18n'
import { withBasePath } from '@/lib/routePath'
import { ExternalLink } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'

type MarkdownPdfEmbedProps = React.ImgHTMLAttributes<HTMLImageElement> & {
  resolveAssetUrl?: (src: string) => string
}

function normalizePdfSrc(src: string) {
  if (src.startsWith('/assets/') || src.startsWith('/api/')) {
    return withBasePath(src)
  }

  if (src.startsWith('assets/')) {
    return withBasePath(`/${src}`)
  }

  return src
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
  const [versionedSrc, setVersionedSrc] = useState(() =>
    normalizePdfSrc(resolvedSrc),
  )

  useEffect(() => {
    if (
      !resolvedSrc?.startsWith('/assets/') &&
      !resolvedSrc?.startsWith('assets/') &&
      !resolvedSrc?.startsWith('/api/')
    ) {
      setVersionedSrc(normalizePdfSrc(resolvedSrc))
      return
    }

    try {
      const url = new URL(normalizePdfSrc(resolvedSrc), location.origin)
      if (!url.searchParams.has('v')) {
        url.searchParams.set('v', Date.now().toString())
      }
      setVersionedSrc(url.toString())
    } catch {
      setVersionedSrc(normalizePdfSrc(resolvedSrc))
    }
  }, [resolvedSrc])

  return (
    <span className="markdown-pdf-embed" style={{ width: width || '100%' }}>
      <iframe
        src={versionedSrc}
        title={alt || 'PDF'}
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
