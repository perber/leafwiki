import { fetchWithAuth } from './auth'

export type UserSettings = {
  userId: string
  language: string
  autoSave: boolean
  dateFormat: string
  timeFormat: string
  updatedAt: string
}

export type UserSettingsPatch = {
  language?: string
  autoSave?: boolean
  dateFormat?: string
  timeFormat?: string
}

export async function getUserSettings(): Promise<UserSettings> {
  return await fetchWithAuth<UserSettings>('/api/user-settings')
}

export async function updateUserSettings(
  patch: UserSettingsPatch,
): Promise<UserSettings> {
  return await fetchWithAuth<UserSettings>('/api/user-settings', {
    method: 'PUT',
    body: JSON.stringify(patch),
  })
}
