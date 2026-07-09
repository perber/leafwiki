import BaseDialog from '@/components/BaseDialog'
import { ListView, ListViewList, ListViewStatus } from '@/components/ListView'
import { Checkbox } from '@/components/ui/checkbox'
import { PageRefactorPreview } from '@/lib/api/pages'
import { DIALOG_PAGE_REFACTOR_CONFIRMATION } from '@/lib/registries'
import { useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

export type PageRefactorDialogProps = {
  preview: PageRefactorPreview
  allowSkipRewrite?: boolean
  onResolve: (rewriteLinks: boolean | null) => void
}

export function PageRefactorDialog({
  preview,
  allowSkipRewrite = false,
  onResolve,
}: PageRefactorDialogProps) {
  const { t } = useTranslation('page')
  const previewWarnings = preview.warnings ?? []
  const defaultRewriteLinks = preview.counts.matchedLinks > 0
  const [rewriteLinks, setRewriteLinks] = useState(defaultRewriteLinks)
  const resolvedRef = useRef(false)

  const resolveOnce = (value: boolean | null) => {
    if (resolvedRef.current) {
      return
    }
    resolvedRef.current = true
    onResolve(value)
  }

  return (
    <BaseDialog
      dialogType={DIALOG_PAGE_REFACTOR_CONFIRMATION}
      dialogTitle={t('refactor.title')}
      dialogDescription={t('refactor.description')}
      onClose={() => {
        resolveOnce(null)
        return true
      }}
      onConfirm={async (type) => {
        if (type === 'confirm') {
          resolveOnce(rewriteLinks)
          return true
        }
        if (type === 'save-without-rewrite') {
          resolveOnce(false)
          return true
        }
        return false
      }}
      defaultAction="cancel"
      testidPrefix="page-refactor-dialog"
      cancelButton={{
        label: t('actions.cancel'),
        variant: 'outline',
        autoFocus: false,
      }}
      buttons={[
        ...(allowSkipRewrite
          ? [
              {
                label: t('refactor.saveWithoutUpdating'),
                actionType: 'save-without-rewrite',
                variant: 'secondary' as const,
              },
            ]
          : []),
        {
          label: t('refactor.continue'),
          actionType: 'confirm',
          variant: 'default',
          autoFocus: true,
        },
      ]}
    >
      <div className="space-y-4">
        <div className="space-y-1 text-sm">
          <div>
            <span className="font-medium">{t('refactor.oldPath')}</span>{' '}
            <span className="font-mono">{preview.oldPath}</span>
          </div>
          <div>
            <span className="font-medium">{t('refactor.newPath')}</span>{' '}
            <span className="font-mono">{preview.newPath}</span>
          </div>
        </div>

        <label className="flex items-center gap-2 text-sm">
          <Checkbox
            data-testid="page-refactor-dialog-checkbox-rewrite-links"
            checked={rewriteLinks}
            onCheckedChange={(value) => setRewriteLinks(!!value)}
            disabled={!defaultRewriteLinks}
          />
          {t('refactor.rewriteLinksLabel')}
        </label>

        <div className="space-y-2">
          <div
            className="text-sm font-medium"
            data-testid="page-refactor-dialog-referencing-pages-heading"
          >
            {t('refactor.referencingPages', {
              count: preview.counts.affectedPages,
            })}
          </div>

          {previewWarnings.length > 0 && (
            <div
              className="rounded border border-amber-300 bg-amber-50 p-2 text-sm text-amber-900"
              data-testid="page-refactor-dialog-warnings"
            >
              {previewWarnings.map((warning) => (
                <div key={warning}>{warning}</div>
              ))}
            </div>
          )}

          <ListView
            as="div"
            className="page-refactor-dialog__results-view"
            contentClassName="page-refactor-dialog__results-content custom-scrollbar"
            testId="page-refactor-dialog-affected-pages"
          >
            {preview.affectedPages.length === 0 ? (
              <div data-testid="page-refactor-dialog-no-references">
                <ListViewStatus className="page-refactor-dialog__result-summary">
                  {t('refactor.noReferences')}
                </ListViewStatus>
              </div>
            ) : (
              <ListViewList>
                {preview.affectedPages.map((page) => {
                  const pageWarnings = page.warnings ?? []
                  const matchedPaths = page.matchedPaths ?? []

                  return (
                    <div
                      key={page.fromPageId}
                      className="list-view__item page-refactor-dialog__affected-page"
                      data-testid="page-refactor-dialog-affected-page"
                    >
                      <div className="page-refactor-dialog__affected-page-title">
                        {page.fromTitle}
                      </div>
                      <div className="page-refactor-dialog__affected-page-path">
                        {page.fromPath}
                      </div>
                      <div
                        className="page-refactor-dialog__affected-page-matches"
                        data-testid="page-refactor-dialog-affected-page-matches"
                      >
                        {matchedPaths
                          .map((p) =>
                            p.replace(/^\[\[/, '').replace(/\]\]$/, ''),
                          )
                          .join(', ')}
                      </div>
                      {pageWarnings.length > 0 && (
                        <div
                          className="page-refactor-dialog__affected-page-warnings"
                          data-testid="page-refactor-dialog-affected-page-warnings"
                        >
                          {pageWarnings.map((warning) => (
                            <div key={warning}>{warning}</div>
                          ))}
                        </div>
                      )}
                    </div>
                  )
                })}
              </ListViewList>
            )}
          </ListView>
        </div>
      </div>
    </BaseDialog>
  )
}
