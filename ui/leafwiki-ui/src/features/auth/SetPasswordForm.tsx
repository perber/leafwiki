import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { acceptInvite, confirmPasswordReset } from '@/lib/api/auth'
import { handleFieldErrors } from '@/lib/handleFieldErrors'
import { useBrandingStore } from '@/stores/branding'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, Navigate, useNavigate, useSearchParams } from 'react-router'

type SetPasswordFormProps = {
  mode: 'reset' | 'invite'
}

// Mirrors coreauth.MinPasswordLength. The backend rejects a shorter password
// with a `validation_error` body; checking it here first spares the round
// trip and shows a localized message (the backend's field messages are not
// translated) — same reasoning as ChangeOwnPasswordPanel.
const MIN_PASSWORD_LENGTH = 8

// SetPasswordForm backs both /reset-password and /accept-invite: the two
// flows share everything except which endpoint they call and what happens on
// success — a reset does NOT log the user in (the backend just revoked every
// session for the account), while accepting an invite does (see
// acceptInvite's doc comment in lib/api/auth.ts).
export function SetPasswordForm({ mode }: SetPasswordFormProps) {
  const { t } = useTranslation('auth')
  const { siteName } = useBrandingStore()
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const token = searchParams.get('token') ?? ''

  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({})
  const [loading, setLoading] = useState(false)
  const [resetSucceeded, setResetSucceeded] = useState(false)

  const ns = mode === 'reset' ? 'resetPassword' : 'acceptInvite'

  if (!token) {
    return <Navigate to="/login" replace />
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setFieldErrors({})

    if (newPassword.length < MIN_PASSWORD_LENGTH) {
      setFieldErrors({ newPassword: t(`${ns}.passwordTooShort`) })
      return
    }

    // The backend only ever sees `newPassword`, so the confirm-match check has
    // no server-side counterpart and must run here.
    if (newPassword !== confirmPassword) {
      setFieldErrors({ confirmPassword: t(`${ns}.passwordsDoNotMatch`) })
      return
    }

    setLoading(true)
    try {
      if (mode === 'reset') {
        await confirmPasswordReset(token, newPassword)
        setResetSucceeded(true)
      } else {
        await acceptInvite(token, newPassword)
        // user already set in the store by acceptInvite
        navigate('/', { replace: true })
      }
    } catch (err) {
      // Routes a `validation_error` body to per-field messages (e.g. a
      // password the backend still rejects) and falls back to a toast for
      // anything else — an invalid/expired token, a network failure.
      handleFieldErrors(err, setFieldErrors, t(`${ns}.errorFallback`))
    } finally {
      setLoading(false)
    }
  }

  if (resetSucceeded) {
    return (
      <>
        <title>{t('resetPassword.pageTitle', { siteName })}</title>
        <div className="login">
          <div className="login__form">
            <h1 className="login__title">{t('resetPassword.successTitle')}</h1>
            <p className="login__totp-description">
              {t('resetPassword.successDescription')}
            </p>
            <Link to="/login">
              <Button variant="ghost" className="login__totp-back">
                {t('resetPassword.goToLogin')}
              </Button>
            </Link>
          </div>
        </div>
      </>
    )
  }

  return (
    <>
      <title>{t(`${ns}.pageTitle`, { siteName })}</title>
      <div className="login">
        <form onSubmit={handleSubmit} className="login__form">
          <h1 className="login__title">{t(`${ns}.title`)}</h1>
          {ns === 'acceptInvite' && (
            <p className="login__totp-description">
              {t('acceptInvite.description')}
            </p>
          )}

          <div className="login__field">
            <Input
              type="password"
              placeholder={t(`${ns}.newPasswordPlaceholder`)}
              value={newPassword}
              onChange={(e) => setNewPassword(e.target.value)}
              required
              name="new-password"
              autoComplete="new-password"
              autoFocus
              data-testid="set-password-new"
              spellCheck={false}
            />
            {fieldErrors.newPassword && (
              <p className="text-error mt-1 text-sm">
                {fieldErrors.newPassword}
              </p>
            )}
          </div>
          <div className="login__field">
            <Input
              type="password"
              placeholder={t(`${ns}.confirmPasswordPlaceholder`)}
              value={confirmPassword}
              onChange={(e) => setConfirmPassword(e.target.value)}
              required
              name="confirm-password"
              autoComplete="new-password"
              data-testid="set-password-confirm"
              spellCheck={false}
            />
            {fieldErrors.confirmPassword && (
              <p className="text-error mt-1 text-sm">
                {fieldErrors.confirmPassword}
              </p>
            )}
          </div>

          <Button
            type="submit"
            className="login__submit"
            disabled={loading}
            data-testid="set-password-submit"
          >
            {loading ? t(`${ns}.submitting`) : t(`${ns}.submit`)}
          </Button>
        </form>
      </div>
    </>
  )
}
