import BaseDialog from '@/components/BaseDialog'
import { FormInput } from '@/components/FormInput'
import { handleFieldErrors } from '@/lib/handleFieldErrors'
import { DIALOG_CHANGE_OWN_PASSWORD } from '@/lib/registries'
import { useSessionStore } from '@/stores/session'
import { useUserStore } from '@/stores/users'
import { useCallback, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

const DIALOG_INPUT_ALLOWED_HOTKEYS = 'Enter'

export function ChangeOwnPasswordDialog() {
  const { t } = useTranslation('users')
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
        newPassword: t('validation.passwordMinLength'),
      }))
    } else {
      setFieldErrors((prev) => ({ ...prev, newPassword: '' }))
    }
  }

  const handleConfirmChange = (val: string) => {
    setConfirm(val)
    if (val !== newPassword) {
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
      await changeOwnPassword(oldPassword, newPassword)
      toast.success(t('toast.passwordChanged'))
      return true
    } catch (err) {
      console.warn(err)
      setOldPassword('')
      setNewPassword('')
      setConfirm('')
      handleFieldErrors(err, setFieldErrors, t('toast.updatePasswordError'))
      return false
    } finally {
      setLoading(false)
    }
  }

  return (
    <BaseDialog
      dialogType={DIALOG_CHANGE_OWN_PASSWORD}
      dialogTitle={t('changeOwnPasswordTitle')}
      dialogDescription={t('changeOwnPasswordDescription')}
      testidPrefix="change-own-password-dialog"
      cancelButton={{
        label: t('actions.cancel'),
        variant: 'outline',
        disabled: loading,
        autoFocus: false,
      }}
      buttons={[
        {
          label: loading ? t('actions.save') : t('actions.save'),
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
        <input
          aria-hidden="true"
          autoComplete="username"
          className="hidden"
          name="username"
          readOnly
          tabIndex={-1}
          type="text"
          value={user.username}
        />
        <FormInput
          autoFocus={true}
          label={t('oldPassword')}
          name="current-password"
          type="password"
          value={oldPassword}
          onChange={handleOldPasswordChange}
          placeholder={t('oldPassword')}
          autoComplete="current-password"
          error={fieldErrors.oldPassword}
          allowedHotkeys={DIALOG_INPUT_ALLOWED_HOTKEYS}
        />
        <FormInput
          label={t('newPassword')}
          name="new-password"
          type="password"
          value={newPassword}
          onChange={handleNewPasswordChange}
          placeholder={t('newPassword')}
          autoComplete="new-password"
          error={fieldErrors.newPassword}
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
