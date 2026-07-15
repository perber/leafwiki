import BaseDialog from '@/components/BaseDialog'
import { FormInput } from '@/components/FormInput'
import { Button } from '@/components/ui/button'
import { mapApiError } from '@/lib/api/errors'
import { confirmTOTPSetup, startTOTPSetup } from '@/lib/api/totp'
import { DIALOG_TOTP_SETUP } from '@/lib/registries'
import { useSessionStore } from '@/stores/session'
import copy from 'copy-to-clipboard'
import { useCallback, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { QRCodeSVG } from 'qrcode.react'
import { toast } from 'sonner'

const DIALOG_INPUT_ALLOWED_HOTKEYS = 'Enter'

type Step = 'password' | 'verify' | 'recovery'

export function TOTPSetupDialog() {
  const { t } = useTranslation('users')
  const user = useSessionStore((s) => s.user)
  const setUser = useSessionStore((s) => s.setUser)

  const [step, setStep] = useState<Step>('password')
  const [currentPassword, setCurrentPassword] = useState('')
  const [otpAuthUrl, setOtpAuthUrl] = useState('')
  const [manualKey, setManualKey] = useState('')
  const [code, setCode] = useState('')
  const [recoveryCodes, setRecoveryCodes] = useState<string[]>([])
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({})
  const [loading, setLoading] = useState(false)

  const resetForm = useCallback((): boolean => {
    setStep('password')
    setCurrentPassword('')
    setOtpAuthUrl('')
    setManualKey('')
    setCode('')
    setRecoveryCodes([])
    setFieldErrors({})
    return true
  }, [])

  if (!user) return null

  const handlePasswordStep = async (): Promise<boolean> => {
    setLoading(true)
    try {
      const result = await startTOTPSetup(currentPassword)
      setOtpAuthUrl(result.otpAuthUrl)
      setManualKey(result.secret)
      setCurrentPassword('')
      setStep('verify')
      return false
    } catch (err) {
      setCurrentPassword('')
      setFieldErrors({
        currentPassword: mapApiError(err, t('totp.setup.errorFallback'))
          .message,
      })
      return false
    } finally {
      setLoading(false)
    }
  }

  const handleVerifyStep = async (): Promise<boolean> => {
    setLoading(true)
    try {
      const result = await confirmTOTPSetup(code)
      setRecoveryCodes(result.recoveryCodes)
      setUser({ ...user, totpEnabled: true })
      setStep('recovery')
      toast.success(t('totp.setup.successToast'))
      return false
    } catch (err) {
      setCode('')
      setFieldErrors({
        code: mapApiError(err, t('totp.setup.errorFallback')).message,
      })
      return false
    } finally {
      setLoading(false)
    }
  }

  const handleConfirm = async (): Promise<boolean> => {
    if (step === 'password') return handlePasswordStep()
    if (step === 'verify') return handleVerifyStep()
    // 'recovery' step: nothing left to confirm, just close and reset.
    resetForm()
    return true
  }

  const handleCopyRecoveryCodes = () => {
    copy(recoveryCodes.join('\n'))
    toast.success(t('totp.setup.recoveryCodesCopied'))
  }

  const dialogTitle =
    step === 'password'
      ? t('totp.setup.passwordStepTitle')
      : step === 'verify'
        ? t('totp.setup.verifyStepTitle')
        : t('totp.setup.recoveryStepTitle')

  const dialogDescription =
    step === 'password'
      ? t('totp.setup.passwordStepDescription')
      : step === 'verify'
        ? t('totp.setup.verifyStepDescription')
        : t('totp.setup.recoveryStepDescription')

  const confirmLabel =
    step === 'password'
      ? loading
        ? t('totp.setup.continuing')
        : t('totp.setup.continue')
      : step === 'verify'
        ? loading
          ? t('totp.setup.enabling')
          : t('totp.setup.enable')
        : t('totp.setup.done')

  const confirmDisabled =
    loading ||
    (step === 'password' && !currentPassword) ||
    (step === 'verify' && code.length === 0)

  return (
    <BaseDialog
      dialogType={DIALOG_TOTP_SETUP}
      dialogTitle={dialogTitle}
      dialogDescription={dialogDescription}
      testidPrefix="totp-setup-dialog"
      cancelButton={{
        label: t('totp.cancel'),
        variant: 'outline',
        disabled: loading,
        autoFocus: false,
      }}
      buttons={[
        {
          label: confirmLabel,
          actionType: 'confirm',
          autoFocus: true,
          loading,
          disabled: confirmDisabled,
        },
      ]}
      onClose={resetForm}
      onConfirm={handleConfirm}
    >
      <div className="space-y-3 pt-2">
        {step === 'password' && (
          <FormInput
            autoFocus={true}
            label={t('totp.setup.passwordPlaceholder')}
            name="current-password"
            type="password"
            value={currentPassword}
            onChange={setCurrentPassword}
            placeholder={t('totp.setup.passwordPlaceholder')}
            autoComplete="current-password"
            error={fieldErrors.currentPassword}
            allowedHotkeys={DIALOG_INPUT_ALLOWED_HOTKEYS}
            testid="totp-setup-password"
          />
        )}

        {step === 'verify' && (
          <>
            <div className="totp-setup__qr">
              <QRCodeSVG value={otpAuthUrl} size={200} marginSize={2} />
            </div>
            <p className="totp-setup__manual-key">
              {t('totp.setup.manualKeyLabel')}:{' '}
              <code data-testid="totp-setup-manual-key">{manualKey}</code>
            </p>
            <FormInput
              autoFocus={true}
              label={t('totp.setup.codePlaceholder')}
              name="totp-code"
              type="text"
              value={code}
              onChange={setCode}
              placeholder={t('totp.setup.codePlaceholder')}
              autoComplete="one-time-code"
              error={fieldErrors.code}
              allowedHotkeys={DIALOG_INPUT_ALLOWED_HOTKEYS}
              testid="totp-setup-code"
            />
          </>
        )}

        {step === 'recovery' && (
          <>
            <pre
              className="totp-setup__recovery-codes"
              data-testid="totp-setup-recovery-codes"
            >
              {recoveryCodes.join('\n')}
            </pre>
            <Button
              type="button"
              variant="outline"
              onClick={handleCopyRecoveryCodes}
            >
              {t('totp.setup.copyRecoveryCodes')}
            </Button>
          </>
        )}
      </div>
    </BaseDialog>
  )
}
