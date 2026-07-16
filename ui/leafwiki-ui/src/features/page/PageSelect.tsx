import { Input } from '@/components/ui/input'
import { NODE_KIND_SECTION, PageNode } from '@/lib/api/pages'
import { deferStateUpdate } from '@/lib/deferState'
import { cn } from '@/lib/utils'
import { useTreeStore } from '@/stores/tree'
import { File, FolderTree } from 'lucide-react'
import { useEffect, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

const LISTBOX_ID = 'page-select-results'

type FlatPage = {
  id: string
  title: string
  depth: number
  kind: 'page' | 'section'
}

function flattenTree(node: PageNode, depth = 1): FlatPage[] {
  const result: FlatPage[] = [
    { id: node.id, title: node.title, depth, kind: node.kind },
  ]
  for (const child of node.children || []) {
    result.push(...flattenTree(child, depth + 1))
  }
  return result
}

export function PageSelect({
  pageID,
  onChange,
  autoFocus = false,
}: {
  pageID: string
  onChange: (id: string) => void
  autoFocus?: boolean
}) {
  const { t } = useTranslation('page')
  const { tree } = useTreeStore()
  const [search, setSearch] = useState('')
  const [activeIndex, setActiveIndex] = useState(0)
  const resultRefs = useRef<(HTMLButtonElement | null)[]>([])

  const flatPages = useMemo<FlatPage[]>(() => {
    if (!tree) return []
    const pages: FlatPage[] = [
      {
        id: 'root',
        title: t('pageSelect.topLevel'),
        depth: 0,
        kind: NODE_KIND_SECTION,
      },
    ]
    for (const child of tree.children || []) {
      pages.push(...flattenTree(child))
    }
    return pages
  }, [tree, t])

  const filtered = useMemo(() => {
    const trimmedSearch = search.trim().toLowerCase()
    return trimmedSearch
      ? flatPages.filter((p) => p.title.toLowerCase().includes(trimmedSearch))
      : flatPages
  }, [flatPages, search])

  const clampedActiveIndex =
    filtered.length === 0 ? 0 : Math.min(activeIndex, filtered.length - 1)

  useEffect(() => {
    deferStateUpdate(() => setActiveIndex(0))
  }, [search])

  useEffect(() => {
    resultRefs.current = resultRefs.current.slice(0, filtered.length)
  }, [filtered])

  useEffect(() => {
    resultRefs.current[clampedActiveIndex]?.scrollIntoView({ block: 'nearest' })
  }, [clampedActiveIndex, filtered])

  return (
    <div className="flex flex-col gap-2">
      <Input
        placeholder={t('pageSelect.placeholder')}
        value={search}
        autoFocus={autoFocus}
        role="combobox"
        aria-haspopup="listbox"
        aria-expanded={filtered.length > 0}
        aria-controls={filtered.length > 0 ? LISTBOX_ID : undefined}
        aria-activedescendant={
          filtered.length > 0 ? filtered[clampedActiveIndex]?.id : undefined
        }
        aria-autocomplete="list"
        aria-label={t('pageSelect.searchAriaLabel')}
        onChange={(e) => setSearch(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === 'ArrowDown') {
            e.preventDefault()
            deferStateUpdate(() =>
              setActiveIndex((i) =>
                Math.min(i + 1, Math.max(filtered.length - 1, 0)),
              ),
            )
          }
          if (e.key === 'ArrowUp') {
            e.preventDefault()
            deferStateUpdate(() => setActiveIndex((i) => Math.max(i - 1, 0)))
          }
          if (e.key === 'Enter') {
            const active = filtered[clampedActiveIndex]
            if (!active) return
            e.preventDefault()
            e.stopPropagation()
            onChange(active.id)
          }
        }}
      />
      <div className="custom-scrollbar max-h-56 overflow-y-auto rounded-md border">
        {filtered.length === 0 && (
          <p className="text-muted-foreground px-3 py-2 text-sm">
            {t('pageSelect.noResults')}
          </p>
        )}
        <ul id={LISTBOX_ID} role="listbox" aria-label={t('pageSelect.listAriaLabel')}>
          {filtered.map((page, index) => {
            const active = index === clampedActiveIndex
            const Icon = page.kind === NODE_KIND_SECTION ? FolderTree : File
            return (
              <li key={page.id}>
                <button
                  id={page.id}
                  ref={(el) => {
                    resultRefs.current[index] = el
                  }}
                  type="button"
                  role="option"
                  aria-selected={page.id === pageID}
                  tabIndex={-1}
                  className={cn(
                    'flex w-full items-center gap-2 py-2 pr-3 text-left text-sm',
                    page.id === pageID && 'bg-brand/10 text-brand font-medium',
                    active ? 'bg-accent' : 'hover:bg-accent/60',
                  )}
                  style={{ paddingLeft: `${page.depth * 14 + 12}px` }}
                  onMouseEnter={() =>
                    deferStateUpdate(() => setActiveIndex(index))
                  }
                  onClick={() => onChange(page.id)}
                >
                  <Icon className="h-4 w-4 shrink-0" />
                  {page.title}
                </button>
              </li>
            )
          })}
        </ul>
      </div>
    </div>
  )
}
