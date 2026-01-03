import { API_BASE_URL } from '../config'

export type Config = {
  publicAccess: boolean
  hideLinkMetadataSection: boolean
  authDisabled: boolean
}

export async function getConfig(): Promise<Config> {
  const res = await fetch(`${API_BASE_URL}/api/config`)
  if (!res.ok)
    throw new Error(`Could not load config: ${res.status} ${res.statusText}`)
  return await res.json()
}
