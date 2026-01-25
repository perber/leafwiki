import { useUserStore } from '@/stores/users'
import { useEffect, useState } from 'react'
import { toast } from 'sonner'
import { useSetTitle } from '../viewer/setTitle'
import { ChangePasswordButton } from './ChangePasswordButton'
import { CreateEditUserButton } from './CreateEditUserButton'
import { DeleteUserButton } from './DeleteUserButton'
import { useToolbarActions } from './useToolbarActions'

export default function UserManagement() {
  const { users, loadUsers, reset } = useUserStore()
  const [loading, setLoading] = useState(true)
  useSetTitle({ title: 'User Management' })
  useToolbarActions()

  useEffect(() => {
    loadUsers()
      .catch((err) => {
        console.warn(err)
        toast.error('Error loading users')
      })
      .finally(() => {
        setLoading(false)
      })

    return () => {
      reset()
    }
  }, [loadUsers, reset])

  return (
    <>
      <div className="settings">
        <h1 className="settings__title">User Management</h1>

        <div className="settings__header-actions">
          <CreateEditUserButton />
        </div>

        <div className="settings__table-card">
          <div className="settings__table-scroll">
            <table className="settings__table">
              <thead className="settings__table-head">
                <tr>
                  <th className="settings__table-header-cell">Username</th>
                  <th className="settings__table-header-cell">Email</th>
                  <th className="settings__table-header-cell">Role</th>
                  <th className="settings__table-header-cell">Actions</th>
                </tr>
              </thead>
              <tbody>
                {loading && (
                  <tr>
                    <td colSpan={4} className="settings__table-body-message">
                      Loading users...
                    </td>
                  </tr>
                )}
                {!loading && users.length === 0 && (
                  <tr>
                    <td colSpan={4} className="settings__table-body-message">
                      No users found.
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
                      <td className="settings__actions-cell">
                        <div className="settings__actions">
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
