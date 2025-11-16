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
import { DIALOG_CHANGE_OWN_PASSWORD } from '@/lib/registries'
import { useAuthStore } from '@/stores/auth'
import { useDialogsStore } from '@/stores/dialogs'
import { useUserStore } from '@/stores/users'
import { DialogDescription } from '@radix-ui/react-dialog'
import { useCallback, useEffect, useState } from 'react'
import { toast } from 'sonner'

export function ChangeOwnPasswordDialog() {
  // Dialog state from zustand store
  const closeDialog = useDialogsStore((s) => s.closeDialog)
  const open = useDialogsStore(
    (s) => s.dialogType === DIALOG_CHANGE_OWN_PASSWORD,
  )

  const [oldPassword, setOldPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirm, setConfirm] = useState('')
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({})
  const [loading, setLoading] = useState(false)

  const { user } = useAuthStore()
  const { changeOwnPassword } = useUserStore()

  const resetForm = useCallback(() => {
    setOldPassword('')
    setNewPassword('')
    setConfirm('')
    setFieldErrors({})
  }, [])

  useEffect(() => {
    if (open) {
      resetForm()
    }
  }, [open, resetForm])

  if (!user) return null

  const handleOldPasswordChange = (val: string) => {
    setOldPassword(val)
    setFieldErrors((prev) => ({ ...prev, oldPassword: '' }))
  }

  const handleNewPasswordChange = (val: string) => {
    setNewPassword(val)
    if (val.length < 8) {
      setFieldErrors((prev) => ({
        ...prev,
        newPassword: 'Password must be at least 8 characters long',
      }))
    } else {
      setFieldErrors((prev) => ({ ...prev, newPassword: '' }))
    }
  }

  const handleConfirmChange = (val: string) => {
    setConfirm(val)
    if (val !== newPassword) {
      setFieldErrors((prev) => ({ ...prev, confirm: 'Passwords do not match' }))
    } else {
      setFieldErrors((prev) => ({ ...prev, confirm: '' }))
    }
  }

  const handleChange = async () => {
    setLoading(true)
    try {
      await changeOwnPassword(oldPassword, newPassword)
      toast.success('Password changed successfully')
      closeDialog()
    } catch (err) {
      console.warn(err)
      setOldPassword('')
      setNewPassword('')
      setConfirm('')
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
        }
      }}
    >
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Change Own Password</DialogTitle>
          <DialogDescription>
            Change your password. Make sure to remember it!
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-3 pt-2">
          <FormInput
            autoFocus={true}
            label="Old Password"
            type="password"
            value={oldPassword}
            onChange={handleOldPasswordChange}
            placeholder="Old Password"
            error={fieldErrors.oldPassword}
          />
          <FormInput
            label="New Password"
            type="password"
            value={newPassword}
            onChange={handleNewPasswordChange}
            placeholder="New Password"
            error={fieldErrors.newPassword}
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
            onCancel={() => closeDialog()}
            onSave={handleChange}
            saveLabel={loading ? 'Saving...' : 'Save'}
            disabled={
              loading ||
              !oldPassword ||
              !newPassword ||
              newPassword !== confirm ||
              fieldErrors.newPassword !== '' ||
              fieldErrors.confirm !== ''
            }
            loading={loading}
          />
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
