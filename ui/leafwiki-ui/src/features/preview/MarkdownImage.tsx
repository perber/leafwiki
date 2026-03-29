/* eslint-disable react-hooks/set-state-in-effect */
import { DIALOG_IMAGE_PREVIEW } from '@/lib/registries'
import { withBasePath } from '@/lib/routePath'
import { useDialogsStore } from '@/stores/dialogs'
import { useEffect, useState } from 'react'

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

export function MarkdownImage({
  src = '',
  style,
  alt,
  node,
  resolveAssetUrl,
  ...rest
}: MarkdownImageProps & { node?: unknown }) {
  void node
  const [versionedSrc, setVersionedSrc] = useState(src)
  const openDialog = useDialogsStore((s) => s.openDialog)

  useEffect(() => {
    const resolvedSrc = resolveAssetUrl?.(src) ?? src

    if (
      !resolvedSrc?.startsWith('/assets/') &&
      !resolvedSrc?.startsWith('/api/')
    ) {
      setVersionedSrc(resolvedSrc)
      return
    }

    const checkVersion = async () => {
      try {
        const v = Date.now()
        const prefixedSrc = withBasePath(resolvedSrc)
        const url = new URL(prefixedSrc, location.origin)
        url.searchParams.set('v', v.toString())
        setVersionedSrc(url.toString())
      } catch {
        setVersionedSrc(resolvedSrc) // fallback
      }
    }

    checkVersion()
  }, [resolveAssetUrl, src])

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
