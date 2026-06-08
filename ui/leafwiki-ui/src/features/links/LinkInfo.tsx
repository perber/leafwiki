import { useConfigStore } from '@/stores/config'
import { createNavigationVisitState } from '@/lib/navigationVisit'
import i18next from '@/lib/i18n'
import { useIsReadOnly } from '@/lib/useIsReadOnly'
import { Link2Off, Paperclip } from 'lucide-react'
import { useEffect } from 'react'
import { Link } from 'react-router-dom'
import { useViewerStore } from '../viewer/viewer'
import { useLinkStatusStore } from './linkstatus_store'

const t = (key: string, opts?: Record<string, unknown>) =>
  i18next.t(key, { ...opts, ns: 'viewer' })

export function BacklinkInfo() {
  const pageID = useViewerStore((s) => s.page?.id)
  const hideLinkMetadataSection = useConfigStore(
    (s) => s.hideLinkMetadataSection,
  )

  const loading = useLinkStatusStore((s) => s.loading)
  const error = useLinkStatusStore((s) => s.error)
  const status = useLinkStatusStore((s) => s.status)
  const isReadOnly = useIsReadOnly()

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

  if (hideLinkMetadataSection) return null
  const backlinks = status?.backlinks ?? []
  const brokenIncoming = status?.broken_incoming ?? []
  const brokenOutgoings = status?.broken_outgoings ?? []

  return (
    <div className="backlinks__pane">
      <div className="backlinks__content">
        {error && !loading ? (
          <p className="page-viewer__error">Error: {error}</p>
        ) : null}

        <div className="backlinks__group">
          <div className="backlinks__group-title">
            {t('backlinks.referencedBy')}{' '}
            <span className="backlinks__badge">{backlinks.length}</span>
          </div>

          {backlinks.length > 0 ? (
            <ul>
              {backlinks.map((bl) => (
                <li key={bl.from_page_id} className="backlinks__item">
                  <Link to={bl.from_path} state={createNavigationVisitState()}>
                    <Paperclip size={16} className="backlinks__icon" />{' '}
                    {bl.from_title}
                  </Link>
                </li>
              ))}
            </ul>
          ) : loading ? (
            <p className="backlinks__empty">{t('backlinks.loading')}</p>
          ) : (
            <p className="backlinks__empty">{t('backlinks.noReferences')}</p>
          )}
        </div>

        {!isReadOnly && !error && (
          <div className="backlinks__group">
            <div className="backlinks__group-title">
              {t('backlinks.brokenLinks')}{' '}
              <span className="backlinks__badge">
                {brokenIncoming.length + brokenOutgoings.length}
              </span>
            </div>


            {loading ? (
              <p className="backlinks__empty">{t('backlinks.loading')}</p>
            ) : brokenIncoming.length + brokenOutgoings.length === 0 ? (
              <p className="backlinks__empty">{t('backlinks.noBrokenLinks')}</p>
            ) : (
              <>
                {brokenOutgoings.length > 0 && (
                  <>
                    <div className="backlinks__subgroup-title">
                      {t('backlinks.brokenOutgoingsTitle')}
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
                      {t('backlinks.brokenIncomingTitle')}
                    </div>
                    <ul>
                      {brokenIncoming.map((bl) => (
                        <li
                          key={bl.from_page_id}
                          className="backlinks__item backlinks__item--broken"
                        >
                          <Link
                            to={bl.from_path}
                            state={createNavigationVisitState()}
                          >
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
        )}
      </div>
    </div>
  )
}
