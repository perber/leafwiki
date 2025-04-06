import { useUserStore } from "@/stores/users"
import { useEffect } from "react"
import { ChangePasswordDialog } from "./ChangePasswordDialog"
import { DeleteUserButton } from "./DeleteUserButton"
import { UserFormDialog } from "./UserFormDialog"
// import { UserFormDialog } from "./UserFormDialog"

export default function UserManagement() {
  const { users, loadUsers } = useUserStore()

  useEffect(() => {
    loadUsers()
  }, [loadUsers])

  return (
    <div className="max-w-4xl mx-auto">
      <h1 className="text-2xl font-bold mb-4">UserManagement</h1>
      <UserFormDialog />
      <div className="mt-4 border rounded-md overflow-hidden shadow">
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
            {users.map(user => (
              <tr key={user.id} className="border-t">
                <td className="p-3">{user.username}</td>
                <td className="p-3">{user.email}</td>
                <td className="p-3 capitalize">{user.role}</td>
                <td className="p-3 flex gap-2">
                    <UserFormDialog user={user} />
                    <ChangePasswordDialog userId={user.id} username={user.username} />
                    <DeleteUserButton userId={user.id} username={user.username} />
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}
