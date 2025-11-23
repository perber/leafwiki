// BaseDialog

import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { useDialogsStore } from '@/stores/dialogs'
import { HotKeyDefinition, useHotKeysStore } from '@/stores/hotkeys'
import { Loader2 } from 'lucide-react'
import { ReactNode, useEffect } from 'react'
import { Button } from './ui/button'

// A generic dialog component that can be used as a base for other dialogs.
// It provides a consistent structure and styling for dialogs in the application.
// It also registers hotkeys for dialog actions like confirm and cancel.

export type BaseDialogProps = {
  dialogTitle: string
  dialogDescription: string
  dialogType: string
  defaultAction?: 'confirm' | 'cancel'
  testidPrefix?: string
  onClose: () => boolean
  onConfirm: (type: string) => Promise<boolean>
  children?: ReactNode
  cancelButton: BaseDialogCancelButton
  buttons?: BaseDialogConfirmButton[]
}

export type BaseDialogCancelButton = {
  label?: string
  variant?:
    | 'default'
    | 'destructive'
    | 'outline'
    | 'ghost'
    | 'link'
    | 'secondary'
  disabled?: boolean
  autoFocus?: boolean
}

export type BaseDialogConfirmButton = {
  label: string
  variant?:
    | 'default'
    | 'destructive'
    | 'outline'
    | 'ghost'
    | 'link'
    | 'secondary'
  loading?: boolean
  disabled?: boolean
  autoFocus?: boolean
  /**
   * The type of action this button represents. This value is passed to the `onConfirm` handler
   * to distinguish between different action buttons. Common values include:
   * - 'confirm': for the main confirmation action
   * - 'cancel': for a cancellation action
   * - 'custom': for any custom action
   */
  actionType: string
}

export default function BaseDialog({
  dialogTitle,
  dialogDescription,
  dialogType,
  onClose,
  onConfirm,
  defaultAction = 'confirm',
  children,
  testidPrefix,
  cancelButton,
  buttons,
}: BaseDialogProps) {
  const closeDialog = useDialogsStore((s) => s.closeDialog)
  const open = useDialogsStore((s) => s.dialogType === dialogType)
  const registerHotkey = useHotKeysStore((s) => s.registerHotkey)
  const unregisterHotkey = useHotKeysStore((s) => s.unregisterHotkey)

  useEffect(() => {
    // Only register hotkeys when the dialog is open
    if (!open) {
      return
    }

    const confirmHotkey: HotKeyDefinition = {
      keyCombo: 'Enter',
      enabled: true,
      mode: ['dialog'],
      action: async () => {
        if (defaultAction === 'cancel') {
          onClose()
          closeDialog()
          return
        }

        const result = await onConfirm('confirm')
        if (result) {
          closeDialog()
        }
      },
    }
    const cancelHotkey: HotKeyDefinition = {
      keyCombo: 'Escape',
      enabled: true,
      mode: ['dialog'],
      action: () => {
        onClose()
        closeDialog()
      },
    }
    registerHotkey(confirmHotkey)
    registerHotkey(cancelHotkey)

    return () => {
      unregisterHotkey(confirmHotkey.keyCombo)
      unregisterHotkey(cancelHotkey.keyCombo)
    }
  }, [
    open,
    onClose,
    onConfirm,
    closeDialog,
    dialogType,
    registerHotkey,
    unregisterHotkey,
    defaultAction,
  ])

  return (
    <Dialog
      open={open}
      onOpenChange={(isOpen) => {
        if (!isOpen) {
          onClose()
          closeDialog()
        }
      }}
    >
      <DialogContent
        onEscapeKeyDown={(e: KeyboardEvent) => {
          // The dialog isn't responsible to handle key events!
          e.preventDefault()
        }}
      >
        <DialogHeader>
          <DialogTitle>{dialogTitle}</DialogTitle>
          <DialogDescription>{dialogDescription}</DialogDescription>
        </DialogHeader>
        {children}
        <div className="mt-6 flex justify-end gap-2">
          {cancelButton && (
            <Button
              autoFocus={cancelButton.autoFocus}
              onClick={() => {
                const result = onClose()
                if (result) {
                  closeDialog()
                }
              }}
              disabled={cancelButton.disabled}
              data-testid={
                testidPrefix ? `${testidPrefix}-button-cancel` : undefined
              }
              variant={
                cancelButton.variant as
                  | 'default'
                  | 'destructive'
                  | 'outline'
                  | 'ghost'
                  | 'link'
                  | 'secondary'
              }
            >
              {cancelButton.label}
            </Button>
          )}
          {buttons?.map((button) => (
            <Button
              key={button.actionType}
              onClick={async () => {
                const result = await onConfirm(button.actionType)
                if (result) {
                  closeDialog()
                }
              }}
              disabled={button.loading || button.disabled}
              variant={
                button.variant as
                  | 'default'
                  | 'destructive'
                  | 'outline'
                  | 'ghost'
                  | 'link'
                  | 'secondary'
              }
              autoFocus={button.autoFocus}
              data-testid={
                testidPrefix
                  ? `${testidPrefix}-button-${button.actionType}`
                  : undefined
              }
            >
              {button.loading && (
                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
              )}
              {button.label}
            </Button>
          ))}
        </div>
      </DialogContent>
    </Dialog>
  )
}
