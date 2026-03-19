import { PageRefactorPreview } from '@/lib/api/pages'
import { DIALOG_PAGE_REFACTOR_CONFIRMATION } from '@/lib/registries'
import { useDialogsStore } from '@/stores/dialogs'

export function confirmPageRefactor(
  preview: PageRefactorPreview,
): Promise<boolean | null> {
  if (
    preview.counts.affectedPages === 0 &&
    (preview.warnings?.length ?? 0) === 0
  ) {
    return Promise.resolve(false)
  }

  return new Promise((resolve) => {
    useDialogsStore.getState().openDialog(DIALOG_PAGE_REFACTOR_CONFIRMATION, {
      preview,
      onResolve: (rewriteLinks: boolean | null) => {
        resolve(rewriteLinks)
      },
    })
  })
}
