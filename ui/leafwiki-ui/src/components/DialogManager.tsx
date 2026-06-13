import { Suspense, useEffect, useState } from 'react'

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
    // When opening (or updating props for) a dialog, render it immediately.
    if (dialogType !== null) {
      setRenderType(dialogType)
      setRenderProps(dialogProps)
      return
    }
    // When closing, keep the dialog mounted briefly so Radix UI Presence can
    // play the exit animation, then unmount to avoid retaining dialog state.
    const timeoutId = setTimeout(() => {
      setRenderType(null)
      setRenderProps(null)
    }, 200)
    return () => clearTimeout(timeoutId)
  }, [dialogType, dialogProps, renderType])

  return (
    <Suspense fallback={null}>
      {dialogs.map((dialog) => {
        if (dialog.type !== renderType) return null
        return dialog.render({ ...renderProps })
      })}
    </Suspense>
  )
}
