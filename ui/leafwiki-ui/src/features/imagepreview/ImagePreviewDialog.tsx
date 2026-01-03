/* eslint-disable react-hooks/set-state-in-effect */
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { DIALOG_IMAGE_PREVIEW } from '@/lib/registries'
import { useDialogsStore } from '@/stores/dialogs'
import { HotKeyDefinition, useHotKeysStore } from '@/stores/hotkeys'
import { useEffect, useMemo, useState } from 'react'

type Props = {
  src: string
  alt?: string
}

export function ImagePreviewDialog({ src, alt }: Props) {
  const closeDialog = useDialogsStore((s) => s.closeDialog)
  const registerHotkey = useHotKeysStore((s) => s.registerHotkey)
  const unregisterHotkey = useHotKeysStore((s) => s.unregisterHotkey)
  const open = useDialogsStore((s) => s.dialogType === DIALOG_IMAGE_PREVIEW)
  const [natural, setNatural] = useState<{ w: number; h: number } | null>(null)

  useEffect(() => {
    if (!open) {
      return
    }
    const cancelHotkey: HotKeyDefinition = {
      keyCombo: 'Escape',
      enabled: true,
      mode: ['dialog'],
      action: () => {
        closeDialog()
      },
    }

    setNatural(null)
    registerHotkey(cancelHotkey)

    return () => {
      unregisterHotkey(cancelHotkey.keyCombo)
    }
  }, [open, registerHotkey, unregisterHotkey, closeDialog])

  const dialogWidth = useMemo(() => {
    if (!natural) return 'min(95vw, 900px)'

    const paddingAndChrome = 12 * 2 + 24 // px-6 left/right + slack
    const target = natural.w + paddingAndChrome

    const minW = 360
    const maxW = 1400

    const px = Math.max(minW, Math.min(target, maxW))
    return `min(95vw, ${px}px)`
  }, [natural])

  return (
    <Dialog open={open} onOpenChange={(isOpen) => !isOpen && closeDialog()}>
      <DialogContent
        className="p-0"
        style={{ width: dialogWidth, maxWidth: '95vw' }}
        onEscapeKeyDown={(e: KeyboardEvent) => {
          // The hotkey handler will take care of this
          e.preventDefault()
        }}
      >
        <DialogHeader className="px-6 pt-6">
          <DialogTitle className="mt-3 flex flex-wrap items-baseline gap-2">
            <span className="truncate">{alt ?? ''}</span>
            {natural ? (
              <span className="text-muted-foreground text-sm font-normal">
                ({natural.w}×{natural.h}px)
              </span>
            ) : null}

            <a
              href={src}
              target="_blank"
              rel="noreferrer noopener"
              className="text-muted-foreground hover:text-foreground ml-auto text-sm underline"
              onClick={(e) => e.stopPropagation()}
            >
              Open in new tab
            </a>
          </DialogTitle>
        </DialogHeader>

        <div className="flex justify-center px-6 pb-6">
          <img
            src={src}
            alt={alt}
            onLoad={(e) => {
              const img = e.currentTarget
              setNatural({ w: img.naturalWidth, h: img.naturalHeight })
            }}
            // Never scale up images beyond their natural size
            className="bg-muted h-auto max-h-[75vh] w-auto max-w-full rounded-lg object-contain"
          />
        </div>
      </DialogContent>
    </Dialog>
  )
}
