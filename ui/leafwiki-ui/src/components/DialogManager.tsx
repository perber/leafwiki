import { useEffect, useState } from 'react'

import { dialogRegistry } from '@/lib/registries'
import { useDialogsStore } from '@/stores/dialogs'

const dialogs = dialogRegistry.getAllDialogs()

export function DialogManager() {
  const dialogType = useDialogsStore((state) => state.dialogType)
  const dialogProps = useDialogsStore((state) => state.dialogProps)

  // Keep the last non-null dialog in the render tree so Radix UI Presence
  // can play the exit animation before the component unmounts.
  // BaseDialog derives open={store.dialogType === ownType}, so it gets
  // open=false and Radix handles the fade-out.
  const [renderType, setRenderType] = useState(dialogType)
  const [renderProps, setRenderProps] = useState(dialogProps)

  useEffect(() => {
    if (dialogType !== null) {
      setRenderType(dialogType)
      setRenderProps(dialogProps)
    }
  }, [dialogType, dialogProps])

  return (
    <>
      {dialogs.map((dialog) => {
        if (dialog.type !== renderType) return null
        return dialog.render({ ...renderProps })
      })}
    </>
  )
}
