import { API_BASE_URL } from '../config'

type ConfigErrorResponse = {
  error?: string
  message?: string
}

export type Config = {
  publicAccess: boolean
  hideLinkMetadataSection: boolean
  authDisabled: boolean
  maxAssetUploadSizeBytes: number
  enableRevision: boolean
  enableLinkRefactor: boolean
}

export async function getConfig(): Promise<Config> {
  const res = await fetch(`${API_BASE_URL}/api/config`)
  if (!res.ok) {
    const errorText = await res.text()
    const fallbackMessage = `Could not load config: ${res.status} ${res.statusText}`
    let errorBody: ConfigErrorResponse | null = null

    try {
      errorBody = errorText
        ? (JSON.parse(errorText) as ConfigErrorResponse)
        : null
    } catch {
      throw new Error(fallbackMessage)
    }

    if (errorBody?.error || errorBody?.message) {
      throw new Error(errorBody.error || errorBody.message)
    }

    throw new Error(fallbackMessage)
  }
  return await res.json()
}
