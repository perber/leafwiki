import { fetchWithAuth } from './auth'

export type User = {
  id: string
  username: string
  email: string
  role: 'admin' | 'editor' | 'viewer'
}

export async function getUsers(): Promise<User[]> {
  try {
    return (await fetchWithAuth('/api/users')) as User[]
  } catch {
    throw new Error('User fetch failed')
  }
}

export async function createUser(
  user: Omit<User, 'id'> & { password: string },
) {
  return await fetchWithAuth('/api/users', {
    method: 'POST',
    body: JSON.stringify(user),
  })
}

export async function updateUser(user: User & { password?: string }) {
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
      new_password: newPassword,
      old_password: oldPassword,
    }),
  })
}

export async function deleteUser(id: string) {
  return await fetchWithAuth(`/api/users/${id}`, {
    method: 'DELETE',
  })
}
