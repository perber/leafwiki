import BaseDialog from '@/components/BaseDialog'
import { type Revision } from '@/lib/api/revisions'
import { DIALOG_RESTORE_REVISION_CONFIRMATION } from '@/lib/registries'
import { useRef } from 'react'
import { Trans, useTranslation } from 'react-i18next'

export type RestoreRevisionDialogProps = {
  revision: Revision
  currentSlug: string
  onResolve: (confirmed: boolean | null) => void
}

export function RestoreRevisionDialog({
  revision,
  currentSlug,
  onResolve,
}: RestoreRevisionDialogProps) {
  const { t } = useTranslation('history')
  const { t: tCommon } = useTranslation('common')
  const resolvedRef = useRef(false)

  const resolveOnce = (value: boolean | null) => {
    if (resolvedRef.current) {
      return
    }
    resolvedRef.current = true
    onResolve(value)
  }

  const showRevisionSlug = revision.slug && revision.slug !== currentSlug

  return (
    <BaseDialog
      dialogType={DIALOG_RESTORE_REVISION_CONFIRMATION}
      dialogTitle={t('restoreDialog.title')}
      dialogDescription={t('restoreDialog.description')}
      onClose={() => {
        resolveOnce(null)
        return true
      }}
      onConfirm={async (type) => {
        if (type === 'confirm') {
          resolveOnce(true)
          return true
        }
        return false
      }}
      defaultAction="cancel"
      testidPrefix="restore-revision-dialog"
      cancelButton={{
        label: tCommon('actions.cancel'),
        variant: 'outline',
        autoFocus: true,
      }}
      buttons={[
        {
          label: t('restoreDialog.confirmButton'),
          actionType: 'confirm',
          variant: 'default',
          autoFocus: false,
        },
      ]}
    >
      <div className="space-y-3 text-sm">
        <div>
          <span className="font-medium">{t('restoreDialog.restoredTitle')}</span>{' '}
          <span data-testid="restore-revision-dialog-title">
            {revision.title}
          </span>
        </div>
        {showRevisionSlug ? (
          <div
            className="rounded border border-amber-300 bg-amber-50 p-3 text-amber-950"
            data-testid="restore-revision-dialog-slug-note"
          >
            <Trans
              i18nKey="restoreDialog.slugNote"
              ns="history"
              values={{
                revisionSlug: revision.slug,
                currentSlug,
              }}
              components={{
                1: <span className="font-mono" />,
                2: <span className="font-mono" />,
              }}
            />
          </div>
        ) : null}
      </div>
    </BaseDialog>
  )
}
