import { AddPageDialog } from '@/features/page/AddPageDialog'
import { EditPageMetadataDialog } from '@/features/page/EditPageMetadataDialog'
import { MovePageDialog } from '@/features/page/MovePageDialog'
import { SortPagesDialog } from '@/features/page/SortPagesDialog'
import { useDialogsStore } from '@/stores/dialogs'

export function DialogManger() {
  const dialogType = useDialogsStore((state) => state.dialogType)
  const dialogProps = useDialogsStore((state) => state.dialogProps)

  return (
    <>
      {dialogType === 'add' && <AddPageDialog {...dialogProps} />}
      {dialogType === 'sort' && <SortPagesDialog {...dialogProps} />}
      {dialogType === 'move' && <MovePageDialog {...dialogProps} />}
      {dialogType === 'edit-page-metadata' && (
        <EditPageMetadataDialog {...dialogProps} />
      )}
    </>
  )
}
