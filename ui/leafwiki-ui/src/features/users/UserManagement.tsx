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
      <div className="mx-auto max-w-4xl">
        <h1 className="mb-4 text-2xl font-bold">User Management</h1>
        <div className="flex justify-end">
          <CreateEditUserButton />
        </div>
        <div className="mt-4 rounded-md border shadow-sm">
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead className="bg-gray-100 text-left">
                <tr>
                  <th className="p-3">Username</th>
                  <th className="p-3">Email</th>
                  <th className="p-3">Role</th>
                  <th className="p-3">Actions</th>
                </tr>
              </thead>
              <tbody>
                {loading && (
                  <tr>
                    <td colSpan={4} className="p-4 text-center text-gray-500">
                      Loading users...
                    </td>
                  </tr>
                )}
                {!loading && users.length === 0 && (
                  <tr>
                    <td colSpan={4} className="p-4 text-center text-gray-500">
                      No users found.
                    </td>
                  </tr>
                )}
                {!loading &&
                  users.length > 0 &&
                  users.map((user) => (
                    <tr key={user.id} className="border-t">
                      <td className="p-3">{user.username}</td>
                      <td className="p-3">{user.email}</td>
                      <td className="p-3">
                        <span
                          className={`rounded px-2 py-1 text-xs font-medium ${user.role === 'admin' ? 'bg-indigo-100 text-indigo-700' : 'bg-gray-100 text-gray-700'}`}
                        >
                          {user.role}
                        </span>
                      </td>
                      <td className="p-3">
                        <div className="flex gap-2">
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
