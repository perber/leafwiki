import { FormActions } from '@/components/FormActions'
import { FormInput } from '@/components/FormInput'
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle
} from '@/components/ui/dialog'
import { handleFieldErrors } from '@/lib/handleFieldErrors'
import { useAuthStore } from '@/stores/auth'
import { useUserStore } from '@/stores/users'
import { DialogDescription } from '@radix-ui/react-dialog'
import { useEffect, useState } from 'react'

type Props = {
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function ChangeOnwnPasswordDialog({open, onOpenChange }: Props) {
  const [currentPassword, setCurrentPassword] = useState('')
  const [password, setPassword] = useState('')
  const [confirm, setConfirm] = useState('')
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({})
  const [loading, setLoading] = useState(false)

  const { user} = useAuthStore()
  const { updateUser } = useUserStore()

  if (!user) return null

  const handleCurrentPasswordChange = (val: string) => {
    setCurrentPassword(val)
  }

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
    } catch (err) {
      console.warn(err)
      handleFieldErrors(err, setFieldErrors, 'Error updating password')
    } finally {
      setLoading(false)
    }
  }

  const resetForm = () => {
    setPassword('')
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
            label="Current Password"
            type="password"
            value={currentPassword}
            onChange={handleCurrentPasswordChange}
            placeholder="Current Password"
            error={fieldErrors.currentPassword}
          />
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
            onCancel={() => onOpenChange(false)}
            onSave={handleChange}
            saveLabel={loading ? 'Saving...' : 'Save'}
            disabled={loading || !currentPassword || !password || password !== confirm}
            loading={loading}
          />
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
