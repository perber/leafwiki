import { FormActions } from '@/components/FormActions'
import { FormInput } from '@/components/FormInput'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog'
import { handleFieldErrors } from '@/lib/handleFieldErrors'
import { useUserStore } from '@/stores/users'
import { DialogDescription } from '@radix-ui/react-dialog'
import { useCallback, useEffect, useState } from 'react'

type Props = {
  userId: string
  username: string
}

export function ChangePasswordDialog({ userId, username }: Props) {
  const [open, setOpen] = useState(false)
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
      setOpen(false)
    } catch (err) {
      console.warn(err)
      handleFieldErrors(err, setFieldErrors, 'Error updating password')
    } finally {
      setLoading(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <Button size="sm" variant="secondary">
          Change Password
        </Button>
      </DialogTrigger>

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
            onCancel={() => setOpen(false)}
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
