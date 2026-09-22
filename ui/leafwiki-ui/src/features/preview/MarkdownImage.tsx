import { versionAssetSrc } from '@/lib/assetSrc'
import { DIALOG_IMAGE_PREVIEW } from '@/lib/registries'
import { useDialogsStore } from '@/stores/dialogs'
import { useMemo } from 'react'

type MarkdownImageProps = React.ImgHTMLAttributes<HTMLImageElement> & {
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
  width,
  resolveAssetUrl,
  ...rest
}: MarkdownImageProps) {
  const openDialog = useDialogsStore((s) => s.openDialog)
  const resolvedSrc = useMemo(
    () => resolveAssetUrl?.(src) ?? src,
    [resolveAssetUrl, src],
  )
  const versionedSrc = useMemo(
    () => versionAssetSrc(resolvedSrc),
    [resolvedSrc],
  )

  return (
    <img
      src={versionedSrc}
      alt={alt}
      style={{
        // Tailwind's preflight resets images to `display: block`, which forces
        // a line break between an image and any text that follows it on the
        // same Markdown line (`![alt](img) text`). Keep Markdown images inline
        // so trailing/leading text stays on the same line (#1471).
        display: 'inline-block',
        // Tailwind Typography's `prose img` rule adds a 2em top/bottom margin
        // sized for a standalone block-level figure. On an inline-block image
        // that margin never collapses with the surrounding paragraph's own
        // margin, so it stacked on top of it and produced an oversized gap
        // around "image + text" lines (and before a wrapped second line of
        // the same paragraph). Let the paragraph's margin be the only source
        // of vertical spacing (#1524).
        marginTop: 0,
        marginBottom: 0,
        ...style,
        cursor: 'zoom-in',
        ...(width
          ? {
              width,
              height: 'auto',
            }
          : {}),
      }}
      draggable={false}
      {...rest}
      onClick={(e) => {
        rest.onClick?.(e)

        // Images wrapped in a link (Markdown `[![alt](img)](url)` or raw
        // `<a><img></a>`) should follow the link instead of opening the
        // preview dialog.
        if (e.currentTarget.closest('a')) {
          return
        }

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
