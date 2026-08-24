import type { ApiLocalizedErrorResponse } from './errors'
import i18next from '@/lib/i18n'
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
  editorLimit: number
  hideLinkMetadataSection: boolean
  authDisabled: boolean
  maxAssetUploadSizeBytes: number
  maxAvatarUploadSizeBytes: number
  avatarAllowedExts: string[]
  enableRevision: boolean
  enableLinkRefactor: boolean
  enableApiKeyManagement: boolean
  gitBackupEnabled: boolean
  snapshotEnabled: boolean
  smtpEnabled: boolean
  totpAvailable: boolean
  httpRemoteUserEnabled: boolean
  loginUrl: string
  logoutUrl: string
  userManagementUrl: string
  defaultLanguage: string
}

export async function getConfig(): Promise<Config> {
  const res = await fetch(`${API_BASE_URL}/api/config`)
  if (!res.ok) {
    const errorText = await res.text()
    const fallbackMessage = i18next.t('configLoad.fetchErrorFallback', {
      ns: 'common',
      status: res.status,
      statusText: res.statusText,
    })
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
