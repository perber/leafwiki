import { mapApiError } from '@/lib/api/errors'
import { useApiKeyStore } from '@/stores/apikeys'
import { useUserStore } from '@/stores/users'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
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
  const { t } = useTranslation('apikeys')
  const { apiKeys, loadApiKeys, reset } = useApiKeyStore()
  const { users, loadUsers } = useUserStore()
  const [loading, setLoading] = useState(true)
  useSetTitle({ title: t('page.title') })
  useToolbarActions()

  useEffect(() => {
    Promise.all([loadApiKeys(), loadUsers()])
      .catch((err) => {
        console.warn(err)
        const mapped = mapApiError(err, t('page.loadErrorFallback'))
        toast.error(mapped.message)
      })
      .finally(() => {
        setLoading(false)
      })

    return () => {
      reset()
    }
  }, [loadApiKeys, loadUsers, reset, t])

  const usernameFor = (userId: string) =>
    users.find((u) => u.id === userId)?.username ?? userId

  return (
    <div className="settings">
      <h1 className="settings__title">{t('page.heading')}</h1>

      <div className="settings__header-actions">
        <CreateApiKeyButton />
      </div>

      <div className="settings__table-card">
        <div className="settings__table-scroll">
          <table className="settings__table">
            <thead className="settings__table-head">
              <tr>
                <th className="settings__table-header-cell">
                  {t('table.name')}
                </th>
                <th className="settings__table-header-cell">
                  {t('table.owner')}
                </th>
                <th className="settings__table-header-cell">
                  {t('table.role')}
                </th>
                <th className="settings__table-header-cell">
                  {t('table.expires')}
                </th>
                <th className="settings__table-header-cell">
                  {t('table.lastUsed')}
                </th>
                <th className="settings__table-header-cell">
                  {t('table.actions')}
                </th>
              </tr>
            </thead>
            <tbody>
              {loading && (
                <tr>
                  <td colSpan={6} className="settings__table-body-message">
                    {t('table.loading')}
                  </td>
                </tr>
              )}
              {!loading && apiKeys.length === 0 && (
                <tr>
                  <td colSpan={6} className="settings__table-body-message">
                    {t('table.empty')}
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
                        {t(`role.${apiKey.role}`, {
                          defaultValue: apiKey.role,
                        })}
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
                            {t('table.revoked')}
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
