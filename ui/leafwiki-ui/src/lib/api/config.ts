import type { ApiLocalizedErrorResponse } from './errors'
import { API_BASE_URL } from '../config'
import {
  ApiLocalizedError,
  isApiLocalizedErrorResponse,
  mapApiError,
} from './errors'

type ConfigErrorResponse = {
  error?: string | ApiLocalizedErrorResponse['error']
  message?: string
}

export type Config = {
  publicAccess: boolean
  hideLinkMetadataSection: boolean
  authDisabled: boolean
  maxAssetUploadSizeBytes: number
  enableRevision: boolean
  enableLinkRefactor: boolean
  gitBackupEnabled: boolean
  httpRemoteUserEnabled: boolean
  httpRemoteUserLogoutUrl: string
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

    if (isApiLocalizedErrorResponse(errorBody)) {
      throw new Error(
        mapApiError(new ApiLocalizedError(errorBody.error), fallbackMessage)
          .message,
      )
    }

    if (typeof errorBody?.error === 'string') {
      throw new Error(errorBody.error)
    }

    if (errorBody?.message) {
      throw new Error(errorBody.message)
    }

    throw new Error(fallbackMessage)
  }
  return await res.json()
}
