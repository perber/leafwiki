import { fetchWithAuth } from './auth'

export type UserSettings = {
  userId: string
  language: string
  autoSave: boolean
  updatedAt: string
}

export type UserSettingsPatch = {
  language?: string
  autoSave?: boolean
}

export async function getUserSettings(): Promise<UserSettings> {
  return (await fetchWithAuth('/api/user-settings')) as UserSettings
}

export async function updateUserSettings(
  patch: UserSettingsPatch,
): Promise<UserSettings> {
  return (await fetchWithAuth('/api/user-settings', {
    method: 'PUT',
    body: JSON.stringify(patch),
  })) as UserSettings
}
