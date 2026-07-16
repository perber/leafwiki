import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { deferStateUpdate } from '@/lib/deferState'
import { searchFlatPageSearchItems } from '@/lib/pageSearch'
import { DIALOG_LINK_INSERT } from '@/lib/registries'
import { createHotkeyDefinition } from '@/lib/shortcuts/shortcutCatalog'
import { useDialogsStore } from '@/stores/dialogs'
import { HotKeyDefinition, useHotKeysStore } from '@/stores/hotkeys'
import { useTreeStore } from '@/stores/tree'
import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import type { MarkdownEditorRef } from './MarkdownEditor'

const EXTERNAL_PREFIXES = ['http', 'https', 'mailto']
const MAX_SUGGESTIONS = 8

function isExternalUrl(value: string) {
  const normalized = value.trimStart().toLowerCase()
  return EXTERNAL_PREFIXES.some(
    (p) => normalized === p || normalized.startsWith(`${p}:`),
  )
}

type LinkInsertDialogProps = {
  editorRef: React.RefObject<MarkdownEditorRef>
  selectedText: string
}

export function LinkInsertDialog({
  editorRef,
  selectedText,
}: LinkInsertDialogProps) {
  const { t } = useTranslation('editor')
  const { t: tCommon } = useTranslation('common')
  const closeDialog = useDialogsStore((s) => s.closeDialog)
  const open = useDialogsStore((s) => s.dialogType === DIALOG_LINK_INSERT)
  const registerHotkey = useHotKeysStore((s) => s.registerHotkey)
  const unregisterHotkey = useHotKeysStore((s) => s.unregisterHotkey)
  const flatPages = useTreeStore((s) => s.flatPages)

  const [text, setText] = useState(selectedText)
  const [url, setUrl] = useState('')
  const [urlFocused, setUrlFocused] = useState(false)
  const [highlightedIndex, setHighlightedIndex] = useState(0)

  const urlInputRef = useRef<HTMLInputElement>(null)
  const textInputRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    if (open) {
      deferStateUpdate(() => {
        setText(selectedText)
        setUrl('')
        setHighlightedIndex(0)
      })
      requestAnimationFrame(() => {
        if (selectedText) {
          urlInputRef.current?.focus()
        } else {
          textInputRef.current?.focus()
        }
      })
    }
  }, [open, selectedText])

  const query = url.startsWith('/') ? url.slice(1) : url
  const showSuggestions = urlFocused && !isExternalUrl(url)
  const suggestions = showSuggestions
    ? searchFlatPageSearchItems(flatPages, query, MAX_SUGGESTIONS)
    : []

  useEffect(() => {
    deferStateUpdate(() => {
      setHighlightedIndex(0)
    })
  }, [url])

  const selectSuggestion = (path: string, title: string) => {
    setUrl(`/${path}`)
    if (!text) setText(title)
    setUrlFocused(false)
  }

  const handleConfirm = () => {
    editorRef.current?.replaceSelection(`[${text}](${url})`)
    closeDialog()
  }

  useEffect(() => {
    const cancelHotkey: HotKeyDefinition = createHotkeyDefinition(
      'dialog.close',
      () => {
        if (open) closeDialog()
      },
    )
    registerHotkey(cancelHotkey)
    return () => unregisterHotkey(cancelHotkey.keyCombo)
  }, [open, closeDialog, registerHotkey, unregisterHotkey])

  return (
    <Dialog
      open={open}
      onOpenChange={(o) => {
        if (!o) closeDialog()
      }}
    >
      <DialogContent className="max-w-sm">
        <DialogHeader>
          <DialogTitle>{t('linkInsert.title')}</DialogTitle>
        </DialogHeader>
        <div className="flex flex-col gap-4">
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="link-text">{t('linkInsert.displayText')}</Label>
            <Input
              id="link-text"
              ref={textInputRef}
              value={text}
              onChange={(e) => setText(e.target.value)}
              placeholder={t('linkInsert.displayTextPlaceholder')}
              onKeyDown={(e) => {
                if (e.key === 'Enter') handleConfirm()
              }}
            />
          </div>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="link-url">{t('linkInsert.url')}</Label>
            <div className="relative">
              <Input
                id="link-url"
                ref={urlInputRef}
                value={url}
                onChange={(e) => setUrl(e.target.value)}
                placeholder={t('linkInsert.urlPageSearchPlaceholder')}
                onFocus={() => setUrlFocused(true)}
                onBlur={() => setTimeout(() => setUrlFocused(false), 150)}
                onKeyDown={(e) => {
                  if (suggestions.length > 0) {
                    if (e.key === 'ArrowDown') {
                      e.preventDefault()
                      setHighlightedIndex((i) =>
                        Math.min(i + 1, suggestions.length - 1),
                      )
                      return
                    }
                    if (e.key === 'ArrowUp') {
                      e.preventDefault()
                      setHighlightedIndex((i) => Math.max(i - 1, 0))
                      return
                    }
                    if (e.key === 'Enter') {
                      e.preventDefault()
                      const s = suggestions[highlightedIndex]
                      if (s) selectSuggestion(s.path, s.title)
                      return
                    }
                  }
                  if (e.key === 'Enter') handleConfirm()
                }}
                autoComplete="off"
              />
              {suggestions.length > 0 && (
                <ul className="border-border bg-surface absolute top-full right-0 left-0 z-50 mt-1 max-h-48 overflow-y-auto rounded-md border shadow-md">
                  {suggestions.map((s, i) => (
                    <li key={s.id}>
                      <button
                        type="button"
                        className={`w-full px-3 py-2 text-left text-sm ${i === highlightedIndex ? 'bg-accent' : 'hover:bg-accent'}`}
                        onMouseDown={(e) => {
                          e.preventDefault()
                          selectSuggestion(s.path, s.title)
                        }}
                        onMouseEnter={() => setHighlightedIndex(i)}
                      >
                        <div className="font-medium">{s.title}</div>
                        <div className="text-muted-foreground text-xs">
                          {s.breadcrumb}
                        </div>
                      </button>
                    </li>
                  ))}
                </ul>
              )}
            </div>
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={closeDialog}>
            {tCommon('actions.cancel')}
          </Button>
          <Button onClick={handleConfirm}>{tCommon('actions.insert')}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
