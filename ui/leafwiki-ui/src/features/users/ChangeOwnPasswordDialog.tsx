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
import { useAuthStore } from '@/stores/auth'
import { useUserStore } from '@/stores/users'
import { DialogDescription } from '@radix-ui/react-dialog'
import { useEffect, useState } from 'react'
import { toast } from 'sonner'

type Props = {
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function ChangeOnwnPasswordDialog({ open, onOpenChange }: Props) {
  const [oldPassword, setOldPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirm, setConfirm] = useState('')
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({})
  const [loading, setLoading] = useState(false)

  const { user } = useAuthStore()
  const { changeOwnPassword } = useUserStore()

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
      onOpenChange(false)
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

  const resetForm = () => {
    setOldPassword('')
    setNewPassword('')
    setConfirm('')
    setFieldErrors({})
  }

  useEffect(() => {
    if (open) {
      resetForm()
    }
  }, [open])

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
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
            onCancel={() => onOpenChange(false)}
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
