import { AddPageDialog } from '@/features/page/AddPageDialog'
import { CopyPageDialog } from '@/features/page/CopyPageDialog'
import { CreatePageByPathDialog } from '@/features/page/CreatePageByPathDialog'
import { EditPageMetadataDialog } from '@/features/page/EditPageMetadataDialog'
import { MovePageDialog } from '@/features/page/MovePageDialog'
import { SortPagesDialog } from '@/features/page/SortPagesDialog'
import { useDialogsStore } from '@/stores/dialogs'

export function DialogManger() {
  const dialogType = useDialogsStore((state) => state.dialogType)
  const dialogProps = useDialogsStore((state) => state.dialogProps)

  return (
    <>
      {dialogType === 'add' && (
        <AddPageDialog
          {...(dialogProps as React.ComponentProps<typeof AddPageDialog>)}
        />
      )}
      {dialogType === 'sort' && (
        <SortPagesDialog
          {...(dialogProps as React.ComponentProps<typeof SortPagesDialog>)}
        />
      )}
      {dialogType === 'move' && (
        <MovePageDialog
          {...(dialogProps as React.ComponentProps<typeof MovePageDialog>)}
        />
      )}
      {dialogType === 'create-by-path' && (
        <CreatePageByPathDialog
          {...(dialogProps as React.ComponentProps<
            typeof CreatePageByPathDialog
          >)}
        />
      )}
      {dialogType === 'copy-page' && (
        <CopyPageDialog
          {...(dialogProps as React.ComponentProps<typeof CopyPageDialog>)}
        />
      )}
      {dialogType === 'edit-page-metadata' && (
        <EditPageMetadataDialog
          {...(dialogProps as React.ComponentProps<
            typeof EditPageMetadataDialog
          >)}
        />
      )}
    </>
  )
}
