import BaseDialog from '@/components/BaseDialog'
import { type Revision } from '@/lib/api/revisions'
import { DIALOG_RESTORE_REVISION_CONFIRMATION } from '@/lib/registries'
import { useRef } from 'react'

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
      dialogTitle="Restore revision?"
      dialogDescription="This restores the revision content, title, and assets. The current slug and location stay unchanged."
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
        label: 'Cancel',
        variant: 'outline',
        autoFocus: true,
      }}
      buttons={[
        {
          label: 'Restore revision',
          actionType: 'confirm',
          variant: 'default',
          autoFocus: false,
        },
      ]}
    >
      <div className="space-y-3 text-sm">
        <div>
          <span className="font-medium">Restored title:</span>{' '}
          <span data-testid="restore-revision-dialog-title">
            {revision.title}
          </span>
        </div>
        {showRevisionSlug ? (
          <div
            className="rounded border border-amber-300 bg-amber-50 p-3 text-amber-950"
            data-testid="restore-revision-dialog-slug-note"
          >
            This revision used slug{' '}
            <span className="font-mono">{revision.slug}</span>. The current slug{' '}
            <span className="font-mono">{currentSlug}</span> will stay
            unchanged.
          </div>
        ) : null}
      </div>
    </BaseDialog>
  )
}
