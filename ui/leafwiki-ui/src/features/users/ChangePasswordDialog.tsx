import BaseDialog from '@/components/BaseDialog'
import { FormInput } from '@/components/FormInput'
import { handleFieldErrors } from '@/lib/handleFieldErrors'
import { DIALOG_CHANGE_USER_PASSWORD } from '@/lib/registries'
import { useUserStore } from '@/stores/users'
import { useCallback, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

const DIALOG_INPUT_ALLOWED_HOTKEYS = 'Enter'

type ChangePasswordDialogProps = {
  userId: string
  username: string
}

export function ChangePasswordDialog({
  userId,
  username,
}: ChangePasswordDialogProps) {
  const { t } = useTranslation('users')
  const { t: tCommon } = useTranslation('common')
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
        password: t('validation.passwordMinLength'),
      }))
    } else {
      setFieldErrors((prev) => ({ ...prev, password: '' }))
    }
  }

  const handleConfirmChange = (val: string) => {
    setConfirm(val)
    if (val !== password) {
      setFieldErrors((prev) => ({
        ...prev,
        confirm: t('validation.passwordMismatch'),
      }))
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
      toast.success(t('toast.passwordChanged'))
      return true
    } catch (err) {
      console.warn(err)
      handleFieldErrors(err, setFieldErrors, t('toast.updatePasswordError'))
      return false
    } finally {
      setLoading(false)
    }
  }

  return (
    <BaseDialog
      dialogType={DIALOG_CHANGE_USER_PASSWORD}
      dialogTitle={t('changePasswordFor', { username })}
      dialogDescription={t('changePasswordDescription')}
      onClose={resetForm}
      onConfirm={async () => {
        return await handleChange()
      }}
      testidPrefix="change-user-password-dialog"
      cancelButton={{
        label: t('actions.cancel'),
        variant: 'outline',
        disabled: loading,
        autoFocus: true,
      }}
      buttons={[
        {
          label: loading
            ? tCommon('actions.updating')
            : t('actions.updatePassword'),
          actionType: 'confirm',
          autoFocus: false,
          loading,
          disabled: submitDisabled,
        },
      ]}
    >
      <div className="space-y-3 pt-2">
        <input
          aria-hidden="true"
          autoComplete="username"
          className="hidden"
          name="username"
          readOnly
          tabIndex={-1}
          type="text"
          value={username}
        />
        <FormInput
          autoFocus={true}
          label={t('newPassword')}
          name="new-password"
          type="password"
          value={password}
          onChange={handlePasswordChange}
          placeholder={t('newPassword')}
          autoComplete="new-password"
          error={fieldErrors.password}
          allowedHotkeys={DIALOG_INPUT_ALLOWED_HOTKEYS}
        />
        <FormInput
          label={t('confirmPassword')}
          name="confirm-new-password"
          type="password"
          value={confirm}
          onChange={handleConfirmChange}
          placeholder={t('confirmPassword')}
          autoComplete="new-password"
          error={fieldErrors.confirm}
          allowedHotkeys={DIALOG_INPUT_ALLOWED_HOTKEYS}
        />
      </div>
    </BaseDialog>
  )
}
