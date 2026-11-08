import { cn } from '@/lib/utils'
import { useTocPanelStore } from '@/stores/tocPanel'
import { PanelRightClose, PanelRightOpen } from 'lucide-react'
import { Fragment } from 'react'
import { useTranslation } from 'react-i18next'
import {
  panelTitleKey,
  visiblePanelSections,
  type PagePanelItem,
  type PagePanelSection,
} from './pagePanelSection'

type Props = {
  sections: PagePanelSection[]
}

function getIndentClass(level: number): string {
  if (level <= 1) return ''
  if (level === 2) return 'pl-3'
  return 'pl-6'
}

export function TocSidePanel({ sections }: Props) {
  const { t } = useTranslation('viewer')
  const collapsed = useTocPanelStore((state) => state.collapsed)
  const setCollapsed = useTocPanelStore((state) => state.setCollapsed)
  const tabIndex = collapsed ? -1 : 0

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
          {t(panelTitleKey(sections))}
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
          'custom-scrollbar',
          collapsed && 'page-viewer__toc-panel-hidden',
        )}
        aria-hidden={collapsed}
        data-testid="toc-side-panel-list"
      >
        {visiblePanelSections(sections).map((section, index) => (
          <Fragment key={section.id}>
            {/* The first section is titled by the panel header. */}
            {index > 0 && (
              <li>
                <p
                  className={cn(
                    'page-viewer__toc-panel-title mt-3',
                    collapsed && 'page-viewer__toc-panel-hidden',
                  )}
                  aria-hidden={collapsed}
                >
                  {t(section.titleKey)}
                </p>
              </li>
            )}
            {section.items.map((item) => (
              <li key={item.key}>
                <SidePanelItem item={item} tabIndex={tabIndex} />
              </li>
            ))}
          </Fragment>
        ))}
      </ul>
    </nav>
  )
}

function SidePanelItem({
  item,
  tabIndex,
}: {
  item: PagePanelItem
  tabIndex: number
}) {
  if (item.kind === 'download') {
    return (
      <a
        href={item.href}
        download={item.fileName}
        className="page-viewer__toc-panel-entry"
        title={item.label}
        data-testid={`toc-${item.testId}`}
        tabIndex={tabIndex}
      >
        {item.label}
      </a>
    )
  }

  const className = cn(
    'page-viewer__toc-panel-entry',
    getIndentClass(item.level ?? 1),
    item.active && 'page-viewer__toc-panel-entry--active',
  )

  // Without onSelect the entry is plain text, like a non-clickable dropdown
  // entry: no dead button in the tab order, no pointer cursor.
  if (!item.onSelect) {
    return (
      <span
        className={cn(className, 'cursor-default')}
        title={item.label}
        data-testid={`toc-${item.testId}`}
      >
        {item.label}
      </span>
    )
  }

  return (
    <button
      className={className}
      onClick={item.onSelect}
      title={item.label}
      data-testid={`toc-${item.testId}`}
      tabIndex={tabIndex}
    >
      {item.label}
    </button>
  )
}
