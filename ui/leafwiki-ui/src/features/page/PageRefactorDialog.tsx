import BaseDialog from '@/components/BaseDialog'
import { ListView, ListViewList, ListViewStatus } from '@/components/ListView'
import { Checkbox } from '@/components/ui/checkbox'
import { PageRefactorPreview } from '@/lib/api/pages'
import { DIALOG_PAGE_REFACTOR_CONFIRMATION } from '@/lib/registries'
import { useRef, useState } from 'react'

export type PageRefactorDialogProps = {
  preview: PageRefactorPreview
  onResolve: (rewriteLinks: boolean | null) => void
}

export function PageRefactorDialog({
  preview,
  onResolve,
}: PageRefactorDialogProps) {
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
      dialogTitle="Update references?"
      dialogDescription="This change affects the page path. Review the impacted pages before continuing."
      onClose={() => {
        resolveOnce(null)
        return true
      }}
      onConfirm={async (type) => {
        if (type === 'confirm') {
          resolveOnce(rewriteLinks)
          return true
        }
        return false
      }}
      defaultAction="cancel"
      testidPrefix="page-refactor-dialog"
      cancelButton={{
        label: 'Cancel',
        variant: 'outline',
        autoFocus: false,
      }}
      buttons={[
        {
          label: 'Continue',
          actionType: 'confirm',
          variant: 'default',
          autoFocus: true,
        },
      ]}
    >
      <div className="space-y-4">
        <div className="space-y-1 text-sm">
          <div>
            <span className="font-medium">Old path:</span>{' '}
            <span className="font-mono">{preview.oldPath}</span>
          </div>
          <div>
            <span className="font-medium">New path:</span>{' '}
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
          Update links on referencing pages automatically
        </label>

        <div className="space-y-2">
          <div
            className="text-sm font-medium"
            data-testid="page-refactor-dialog-referencing-pages-heading"
          >
            Referencing pages ({preview.counts.affectedPages})
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
                  No pages reference this path.
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
                        {matchedPaths.join(', ')}
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
