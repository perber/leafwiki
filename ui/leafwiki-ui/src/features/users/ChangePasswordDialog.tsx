import BaseDialog from '@/components/BaseDialog'
import { FormInput } from '@/components/FormInput'
import { handleFieldErrors } from '@/lib/handleFieldErrors'
import { DIALOG_CHANGE_USER_PASSWORD } from '@/lib/registries'
import { useUserStore } from '@/stores/users'
import { useCallback, useState } from 'react'

type ChangePasswordDialogProps = {
  userId: string
  username: string
}

export function ChangePasswordDialog({
  userId,
  username,
}: ChangePasswordDialogProps) {
  const [password, setPassword] = useState('')
  const [confirm, setConfirm] = useState('')
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({})
  const [loading, setLoading] = useState(false)

  const { users, updateUser } = useUserStore()

  const user = users.find((u) => u.id === userId)

  const resetForm = useCallback((): boolean => {
    setPassword('')
    setConfirm('')
    setFieldErrors({})
    return true
  }, [])

  if (!user) return null

  const submitDisabled =
    loading ||
    password.length < 8 ||
    password !== confirm ||
    fieldErrors.password !== '' ||
    fieldErrors.confirm !== ''

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

  const handleChange = async (): Promise<boolean> => {
    setLoading(true)
    try {
      await updateUser({
        ...user,
        password,
      })
      return true // Close the dialog
    } catch (err) {
      console.warn(err)
      handleFieldErrors(err, setFieldErrors, 'Error updating password')
      return false // Keep the dialog open
    } finally {
      setLoading(false)
    }
  }

  return (
    <BaseDialog
      dialogType={DIALOG_CHANGE_USER_PASSWORD}
      dialogTitle={`Change Password for ${username}`}
      dialogDescription="Set a new password for the user."
      onClose={resetForm}
      onConfirm={async () => {
        return await handleChange()
      }}
      testidPrefix="change-user-password-dialog"
      cancelButton={{
        label: 'Cancel',
        variant: 'outline',
        disabled: loading,
        autoFocus: true,
      }}
      buttons={[
        {
          label: loading ? 'Updating...' : 'Update Password',
          actionType: 'confirm',
          autoFocus: false,
          loading,
          disabled: submitDisabled,
        },
      ]}
    >
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
    </BaseDialog>
  )
}
