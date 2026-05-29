import { Button } from '@/components/ui/button'
import { fetchOAuthApprovalDetails, OAuthApprovalDetails } from '@/lib/api/oauth'
import { withBasePath } from '@/lib/routePath'
import { useBrandingStore } from '@/stores/branding'
import { useSessionStore } from '@/stores/session'
import { useEffect, useMemo, useState } from 'react'
import { useSearchParams } from 'react-router-dom'

const authorizationFieldNames = new Set([
  'client_id',
  'response_type',
  'redirect_uri',
  'scope',
  'state',
  'code_challenge',
  'code_challenge_method',
  'resource',
])

export default function OAuthApproval() {
  const [searchParams] = useSearchParams()
  const user = useSessionStore((s) => s.user)
  const { siteName, logoFile, logoVersion } = useBrandingStore()
  const [details, setDetails] = useState<OAuthApprovalDetails | null>(null)
  const [error, setError] = useState<string | null>(null)

  const approvalToken = searchParams.get('approval_token') || ''
  const hiddenFields = useMemo(
    () =>
      Array.from(searchParams.entries()).filter(([name]) =>
        authorizationFieldNames.has(name),
      ),
    [searchParams],
  )

  useEffect(() => {
    if (!approvalToken) {
      return
    }

    let cancelled = false
    fetchOAuthApprovalDetails(approvalToken)
      .then((nextDetails) => {
        if (!cancelled) {
          setError(null)
          setDetails(nextDetails)
        }
      })
      .catch(() => {
        if (!cancelled) setError('Approval request expired or is no longer valid')
      })

    return () => {
      cancelled = true
    }
  }, [approvalToken])

  return (
    <>
      <title>Authorize MCP - {siteName}</title>
      <div className="oauth-approval">
        <form method="post" action={withBasePath('/oauth/authorize')} className="oauth-approval__form">
          <div className="oauth-approval__brand">
            {logoFile ? (
              <img
                src={`${withBasePath(`/branding/${logoFile}`)}?v=${logoVersion}`}
                alt={siteName}
                className="oauth-approval__logo-image"
              />
            ) : (
              <span>🌿</span>
            )}{' '}
            {siteName}
          </div>

          <h1 className="oauth-approval__title">Authorize MCP access</h1>

          <p className="oauth-approval__prompt">
            Allow {details?.clientLabel || 'this MCP client'} to access {siteName}
            {user?.username ? ` as ${user.username}` : ''}?
          </p>

          {details ? (
            <dl className="oauth-approval__details">
              <dt>Client ID</dt>
              <dd>{details.clientId}</dd>
              <dt>Redirect URI</dt>
              <dd>{details.redirectUri}</dd>
              <dt>Scope</dt>
              <dd>{details.scope}</dd>
              <dt>Resource</dt>
              <dd>{details.resource}</dd>
            </dl>
          ) : (
            <p className="oauth-approval__status">
              {!approvalToken ? 'Invalid approval request' : error || 'Loading approval request...'}
            </p>
          )}

          {hiddenFields.map(([name, value], index) => (
            <input
              key={`${name}-${index}`}
              type="hidden"
              name={name}
              value={value}
              readOnly
            />
          ))}
          <input type="hidden" name="approval_token" value={approvalToken} readOnly />

          <div className="oauth-approval__actions">
            <Button type="submit" name="decision" value="deny" variant="outline">
              Cancel
            </Button>
            <Button type="submit" name="decision" value="approve" disabled={!details}>
              Approve
            </Button>
          </div>
        </form>
      </div>
    </>
  )
}
