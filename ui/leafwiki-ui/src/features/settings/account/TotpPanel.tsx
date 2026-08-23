import { FormInput } from '@/components/FormInput'
import { Button } from '@/components/ui/button'
import { mapApiError } from '@/lib/api/errors'
import { confirmTOTPSetup, disableTOTP, startTOTPSetup } from '@/lib/api/totp'
import { useSessionStore } from '@/stores/session'
import copy from 'copy-to-clipboard'
import { Loader2 } from 'lucide-react'
import { QRCodeSVG } from 'qrcode.react'
import { useCallback, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

type SetupStep = 'password' | 'verify' | 'recovery'

function TotpSetupFlow() {
  const { t } = useTranslation('users')
  const user = useSessionStore((s) => s.user)
  const setUser = useSessionStore((s) => s.setUser)

  const [step, setStep] = useState<SetupStep>('password')
  const [currentPassword, setCurrentPassword] = useState('')
  const [otpAuthUrl, setOtpAuthUrl] = useState('')
  const [manualKey, setManualKey] = useState('')
  const [code, setCode] = useState('')
  const [recoveryCodes, setRecoveryCodes] = useState<string[]>([])
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({})
  const [loading, setLoading] = useState(false)

  const handlePasswordStep = async () => {
    setLoading(true)
    try {
      const result = await startTOTPSetup(currentPassword)
      setOtpAuthUrl(result.otpAuthUrl)
      setManualKey(result.secret)
      setCurrentPassword('')
      setStep('verify')
    } catch (err) {
      setCurrentPassword('')
      setFieldErrors({
        currentPassword: mapApiError(err, t('totp.setup.errorFallback'))
          .message,
      })
    } finally {
      setLoading(false)
    }
  }

  const handleVerifyStep = async () => {
    if (!user) return
    setLoading(true)
    try {
      const result = await confirmTOTPSetup(code)
      setRecoveryCodes(result.recoveryCodes)
      setUser({ ...user, totpEnabled: true })
      setStep('recovery')
      toast.success(t('totp.setup.successToast'))
    } catch (err) {
      setCode('')
      setFieldErrors({
        code: mapApiError(err, t('totp.setup.errorFallback')).message,
      })
    } finally {
      setLoading(false)
    }
  }

  const handleCopyRecoveryCodes = () => {
    copy(recoveryCodes.join('\n'))
    toast.success(t('totp.setup.recoveryCodesCopied'))
  }

  if (!user) return null

  return (
    <div className="settings__field space-y-3" data-testid="totp-setup-panel">
      {step === 'password' && (
        <>
          <FormInput
            label={t('totp.setup.passwordPlaceholder')}
            name="current-password"
            type="password"
            value={currentPassword}
            onChange={setCurrentPassword}
            placeholder={t('totp.setup.passwordPlaceholder')}
            autoComplete="current-password"
            error={fieldErrors.currentPassword}
            testid="totp-setup-password"
          />
          <div className="settings__actions">
            <Button
              onClick={handlePasswordStep}
              disabled={loading || !currentPassword}
              data-testid="totp-setup-continue"
            >
              {loading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
              {loading ? t('totp.setup.continuing') : t('totp.setup.continue')}
            </Button>
          </div>
        </>
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
            label={t('totp.setup.codePlaceholder')}
            name="totp-code"
            type="text"
            value={code}
            onChange={setCode}
            placeholder={t('totp.setup.codePlaceholder')}
            autoComplete="one-time-code"
            error={fieldErrors.code}
            testid="totp-setup-code"
          />
          <div className="settings__actions">
            <Button
              onClick={handleVerifyStep}
              disabled={loading || code.length === 0}
              data-testid="totp-setup-enable"
            >
              {loading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
              {loading ? t('totp.setup.enabling') : t('totp.setup.enable')}
            </Button>
          </div>
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
  )
}

function TotpDisableFlow() {
  const { t } = useTranslation('users')
  const user = useSessionStore((s) => s.user)
  const setUser = useSessionStore((s) => s.setUser)

  const [currentPassword, setCurrentPassword] = useState('')
  const [code, setCode] = useState('')
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({})
  const [loading, setLoading] = useState(false)

  const resetForm = useCallback(() => {
    setCurrentPassword('')
    setCode('')
    setFieldErrors({})
  }, [])

  const handleDisable = async () => {
    if (!user) return
    setLoading(true)
    try {
      await disableTOTP(currentPassword, code)
      setUser({ ...user, totpEnabled: false })
      toast.success(t('totp.disable.successToast'))
      resetForm()
    } catch (err) {
      setCurrentPassword('')
      setCode('')
      const message = mapApiError(err, t('totp.disable.errorFallback')).message
      setFieldErrors({ currentPassword: message, code: message })
    } finally {
      setLoading(false)
    }
  }

  if (!user) return null

  return (
    <div className="settings__field space-y-3" data-testid="totp-disable-panel">
      <FormInput
        label={t('totp.disable.passwordPlaceholder')}
        name="current-password"
        type="password"
        value={currentPassword}
        onChange={setCurrentPassword}
        placeholder={t('totp.disable.passwordPlaceholder')}
        autoComplete="current-password"
        error={fieldErrors.currentPassword}
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
        testid="totp-disable-code"
      />
      <div className="settings__actions">
        <Button
          variant="destructive"
          onClick={handleDisable}
          disabled={loading || !currentPassword || !code}
          data-testid="totp-disable-confirm"
        >
          {loading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
          {loading ? t('totp.disable.disabling') : t('totp.disable.confirm')}
        </Button>
      </div>
    </div>
  )
}

export function TotpPanel() {
  const user = useSessionStore((s) => s.user)
  // Locked at mount, not reactive to user.totpEnabled: the two dialogs this
  // panel replaces were each opened independently, so completing the setup
  // flow (which flips totpEnabled to true partway through, before the
  // recovery-codes step) never used to yank the UI into the disable flow
  // mid-stream. Whichever flow a mount starts in, it stays in until the
  // panel remounts (e.g. navigating away from /settings/account and back).
  const [startedEnabled] = useState(() => user?.totpEnabled ?? false)

  if (!user) return null

  return startedEnabled ? <TotpDisableFlow /> : <TotpSetupFlow />
}
