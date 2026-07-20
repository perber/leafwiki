import BaseDialog from '@/components/BaseDialog'
import { FormInput } from '@/components/FormInput'
import { mapApiError } from '@/lib/api/errors'
import { disableTOTP } from '@/lib/api/totp'
import { DIALOG_TOTP_DISABLE } from '@/lib/registries'
import { useSessionStore } from '@/stores/session'
import { useCallback, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

const DIALOG_INPUT_ALLOWED_HOTKEYS = 'Enter'

export function TOTPDisableDialog() {
  const { t } = useTranslation('users')
  const user = useSessionStore((s) => s.user)
  const setUser = useSessionStore((s) => s.setUser)

  const [currentPassword, setCurrentPassword] = useState('')
  const [code, setCode] = useState('')
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({})
  const [loading, setLoading] = useState(false)

  const resetForm = useCallback((): boolean => {
    setCurrentPassword('')
    setCode('')
    setFieldErrors({})
    return true
  }, [])

  if (!user) return null

  const handleDisable = async (): Promise<boolean> => {
    setLoading(true)
    try {
      await disableTOTP(currentPassword, code)
      setUser({ ...user, totpEnabled: false })
      toast.success(t('totp.disable.successToast'))
      return true
    } catch (err) {
      setCurrentPassword('')
      setCode('')
      const message = mapApiError(err, t('totp.disable.errorFallback')).message
      setFieldErrors({ currentPassword: message, code: message })
      return false
    } finally {
      setLoading(false)
    }
  }

  return (
    <BaseDialog
      dialogType={DIALOG_TOTP_DISABLE}
      dialogTitle={t('totp.disable.title')}
      dialogDescription={t('totp.disable.description')}
      testidPrefix="totp-disable-dialog"
      cancelButton={{
        label: t('totp.cancel'),
        variant: 'outline',
        disabled: loading,
        autoFocus: false,
      }}
      buttons={[
        {
          label: loading
            ? t('totp.disable.disabling')
            : t('totp.disable.confirm'),
          actionType: 'confirm',
          variant: 'destructive',
          autoFocus: true,
          loading,
          disabled: loading || !currentPassword || !code,
        },
      ]}
      onClose={resetForm}
      onConfirm={handleDisable}
    >
      <div className="space-y-3 pt-2">
        <FormInput
          autoFocus={true}
          label={t('totp.disable.passwordPlaceholder')}
          name="current-password"
          type="password"
          value={currentPassword}
          onChange={setCurrentPassword}
          placeholder={t('totp.disable.passwordPlaceholder')}
          autoComplete="current-password"
          error={fieldErrors.currentPassword}
          allowedHotkeys={DIALOG_INPUT_ALLOWED_HOTKEYS}
          testid="totp-disable-password"
        />
        <FormInput
          label={t('totp.disable.codePlaceholder')}
          name="totp-code"
          type="text"
          value={code}
          onChange={setCode}
          placeholder={t('totp.disable.codePlaceholder')}
          autoComplete="one-time-code"
          error={fieldErrors.code}
          allowedHotkeys={DIALOG_INPUT_ALLOWED_HOTKEYS}
          testid="totp-disable-code"
        />
      </div>
    </BaseDialog>
  )
}
