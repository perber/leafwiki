import { mapApiError } from '@/lib/api/errors'
import { useUserStore } from '@/stores/users'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { useSetTitle } from '../viewer/setTitle'
import { ChangePasswordButton } from './ChangePasswordButton'
import { CreateEditUserButton } from './CreateEditUserButton'
import { DeleteUserButton } from './DeleteUserButton'
import { ResendInviteButton } from './ResendInviteButton'

export default function UserManagement() {
  const { t } = useTranslation('users')
  const { users, loadUsers, reset } = useUserStore()
  const [loading, setLoading] = useState(true)
  useSetTitle({ title: t('pageTitle') })

  useEffect(() => {
    loadUsers()
      .catch((err) => {
        console.warn(err)
        const mapped = mapApiError(err, t('loadErrorFallback'))
        toast.error(mapped.message)
      })
      .finally(() => {
        setLoading(false)
      })

    return () => {
      reset()
    }
  }, [loadUsers, reset, t])

  return (
    <>
      <div className="settings">
        <h1 className="settings__title">{t('pageTitle')}</h1>

        <div className="settings__header-actions">
          <CreateEditUserButton />
        </div>

        <div className="settings__table-card">
          <div className="settings__table-scroll">
            <table className="settings__table">
              <thead className="settings__table-head">
                <tr>
                  <th className="settings__table-header-cell">
                    {t('columns.username')}
                  </th>
                  <th className="settings__table-header-cell">
                    {t('columns.email')}
                  </th>
                  <th className="settings__table-header-cell">
                    {t('columns.role')}
                  </th>
                  <th className="settings__table-header-cell">
                    {t('totp.column')}
                  </th>
                  <th className="settings__table-header-cell">
                    {t('columns.actions')}
                  </th>
                </tr>
              </thead>
              <tbody>
                {loading && (
                  <tr>
                    <td colSpan={5} className="settings__table-body-message">
                      {t('loading')}
                    </td>
                  </tr>
                )}
                {!loading && users.length === 0 && (
                  <tr>
                    <td colSpan={5} className="settings__table-body-message">
                      {t('noUsersFound')}
                    </td>
                  </tr>
                )}
                {!loading &&
                  users.length > 0 &&
                  users.map((user) => (
                    <tr key={user.id} className="settings__table-row">
                      <td className="settings__table-cell">{user.username}</td>
                      <td className="settings__table-cell">{user.email}</td>
                      <td className="settings__table-cell">
                        <span
                          className={`settings__role-pill ${
                            user.role === 'admin'
                              ? 'settings__role-pill--admin'
                              : 'settings__role-pill--default'
                          }`}
                        >
                          {user.role}
                        </span>
                      </td>
                      <td className="settings__table-cell">
                        <span
                          className={`settings__pill ${
                            user.totpEnabled
                              ? 'settings__pill-success'
                              : 'settings__pill-warning'
                          }`}
                        >
                          {user.totpEnabled
                            ? t('totp.enabled')
                            : t('totp.disabled')}
                        </span>
                      </td>
                      <td className="settings__actions-cell">
                        <div className="settings__actions">
                          {user.mustSetPassword && (
                            <>
                              <span className="settings__pill settings__pill-warning">
                                {t('invite.pendingPill')}
                              </span>
                              <ResendInviteButton user={user} />
                            </>
                          )}
                          <CreateEditUserButton user={user} />
                          <ChangePasswordButton user={user} />
                          <DeleteUserButton user={user} />
                        </div>
                      </td>
                    </tr>
                  ))}
              </tbody>
            </table>
          </div>
        </div>
      </div>
    </>
  )
}
