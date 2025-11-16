import { FormActions } from '@/components/FormActions'
import { FormInput } from '@/components/FormInput'
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { handleFieldErrors } from '@/lib/handleFieldErrors'
import { DIALOG_CHANGE_USER_PASSWORD } from '@/lib/registries'
import { useDialogsStore } from '@/stores/dialogs'
import { useUserStore } from '@/stores/users'
import { DialogDescription } from '@radix-ui/react-dialog'
import { useCallback, useEffect, useState } from 'react'

type ChangePasswordDialogProps = {
  userId: string
  username: string
}

export function ChangePasswordDialog({
  userId,
  username,
}: ChangePasswordDialogProps) {
  const closeDialog = useDialogsStore((s) => s.closeDialog)
  const open = useDialogsStore(
    (s) => s.dialogType === DIALOG_CHANGE_USER_PASSWORD,
  )

  const [password, setPassword] = useState('')
  const [confirm, setConfirm] = useState('')
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({})
  const [loading, setLoading] = useState(false)

  const { users, updateUser } = useUserStore()

  const user = users.find((u) => u.id === userId)

  const resetForm = useCallback(() => {
    setPassword('')
    setConfirm('')
    setFieldErrors({})
  }, [])

  useEffect(() => {
    if (open) {
      resetForm()
    }
  }, [open, resetForm])

  if (!user) return null

  const handlePasswordChange = (val: string) => {
    setPassword(val)
    if (val.length < 8) {
      setFieldErrors((prev) => ({
        ...prev,
        password: 'Password must be at least 8 characters long',
      }))
    } else {
      setFieldErrors((prev) => ({ ...prev, password: '' }))
    }
  }

  const handleConfirmChange = (val: string) => {
    setConfirm(val)
    if (val !== password) {
      setFieldErrors((prev) => ({ ...prev, confirm: 'Passwords do not match' }))
    } else {
      setFieldErrors((prev) => ({ ...prev, confirm: '' }))
    }
  }

  const handleChange = async () => {
    setLoading(true)
    try {
      await updateUser({
        ...user,
        password,
      })
      closeDialog()
    } catch (err) {
      console.warn(err)
      handleFieldErrors(err, setFieldErrors, 'Error updating password')
    } finally {
      setLoading(false)
    }
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(isOpen) => {
        if (!isOpen) {
          closeDialog()
          resetForm()
        }
      }}
    >
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Change password for user {username}</DialogTitle>
          <DialogDescription>
            Enter a new password for the user.
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-3 pt-2">
          <FormInput
            autoFocus={true}
            label="New Password"
            type="password"
            value={password}
            onChange={handlePasswordChange}
            placeholder="New Password"
            error={fieldErrors.password}
          />
          <FormInput
            label="Confirm Password"
            type="password"
            value={confirm}
            onChange={handleConfirmChange}
            placeholder="Confirm Password"
            error={fieldErrors.confirm}
          />
        </div>

        <DialogFooter className="pt-4">
          <FormActions
            onCancel={() => {
              closeDialog()
              resetForm()
            }}
            onSave={handleChange}
            saveLabel={loading ? 'Saving...' : 'Save'}
            disabled={
              loading ||
              !password ||
              password !== confirm ||
              fieldErrors.password !== '' ||
              fieldErrors.confirm !== ''
            }
            loading={loading}
          />
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
