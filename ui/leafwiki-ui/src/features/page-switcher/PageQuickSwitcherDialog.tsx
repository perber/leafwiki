import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { deferStateUpdate } from '@/lib/deferState'
import { DIALOG_PAGE_QUICK_SWITCHER } from '@/lib/registries'
import { cn } from '@/lib/utils'
import { useDialogsStore } from '@/stores/dialogs'
import { useTreeStore } from '@/stores/tree'
import { File, FolderTree } from 'lucide-react'
import { useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import {
  buildQuickSwitcherItems,
  searchQuickSwitcherItems,
} from './pageQuickSwitcher'

export function PageQuickSwitcherDialog() {
  const navigate = useNavigate()
  const closeDialog = useDialogsStore((state) => state.closeDialog)
  const isOpen = useDialogsStore(
    (state) => state.dialogType === DIALOG_PAGE_QUICK_SWITCHER,
  )
  const tree = useTreeStore((state) => state.tree)
  const openAncestorsForPath = useTreeStore(
    (state) => state.openAncestorsForPath,
  )

  const [query, setQuery] = useState('')
  const [activeIndex, setActiveIndex] = useState(0)
  const inputRef = useRef<HTMLInputElement | null>(null)
  const resultRefs = useRef<(HTMLButtonElement | null)[]>([])

  const items = useMemo(() => buildQuickSwitcherItems(tree), [tree])
  const results = useMemo(
    () => searchQuickSwitcherItems(items, query, 20),
    [items, query],
  )
  const clampedActiveIndex =
    results.length === 0 ? 0 : Math.min(activeIndex, results.length - 1)

  useEffect(() => {
    if (!isOpen) {
      deferStateUpdate(() => {
        setQuery('')
        setActiveIndex(0)
      })
      return
    }

    const frame = requestAnimationFrame(() => {
      inputRef.current?.focus()
      inputRef.current?.select()
    })

    return () => cancelAnimationFrame(frame)
  }, [isOpen])

  useEffect(() => {
    deferStateUpdate(() => {
      setActiveIndex(0)
    })
  }, [query])

  useEffect(() => {
    resultRefs.current = resultRefs.current.slice(0, results.length)
  }, [results])

  useEffect(() => {
    if (!isOpen) return

    resultRefs.current[clampedActiveIndex]?.scrollIntoView({
      block: 'nearest',
    })
  }, [clampedActiveIndex, isOpen, results])

  const openResult = (path: string) => {
    queueMicrotask(() => {
      openAncestorsForPath(path)
      navigate(`/${path}`)
      closeDialog()
    })
  }

  return (
    <Dialog
      open={isOpen}
      onOpenChange={(open) => {
        if (!open) queueMicrotask(() => closeDialog())
      }}
    >
      <DialogContent className="max-h-[85dvh] gap-0 overflow-hidden p-0 sm:max-w-2xl">
        <DialogHeader className="border-b px-4 pt-4 pb-3">
          <DialogTitle>Go to page</DialogTitle>
          <DialogDescription>
            Search existing pages by title, path, or breadcrumb.
          </DialogDescription>
        </DialogHeader>

        <div className="border-b px-4 py-3">
          <Input
            ref={inputRef}
            defaultValue=""
            placeholder="Type a page title…"
            aria-label="Search pages"
            role="combobox"
            aria-haspopup="listbox"
            aria-activedescendant={
              results.length > 0 ? results[clampedActiveIndex]?.id : undefined
            }
            aria-controls={
              results.length > 0 ? 'page-quick-switcher-results' : undefined
            }
            aria-expanded={results.length > 0}
            aria-autocomplete="list"
            onChange={(e) => {
              const nextValue = e.target.value
              deferStateUpdate(() => {
                setQuery(nextValue)
              })
            }}
            onKeyDown={(e) => {
              if (e.key === 'ArrowDown') {
                e.preventDefault()
                deferStateUpdate(() => {
                  setActiveIndex((current) =>
                    Math.min(current + 1, Math.max(results.length - 1, 0)),
                  )
                })
              }

              if (e.key === 'ArrowUp') {
                e.preventDefault()
                deferStateUpdate(() => {
                  setActiveIndex((current) => Math.max(current - 1, 0))
                })
              }

              if (e.key === 'Enter') {
                const activeItem = results[clampedActiveIndex]
                if (!activeItem) return

                e.preventDefault()
                openResult(activeItem.path)
              }
            }}
          />
        </div>

        <div className="custom-scrollbar max-h-[70dvh] overflow-y-auto px-2 pb-2">
          {results.length === 0 ? (
            <div className="text-muted-foreground px-2 py-6 text-sm">
              No matching page found.
            </div>
          ) : (
            <ul
              id="page-quick-switcher-results"
              role="listbox"
              aria-label="Matching pages"
              className="space-y-1"
            >
              {results.map((item, index) => {
                const active = index === clampedActiveIndex
                const Icon = item.kind === 'section' ? FolderTree : File

                return (
                  <li key={item.id}>
                    <button
                      id={item.id}
                      ref={(element) => {
                        resultRefs.current[index] = element
                      }}
                      type="button"
                      role="option"
                      aria-selected={active}
                      tabIndex={-1}
                      className={cn(
                        'flex w-full items-start gap-3 rounded-md px-3 py-2 text-left',
                        active
                          ? 'bg-accent text-accent-foreground'
                          : 'hover:bg-accent/60',
                      )}
                      onMouseEnter={() =>
                        deferStateUpdate(() => {
                          setActiveIndex(index)
                        })
                      }
                      onClick={() => openResult(item.path)}
                    >
                      <Icon className="mt-0.5 h-4 w-4 shrink-0" />
                      <span className="min-w-0 flex-1">
                        <span className="block truncate text-sm font-medium">
                          {item.title}
                        </span>
                        <span className="text-muted-foreground block truncate text-xs">
                          {item.breadcrumb}
                        </span>
                        <span className="text-muted-foreground/80 block truncate text-xs">
                          /{item.path}
                        </span>
                      </span>
                    </button>
                  </li>
                )
              })}
            </ul>
          )}
        </div>
      </DialogContent>
    </Dialog>
  )
}
