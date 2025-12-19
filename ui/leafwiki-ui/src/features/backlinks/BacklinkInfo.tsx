import { Paperclip } from 'lucide-react'
import { useEffect } from 'react'
import { Link } from 'react-router-dom'
import { useViewerStore } from '../viewer/viewer'
import { useBacklinksStore } from './backlinks'

export function BacklinkInfo() {
  const loading = useBacklinksStore((s) => s.loading)
  const backlinks = useBacklinksStore((s) => s.backlinks)
  const fetchPageBacklinks = useBacklinksStore((s) => s.fetchPageBacklinks)
  const pageID = useViewerStore((s) => s.page?.id)

  useEffect(() => {
    if (!pageID) return
    fetchPageBacklinks(pageID)
  }, [fetchPageBacklinks, pageID])

  return (
    <div className="backlinks__pane">
      <div className="backlinks__header">
        <h2 className="mb-2 text-lg font-medium">
          Page is referenced by (
          {backlinks && backlinks.length > 0 ? backlinks.length : 0})
        </h2>
      </div>
      <div className="backlinks__content">
        {backlinks && backlinks.length > 0 ? (
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
          <p>Loading...</p>
        ) : (
          <p>No backlinks found.</p>
        )}
      </div>
    </div>
  )
}
