import { useAppMode } from '@/lib/useAppMode'
import { ArrowUpRightFromSquare } from 'lucide-react'
import { useEffect } from 'react'
import { Link } from 'react-router-dom'
import { usePageEditorStore } from '../editor/pageEditor'
import { useViewerStore } from '../viewer/viewer'
import { useBacklinksStore } from './backlinks'

export function BacklinkPane() {
  const loading = useBacklinksStore((s) => s.loading)
  const backlinks = useBacklinksStore((s) => s.backlinks)
  const fetchPageBacklinks = useBacklinksStore((s) => s.fetchPageBacklinks)
  const appMode = useAppMode()
  /**
   * Kinda hacky way to get the page ID depending on the app mode
   * In view mode, we get it from the viewer store
   * In edit mode, we get it from the page editor store
   */
  const viewerPageID = useViewerStore((s) => s.page?.id)
  const editorPageID = usePageEditorStore((s) => s.page?.id)

  useEffect(() => {
    const currentPageID = appMode === 'view' ? viewerPageID : editorPageID
    if (currentPageID) fetchPageBacklinks(currentPageID)
  }, [appMode, fetchPageBacklinks, viewerPageID, editorPageID])

  return (
    <div className="backlinks__pane">
      <div className="backlinks__header">
        <h2 className="mb-2 text-lg font-medium">Page is referenced by</h2>
      </div>
      <div className="backlinks__content">
        {backlinks && backlinks.length > 0 ? (
          <ul>
            {backlinks.map((bl) => (
              <li key={bl.from_page_id} className="backlinks__item">
                <Link to={bl.from_path}>{bl.from_title}</Link>
                <ArrowUpRightFromSquare
                  className="backlinks__item_icon"
                  size={16}
                />
              </li>
            ))}
          </ul>
        ) : loading ? (
          <p>Loading...</p>
        ) : (
          <p>No backlinks found.</p>
        )}
      </div>
    </div>
  )
}
