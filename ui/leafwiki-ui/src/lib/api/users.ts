import { fetchWithAuth } from './auth'

export type User = {
  id: string
  username: string
  email: string
  role: 'admin' | 'editor' | 'viewer'
  totpEnabled: boolean
}

// UserInput is the writable subset of User: totpEnabled is only ever changed
// via the dedicated /api/users/me/totp/* endpoints, never through create/update.
type UserInput = Pick<User, 'username' | 'email' | 'role'>

export async function getUsers(): Promise<User[]> {
  return (await fetchWithAuth('/api/users')) as User[]
}

export async function createUser(user: UserInput & { password: string }) {
  return await fetchWithAuth('/api/users', {
    method: 'POST',
    body: JSON.stringify(user),
  })
}

export async function updateUser(
  user: UserInput & { id: string; password?: string },
) {
  return await fetchWithAuth(`/api/users/${user.id}`, {
    method: 'PUT',
    body: JSON.stringify(user),
  })
}

export async function changeOwnPassword(
  oldPassword: string,
  newPassword: string,
) {
  return await fetchWithAuth(`/api/users/me/password`, {
    method: 'PUT',
    body: JSON.stringify({
      oldPassword,
      newPassword,
    }),
  })
}

export async function deleteUser(id: string) {
  return await fetchWithAuth(`/api/users/${id}`, {
    method: 'DELETE',
  })
}
