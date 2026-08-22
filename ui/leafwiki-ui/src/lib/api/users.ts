import { fetchWithAuth } from './auth'

export type User = {
  id: string
  username: string
  email: string
  role: 'admin' | 'editor' | 'viewer'
  totpEnabled: boolean
  // True for a user created via inviteUser who hasn't accepted their invite
  // yet (see the backend's UserService.InviteUser) — never set on a
  // password-created user.
  mustSetPassword: boolean
}

// UserInput is the writable subset of User: totpEnabled/mustSetPassword are
// only ever changed via dedicated endpoints, never through create/update.
type UserInput = Pick<User, 'username' | 'email' | 'role'>

export type InviteUserResponse = {
  user: User
  emailSent: boolean
}

export async function getUsers(): Promise<User[]> {
  return (await fetchWithAuth('/api/users')) as User[]
}

export async function createUser(user: UserInput & { password: string }) {
  return await fetchWithAuth('/api/users', {
    method: 'POST',
    body: JSON.stringify(user),
  })
}

// inviteUser creates a user with no password an admin ever sees — the
// backend generates one internally — and emails an invite link. emailSent
// can be false even on success (user creation never rolls back on a send
// failure): the caller should offer resendInvite when that happens.
export async function inviteUser(user: UserInput): Promise<InviteUserResponse> {
  return (await fetchWithAuth('/api/users/invite', {
    method: 'POST',
    body: JSON.stringify(user),
  })) as InviteUserResponse
}

export async function resendInvite(id: string) {
  return await fetchWithAuth(`/api/users/${id}/invite/resend`, {
    method: 'POST',
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
