/* eslint-disable react-hooks/set-state-in-effect */
import { DIALOG_IMAGE_PREVIEW } from '@/lib/registries'
import { useDialogsStore } from '@/stores/dialogs'
import { useEffect, useState } from 'react'

type Props = React.ImgHTMLAttributes<HTMLImageElement>

function shouldOpenPreview(e: React.MouseEvent<HTMLImageElement>) {
  if (e.button !== 0) return false
  if (e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return false
  return true
}

function shouldOpenInNewTab(e: React.MouseEvent<HTMLImageElement>) {
  return e.button === 0 && (e.metaKey || e.ctrlKey)
}

export function MarkdownImage({ src = '', style, alt, ...rest }: Props) {
  const [versionedSrc, setVersionedSrc] = useState(src)
  const openDialog = useDialogsStore((s) => s.openDialog)

  useEffect(() => {
    if (!src?.startsWith('/assets/')) {
      setVersionedSrc(src)
      return
    }

    const checkVersion = async () => {
      try {
        const v = Date.now()
        const url = new URL(src, location.origin)
        url.searchParams.set('v', v.toString())
        setVersionedSrc(url.toString())
      } catch {
        setVersionedSrc(src) // fallback
      }
    }

    checkVersion()
  }, [src])

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
