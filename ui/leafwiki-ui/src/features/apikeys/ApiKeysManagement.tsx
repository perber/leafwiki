import { mapApiError } from '@/lib/api/errors'
import { useApiKeyStore } from '@/stores/apikeys'
import { useUserStore } from '@/stores/users'
import { useEffect, useState } from 'react'
import { toast } from 'sonner'
import { useSetTitle } from '../viewer/setTitle'
import { CreateApiKeyButton } from './CreateApiKeyButton'
import { DeleteApiKeyButton } from './DeleteApiKeyButton'
import { useToolbarActions } from './useToolbarActions'

function formatTimestamp(value?: string): string {
  if (!value) return '—'
  return new Date(value).toLocaleString()
}

export default function ApiKeysManagement() {
  const { apiKeys, loadApiKeys, reset } = useApiKeyStore()
  const { users, loadUsers } = useUserStore()
  const [loading, setLoading] = useState(true)
  useSetTitle({ title: 'API Keys' })
  useToolbarActions()

  useEffect(() => {
    Promise.all([loadApiKeys(), loadUsers()])
      .catch((err) => {
        console.warn(err)
        const mapped = mapApiError(err, 'Error loading API keys')
        toast.error(mapped.message)
      })
      .finally(() => {
        setLoading(false)
      })

    return () => {
      reset()
    }
  }, [loadApiKeys, loadUsers, reset])

  const usernameFor = (userId: string) =>
    users.find((u) => u.id === userId)?.username ?? userId

  return (
    <div className="settings">
      <h1 className="settings__title">API Keys</h1>

      <div className="settings__header-actions">
        <CreateApiKeyButton />
      </div>

      <div className="settings__table-card">
        <div className="settings__table-scroll">
          <table className="settings__table">
            <thead className="settings__table-head">
              <tr>
                <th className="settings__table-header-cell">Name</th>
                <th className="settings__table-header-cell">Owner</th>
                <th className="settings__table-header-cell">Role</th>
                <th className="settings__table-header-cell">Expires</th>
                <th className="settings__table-header-cell">Last used</th>
                <th className="settings__table-header-cell">Actions</th>
              </tr>
            </thead>
            <tbody>
              {loading && (
                <tr>
                  <td colSpan={6} className="settings__table-body-message">
                    Loading API keys...
                  </td>
                </tr>
              )}
              {!loading && apiKeys.length === 0 && (
                <tr>
                  <td colSpan={6} className="settings__table-body-message">
                    No API keys found.
                  </td>
                </tr>
              )}
              {!loading &&
                apiKeys.length > 0 &&
                apiKeys.map((apiKey) => (
                  <tr key={apiKey.id} className="settings__table-row">
                    <td className="settings__table-cell">{apiKey.name}</td>
                    <td className="settings__table-cell">
                      {usernameFor(apiKey.userId)}
                    </td>
                    <td className="settings__table-cell">
                      <span
                        className={`settings__role-pill ${
                          apiKey.role === 'admin'
                            ? 'settings__role-pill--admin'
                            : 'settings__role-pill--default'
                        }`}
                      >
                        {apiKey.role}
                      </span>
                    </td>
                    <td className="settings__table-cell">
                      {formatTimestamp(apiKey.expiresAt)}
                    </td>
                    <td className="settings__table-cell">
                      {formatTimestamp(apiKey.lastUsedAt)}
                    </td>
                    <td className="settings__actions-cell">
                      <div className="settings__actions">
                        {apiKey.revokedAt ? (
                          <span className="settings__table-body-message">
                            Revoked
                          </span>
                        ) : (
                          <DeleteApiKeyButton apiKey={apiKey} />
                        )}
                      </div>
                    </td>
                  </tr>
                ))}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  )
}
