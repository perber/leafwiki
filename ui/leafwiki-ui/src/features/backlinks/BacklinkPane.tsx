import { useAppMode } from '@/lib/useAppMode'
import { ArrowUpRightFromSquare } from 'lucide-react'
import { useEffect } from 'react'
import { Link } from 'react-router-dom'
import { usePageEditorStore } from '../editor/pageEditor'
import { useViewerStore } from '../viewer/viewer'
import { useBacklinksStore } from './backlinks'
import { useOutgoingLinksStore } from './outgoinglinks'

export function BacklinkPane() {
  const loading = useBacklinksStore((s) => s.loading)
  const backlinks = useBacklinksStore((s) => s.backlinks)
  const outgoingLinks = useOutgoingLinksStore((s) => s.outgoing)
  const outgoingLinksLoading = useOutgoingLinksStore((s) => s.loading)
  const fetchPageBacklinks = useBacklinksStore((s) => s.fetchPageBacklinks)
  const fetchOutgoingLinks = useOutgoingLinksStore(
    (s) => s.fetchPageOutgoingLinks,
  )
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
    if (currentPageID) fetchOutgoingLinks(currentPageID)
  }, [
    appMode,
    fetchPageBacklinks,
    fetchOutgoingLinks,
    viewerPageID,
    editorPageID,
  ])

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
      <div className="backlinks__header">
        <h2 className="mt-4 mb-2 text-lg font-medium">Page references</h2>
      </div>
      <div className="backlinks__content">
        {outgoingLinks && outgoingLinks.length > 0 ? (
          <ul>
            {outgoingLinks.map((ol) => (
              <li key={ol.to_page_id} className="backlinks__item">
                <Link to={ol.to_path}>{ol.to_page_title}</Link>
                <ArrowUpRightFromSquare
                  className="backlinks__item_icon"
                  size={16}
                />
              </li>
            ))}
          </ul>
        ) : outgoingLinksLoading ? (
          <p>Loading...</p>
        ) : (
          <p>No outgoing links found.</p>
        )}
      </div>
    </div>
  )
}
