import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { acceptInvite, confirmPasswordReset } from '@/lib/api/auth'
import { mapApiError } from '@/lib/api/errors'
import { useBrandingStore } from '@/stores/branding'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, Navigate, useNavigate, useSearchParams } from 'react-router'
import { toast } from 'sonner'

type SetPasswordFormProps = {
  mode: 'reset' | 'invite'
}

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
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  const [resetSucceeded, setResetSucceeded] = useState(false)

  const ns = mode === 'reset' ? 'resetPassword' : 'acceptInvite'

  if (!token) {
    return <Navigate to="/login" replace />
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)

    if (newPassword !== confirmPassword) {
      setError(t(`${ns}.passwordsDoNotMatch`))
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
      const mapped = mapApiError(err, t(`${ns}.errorFallback`))
      toast.error(mapped.message)
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
          </div>
          {error && <p className="text-error mb-4 text-sm">{error}</p>}

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
