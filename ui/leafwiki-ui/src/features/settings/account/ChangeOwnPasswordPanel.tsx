import { Button } from '@/components/ui/button'
import { FormInput } from '@/components/FormInput'
import { handleFieldErrors } from '@/lib/handleFieldErrors'
import { useSessionStore } from '@/stores/session'
import { useUserStore } from '@/stores/users'
import { Loader2, SaveIcon } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

export function ChangeOwnPasswordPanel() {
  const { t } = useTranslation('users')
  const [oldPassword, setOldPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirm, setConfirm] = useState('')
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({})
  const [loading, setLoading] = useState(false)

  const { user } = useSessionStore()
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
        newPassword: t('validation.passwordTooShort'),
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
        confirm: t('validation.passwordsDoNotMatch'),
      }))
    } else {
      setFieldErrors((prev) => ({ ...prev, confirm: '' }))
    }
  }

  const handleSave = async () => {
    setLoading(true)
    try {
      await changeOwnPassword(oldPassword, newPassword)
      toast.success(t('changeOwnPassword.successToast'))
      setOldPassword('')
      setNewPassword('')
      setConfirm('')
      setFieldErrors({})
    } catch (err) {
      console.warn(err)
      setOldPassword('')
      setNewPassword('')
      setConfirm('')
      handleFieldErrors(
        err,
        setFieldErrors,
        t('changeOwnPassword.errorFallback'),
      )
    } finally {
      setLoading(false)
    }
  }

  const saveDisabled =
    loading ||
    !oldPassword ||
    !newPassword ||
    newPassword !== confirm ||
    fieldErrors.newPassword !== '' ||
    fieldErrors.confirm !== ''

  return (
    <div
      className="settings__field space-y-3"
      data-testid="change-own-password-panel"
    >
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
        label={t('changeOwnPassword.oldPasswordLabel')}
        name="current-password"
        type="password"
        value={oldPassword}
        onChange={handleOldPasswordChange}
        placeholder={t('changeOwnPassword.oldPasswordLabel')}
        autoComplete="current-password"
        error={fieldErrors.oldPassword}
        testid="change-own-password-old"
      />
      <FormInput
        label={t('changeOwnPassword.newPasswordLabel')}
        name="new-password"
        type="password"
        value={newPassword}
        onChange={handleNewPasswordChange}
        placeholder={t('changeOwnPassword.newPasswordLabel')}
        autoComplete="new-password"
        error={fieldErrors.newPassword}
        testid="change-own-password-new"
      />
      <FormInput
        label={t('changeOwnPassword.confirmPasswordLabel')}
        name="confirm-new-password"
        type="password"
        value={confirm}
        onChange={handleConfirmChange}
        placeholder={t('changeOwnPassword.confirmPasswordLabel')}
        autoComplete="new-password"
        error={fieldErrors.confirm}
        testid="change-own-password-confirm"
      />
      <div className="settings__actions">
        <Button
          onClick={handleSave}
          disabled={saveDisabled}
          data-testid="change-own-password-save"
        >
          {loading ? (
            <Loader2 className="mr-2 h-4 w-4 animate-spin" />
          ) : (
            <SaveIcon className="mr-2 h-4 w-4" />
          )}
          {loading
            ? t('changeOwnPassword.saving')
            : t('changeOwnPassword.save')}
        </Button>
      </div>
    </div>
  )
}
