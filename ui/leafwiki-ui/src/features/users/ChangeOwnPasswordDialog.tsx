import BaseDialog from '@/components/BaseDialog'
import { FormInput } from '@/components/FormInput'
import { handleFieldErrors } from '@/lib/handleFieldErrors'
import { DIALOG_CHANGE_OWN_PASSWORD } from '@/lib/registries'
import { useSessionStore } from '@/stores/session'
import { useUserStore } from '@/stores/users'
import { useCallback, useState } from 'react'
import { toast } from 'sonner'

export function ChangeOwnPasswordDialog() {
  const [oldPassword, setOldPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirm, setConfirm] = useState('')
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({})
  const [loading, setLoading] = useState(false)

  const { user } = useSessionStore()
  const { changeOwnPassword } = useUserStore()

  const resetForm = useCallback((): boolean => {
    setOldPassword('')
    setNewPassword('')
    setConfirm('')
    setFieldErrors({})
    return true
  }, [])

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

  const handleChange = async (): Promise<boolean> => {
    setLoading(true)
    try {
      await changeOwnPassword(oldPassword, newPassword)
      toast.success('Password changed successfully')
      return true
    } catch (err) {
      console.warn(err)
      setOldPassword('')
      setNewPassword('')
      setConfirm('')
      handleFieldErrors(err, setFieldErrors, 'Error updating password')
      return false
    } finally {
      setLoading(false)
    }
  }

  return (
    <BaseDialog
      dialogType={DIALOG_CHANGE_OWN_PASSWORD}
      defaultAction="cancel"
      dialogTitle="Change Own Password"
      dialogDescription="Change your password. Make sure to remember it!"
      testidPrefix="change-own-password-dialog"
      cancelButton={{
        label: 'Cancel',
        variant: 'outline',
        disabled: loading,
        autoFocus: false,
      }}
      buttons={[
        {
          label: loading ? 'Saving...' : 'Save',
          actionType: 'confirm',
          autoFocus: true,
          loading,
          disabled:
            loading ||
            !oldPassword ||
            !newPassword ||
            newPassword !== confirm ||
            fieldErrors.newPassword !== '' ||
            fieldErrors.confirm !== '',
        },
      ]}
      onClose={resetForm}
      onConfirm={async (): Promise<boolean> => {
        return await handleChange()
      }}
    >
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
    </BaseDialog>
  )
}
