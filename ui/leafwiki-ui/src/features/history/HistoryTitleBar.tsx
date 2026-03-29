import { useAppMode } from '@/lib/useAppMode'
import { useViewerStore } from '../viewer/viewer'

export function HistoryTitleBar() {
  const appMode = useAppMode()
  const page = useViewerStore((state) => state.page)

  if (appMode !== 'history' || !page) {
    return null
  }

  return (
    <div className="history-title-bar" data-testid="history-title-bar">
      <span className="history-title-bar__mode">Revision</span>
      <span className="history-title-bar__title">{page.title}</span>
      <span className="history-title-bar__slug">{page.slug}</span>
    </div>
  )
}
