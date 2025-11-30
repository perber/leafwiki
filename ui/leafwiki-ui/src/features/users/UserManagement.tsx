import { useUserStore } from '@/stores/users'
import { useEffect, useState } from 'react'
import { toast } from 'sonner'
import { ChangePasswordButton } from './ChangePasswordButton'
import { CreateEditUserButton } from './CreateEditUserButton'
import { DeleteUserButton } from './DeleteUserButton'

export default function UserManagement() {
  const { users, loadUsers, reset } = useUserStore()
  const [loading, setLoading] = useState(true)

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
      <title>User Management - LeafWiki</title>
      <div className="user-management">
        <h1 className="user-management__title">User Management</h1>

        <div className="user-management__header-actions">
          <CreateEditUserButton />
        </div>

        <div className="user-management__table-card">
          <div className="user-management__table-scroll">
            <table className="user-management__table">
              <thead className="user-management__table-head">
                <tr>
                  <th className="user-management__table-header-cell">
                    Username
                  </th>
                  <th className="user-management__table-header-cell">Email</th>
                  <th className="user-management__table-header-cell">Role</th>
                  <th className="user-management__table-header-cell">
                    Actions
                  </th>
                </tr>
              </thead>
              <tbody>
                {loading && (
                  <tr>
                    <td
                      colSpan={4}
                      className="user-management__table-body-message"
                    >
                      Loading users...
                    </td>
                  </tr>
                )}
                {!loading && users.length === 0 && (
                  <tr>
                    <td
                      colSpan={4}
                      className="user-management__table-body-message"
                    >
                      No users found.
                    </td>
                  </tr>
                )}
                {!loading &&
                  users.length > 0 &&
                  users.map((user) => (
                    <tr key={user.id} className="user-management__table-row">
                      <td className="user-management__table-cell">
                        {user.username}
                      </td>
                      <td className="user-management__table-cell">
                        {user.email}
                      </td>
                      <td className="user-management__table-cell">
                        <span
                          className={`user-management__role-pill ${
                            user.role === 'admin'
                              ? 'user-management__role-pill--admin'
                              : 'user-management__role-pill--default'
                          }`}
                        >
                          {user.role}
                        </span>
                      </td>
                      <td className="user-management__actions-cell">
                        <div className="user-management__actions">
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
