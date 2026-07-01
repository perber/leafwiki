import { scrollToHeadlineHash } from '@/lib/scrollToHeadline'
import { cn } from '@/lib/utils'
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
  // When the parent provides activeId, skip internal scroll spy (no duplicate listener).
  const internalActiveId = useTocScrollSpy(
    externalActiveId !== undefined ? [] : entries,
  )
  const activeId =
    externalActiveId !== undefined ? externalActiveId : internalActiveId

  return (
    <nav
      className="page-viewer__toc-panel"
      aria-label={t('toc.title')}
      data-testid="toc-side-panel"
    >
      <div className="page-viewer__toc-panel-inner">
        <p className="page-viewer__toc-panel-title">{t('toc.onThisPage')}</p>
        <ul>
          {entries.map((entry) => (
            <li key={entry.id}>
              <button
                className={cn(
                  'page-viewer__toc-panel-entry',
                  getIndentClass(entry.level),
                  activeId === entry.id &&
                    'page-viewer__toc-panel-entry--active',
                )}
                onClick={() =>
                  scrollToHeadlineHash(`#${encodeURIComponent(entry.id)}`, {
                    waitForStableLayout: false,
                  })
                }
                data-testid={`toc-entry-${entry.id}`}
              >
                {entry.text}
              </button>
            </li>
          ))}
        </ul>
      </div>
    </nav>
  )
}
