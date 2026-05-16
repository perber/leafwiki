import { PageRefactorPreview } from '@/lib/api/pages'
import { DIALOG_PAGE_REFACTOR_CONFIRMATION } from '@/lib/registries'
import { useConfigStore } from '@/stores/config'
import { useDialogsStore } from '@/stores/dialogs'

export function confirmPageRefactor(
  preview: PageRefactorPreview,
): Promise<boolean | null> {
  if (!useConfigStore.getState().enableLinkRefactor) {
    return Promise.resolve(false)
  }

  const affectedPages = preview?.counts?.affectedPages ?? 0
  const warnings = preview?.warnings ?? []

  if (affectedPages === 0 && warnings.length === 0) {
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
