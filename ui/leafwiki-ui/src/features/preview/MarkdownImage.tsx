/* eslint-disable react-hooks/set-state-in-effect */
import { DIALOG_IMAGE_PREVIEW } from '@/lib/registries'
import { withBasePath } from '@/lib/routePath'
import { useDialogsStore } from '@/stores/dialogs'
import { useEffect, useMemo, useState } from 'react'

type Props = React.ImgHTMLAttributes<HTMLImageElement> & { node?: unknown }
type MarkdownImageProps = Omit<Props, 'node'> & {
  resolveAssetUrl?: (src: string) => string
}

function shouldOpenPreview(e: React.MouseEvent<HTMLImageElement>) {
  if (e.button !== 0) return false
  if (e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return false
  return true
}

function shouldOpenInNewTab(e: React.MouseEvent<HTMLImageElement>) {
  return e.button === 0 && (e.metaKey || e.ctrlKey)
}

function normalizeImageSrc(src: string) {
  if (src.startsWith('/assets/') || src.startsWith('/api/')) {
    return withBasePath(src)
  }

  if (src.startsWith('assets/')) {
    return withBasePath(`/${src}`)
  }

  return src
}

export function MarkdownImage({
  src = '',
  style,
  alt,
  node,
  resolveAssetUrl,
  ...rest
}: MarkdownImageProps & { node?: unknown }) {
  void node
  const openDialog = useDialogsStore((s) => s.openDialog)
  const resolvedSrc = useMemo(
    () => resolveAssetUrl?.(src) ?? src,
    [resolveAssetUrl, src],
  )
  const [versionedSrc, setVersionedSrc] = useState(() =>
    normalizeImageSrc(resolvedSrc),
  )

  useEffect(() => {
    if (
      !resolvedSrc?.startsWith('/assets/') &&
      !resolvedSrc?.startsWith('assets/') &&
      !resolvedSrc?.startsWith('/api/')
    ) {
      setVersionedSrc(normalizeImageSrc(resolvedSrc))
      return
    }

    try {
      const url = new URL(normalizeImageSrc(resolvedSrc), location.origin)
      if (!url.searchParams.has('v')) {
        url.searchParams.set('v', Date.now().toString())
      }
      setVersionedSrc(url.toString())
    } catch {
      setVersionedSrc(normalizeImageSrc(resolvedSrc))
    }
  }, [resolvedSrc])

  return (
    <img
      src={versionedSrc}
      alt={alt}
      style={{
        ...style,
        cursor: 'zoom-in',
      }}
      draggable={false}
      {...rest}
      onClick={(e) => {
        rest.onClick?.(e)

        if (shouldOpenInNewTab(e)) {
          window.open(versionedSrc, '_blank', 'noopener,noreferrer')
          return
        }

        if (!shouldOpenPreview(e)) {
          return
        }

        e.preventDefault()
        openDialog(DIALOG_IMAGE_PREVIEW, { src: versionedSrc, alt })
      }}
    />
  )
}
