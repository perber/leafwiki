import { useConfigStore } from '@/stores/config'
import { Link2Off, Paperclip } from 'lucide-react'
import { useEffect } from 'react'
import { Link } from 'react-router-dom'
import { useViewerStore } from '../viewer/viewer'
import { useLinkStatusStore } from './linkstatus_store'

export function BacklinkInfo() {
  const pageID = useViewerStore((s) => s.page?.id)
  const hideLinkMetadataSection = useConfigStore((s) => s.hideLinkMetadataSection)

  const loading = useLinkStatusStore((s) => s.loading)
  const error = useLinkStatusStore((s) => s.error)
  const status = useLinkStatusStore((s) => s.status)

  const fetchLinkStatusForPage = useLinkStatusStore(
    (s) => s.fetchLinkStatusForPage,
  )
  const clear = useLinkStatusStore((s) => s.clear)

  useEffect(() => {
    // Clear link status when there is no page or the link metadata section is hidden,
    // and fetch link status when a page is selected and the section is visible.
    if (!pageID || hideLinkMetadataSection) {
      clear()
      return
    }
    fetchLinkStatusForPage(pageID)
  }, [fetchLinkStatusForPage, pageID, clear, hideLinkMetadataSection])


  // if the link impact feature is disabled, do not show anything - return null
  if (hideLinkMetadataSection) return null
  const backlinks = status?.backlinks ?? []
  const brokenIncoming = status?.broken_incoming ?? []
  const brokenOutgoings = status?.broken_outgoings ?? []

  // Render backlinks and broken links only if a link is present
  if (backlinks.length === 0 && brokenIncoming.length === 0 && brokenOutgoings.length === 0) {
    return null
  }

  return (
    <div className="backlinks__pane">
      <div className="backlinks__content">
        {/* Referenced by */}
        <div className="backlinks__group">
          <div className="backlinks__group-title">
            Referenced by{' '}
            <span className="backlinks__badge">{backlinks.length}</span>
          </div>

          {backlinks.length > 0 ? (
            <ul>
              {backlinks.map((bl) => (
                <li key={bl.from_page_id} className="backlinks__item">
                  <Link to={bl.from_path}>
                    <Paperclip size={16} className="backlinks__icon" />{' '}
                    {bl.from_title}
                  </Link>
                </li>
              ))}
            </ul>
          ) : loading ? (
            <p className="backlinks__empty">Loading…</p>
          ) : (
            <p className="backlinks__empty">No pages reference this page.</p>
          )}
        </div>

        {/* Broken links */}
        <div className="backlinks__group">
          <div className="backlinks__group-title">
            Broken links{' '}
            <span className="backlinks__badge">
              {brokenIncoming.length + brokenOutgoings.length}
            </span>
          </div>

          {error && !loading ? (
            <p className="page-viewer__error">Error: {error}</p>
          ) : null}

          {loading ? (
            <p className="backlinks__empty">Loading…</p>
          ) : brokenIncoming.length + brokenOutgoings.length === 0 ? (
            <p className="backlinks__empty">No broken links.</p>
          ) : (
            <>
              {brokenOutgoings.length > 0 && (
                <>
                  <div className="backlinks__subgroup-title">
                    This page links to missing targets
                  </div>
                  <ul>
                    {brokenOutgoings.map((ol) => (
                      <li
                        key={ol.to_path}
                        className="backlinks__item backlinks__item--broken"
                      >
                        <span className="backlinks__icon-inline">
                          <Link2Off size={16} className="backlinks__icon" />
                        </span>
                        <span className="ml-1">
                          {ol.to_page_title ? `${ol.to_page_title} ` : ''}
                          <span className="text-muted font-mono text-xs">
                            {ol.to_path}
                          </span>
                        </span>
                      </li>
                    ))}
                  </ul>
                </>
              )}

              {brokenIncoming.length > 0 && (
                <>
                  <div className="backlinks__subgroup-title">
                    Pages linking to an old path
                  </div>
                  <ul>
                    {brokenIncoming.map((bl) => (
                      <li
                        key={bl.from_page_id}
                        className="backlinks__item backlinks__item--broken"
                      >
                        <Link to={bl.from_path}>
                          <Link2Off size={16} className="backlinks__icon" />{' '}
                          {bl.from_title}
                        </Link>
                      </li>
                    ))}
                  </ul>
                </>
              )}
            </>
          )}
        </div>
      </div>
    </div>
  )
}
