import { scrollToHeadlineHash } from '@/lib/scrollToHeadline'
import { cn } from '@/lib/utils'
import { useTocPanelStore } from '@/stores/tocPanel'
import { PanelRightClose, PanelRightOpen } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import type { TocEntry } from './extractTocEntries'
import { useTocScrollSpy } from './useTocScrollSpy'

type Props = {
  entries: TocEntry[]
  activeId?: string | null
}

function getIndentClass(level: number): string {
  if (level <= 1) return ''
  if (level === 2) return 'pl-3'
  return 'pl-6'
}

export function TocSidePanel({ entries, activeId: externalActiveId }: Props) {
  const { t } = useTranslation('viewer')
  const collapsed = useTocPanelStore((state) => state.collapsed)
  const setCollapsed = useTocPanelStore((state) => state.setCollapsed)
  // When the parent provides activeId, skip internal scroll spy (no duplicate listener).
  const internalActiveId = useTocScrollSpy(
    externalActiveId !== undefined ? [] : entries,
  )
  const activeId =
    externalActiveId !== undefined ? externalActiveId : internalActiveId

  // The toggle button is a single, always-mounted element so it never moves
  // between the collapsed and expanded states — only its icon/label swaps.
  // The panel's own width never changes either (see .page-viewer__toc-panel
  // in index.css), so collapsing never shifts the main content column.
  return (
    <nav
      className={cn(
        'page-viewer__toc-panel',
        collapsed && 'page-viewer__toc-panel--collapsed',
      )}
      aria-label={t('toc.title')}
      data-testid="toc-side-panel"
    >
      <div className="page-viewer__toc-panel-header">
        <p
          className={cn(
            'page-viewer__toc-panel-title',
            collapsed && 'page-viewer__toc-panel-hidden',
          )}
          aria-hidden={collapsed}
        >
          {t('toc.onThisPage')}
        </p>
        <button
          type="button"
          className="page-viewer__toc-panel-toggle-button"
          onClick={() => setCollapsed(!collapsed)}
          title={collapsed ? t('toc.expand') : t('toc.collapse')}
          aria-label={collapsed ? t('toc.expand') : t('toc.collapse')}
          data-testid={
            collapsed ? 'toc-side-panel-expand' : 'toc-side-panel-collapse'
          }
        >
          {collapsed ? (
            <PanelRightOpen size={14} />
          ) : (
            <PanelRightClose size={14} />
          )}
        </button>
      </div>
      <ul
        className={cn(
          'page-viewer__toc-panel-list',
          collapsed && 'page-viewer__toc-panel-hidden',
        )}
        aria-hidden={collapsed}
        data-testid="toc-side-panel-list"
      >
        {entries.map((entry) => (
          <li key={entry.id}>
            <button
              className={cn(
                'page-viewer__toc-panel-entry',
                getIndentClass(entry.level),
                activeId === entry.id && 'page-viewer__toc-panel-entry--active',
              )}
              onClick={() =>
                scrollToHeadlineHash(`#${encodeURIComponent(entry.id)}`, {
                  waitForStableLayout: false,
                })
              }
              data-testid={`toc-entry-${entry.id}`}
              tabIndex={collapsed ? -1 : 0}
            >
              {entry.text}
            </button>
          </li>
        ))}
      </ul>
    </nav>
  )
}
