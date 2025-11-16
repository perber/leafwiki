import { dialogRegistry } from '@/lib/registries'
import { useDialogsStore } from '@/stores/dialogs'

const dialogs = dialogRegistry.getAllDialogs()

export function DialogManger() {
  const dialogType = useDialogsStore((state) => state.dialogType)
  const dialogProps = useDialogsStore((state) => state.dialogProps)

  return (
    <>
      {dialogs.map((dialog) => {
        if (dialog.type !== dialogType) {
          return null
        }
        const DialogComponent = dialog.render
        return <DialogComponent key={dialog.type} {...dialogProps} />
      })}
    </>
  )
}
