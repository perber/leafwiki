import { getConfig } from '@/lib/api/config'
import {
  DEFAULT_AVATAR_ALLOWED_EXTS,
  DEFAULT_MAX_ASSET_UPLOAD_SIZE_BYTES,
  DEFAULT_MAX_AVATAR_UPLOAD_SIZE_BYTES,
} from '@/lib/config'
import i18next, { getAvailableLanguages } from '@/lib/i18n'
import { sleep } from '@/lib/sleep'
import { create } from 'zustand'

type ConfigStore = {
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
  error: string | null
  loading: boolean
  hasLoaded: boolean
  // True only after a *successful* /api/config fetch — distinct from
  // hasLoaded, which also becomes true after a failed attempt (so the UI can
  // stop showing the boot spinner). Consumers that would otherwise treat a
  // failed session-token refresh as confirmed-unauthorized (and force a
  // logout) must gate that reaction on this, not on hasLoaded — while the
  // auth mode is unconfirmed, a refresh failure could just as easily mean
  // "this is actually a header-auth deployment with no refresh cookie" as
  // "the session really expired," and guessing wrong destroys a valid
  // header-auth session (see lib/api/auth.ts's fetchWithAuth).
  configLoadSucceeded: boolean
  loadConfig: () => Promise<void>
}

// A single failed attempt would otherwise leave configLoadSucceeded false —
// and the auth mode unconfirmed — for the tab's whole life (loadConfig only
// ever runs once, from App.tsx's mount effect). Bounded retry shrinks that
// window to a few seconds for the common transient-blip case.
const CONFIG_LOAD_MAX_ATTEMPTS = 3
const CONFIG_LOAD_RETRY_DELAYS_MS = [500, 1000]

export const useConfigStore = create<ConfigStore>((set) => ({
  publicAccess: false,
  editorLimit: 0,
  hideLinkMetadataSection: false,
  authDisabled: false,
  maxAssetUploadSizeBytes: DEFAULT_MAX_ASSET_UPLOAD_SIZE_BYTES,
  maxAvatarUploadSizeBytes: DEFAULT_MAX_AVATAR_UPLOAD_SIZE_BYTES,
  avatarAllowedExts: DEFAULT_AVATAR_ALLOWED_EXTS,
  enableRevision: false,
  enableLinkRefactor: false,
  enableApiKeyManagement: false,
  gitBackupEnabled: false,
  snapshotEnabled: false,
  smtpEnabled: false,
  totpAvailable: false,
  httpRemoteUserEnabled: false,
  loginUrl: '',
  logoutUrl: '',
  userManagementUrl: '',
  defaultLanguage: '',
  error: null,
  loading: false,
  hasLoaded: false,
  configLoadSucceeded: false,

  loadConfig: async () => {
    set({ loading: true, error: null })

    for (let attempt = 1; attempt <= CONFIG_LOAD_MAX_ATTEMPTS; attempt++) {
      try {
        const config = await getConfig()
        const maxAssetUploadSizeBytes = Number.isFinite(
          config.maxAssetUploadSizeBytes,
        )
          ? config.maxAssetUploadSizeBytes
          : DEFAULT_MAX_ASSET_UPLOAD_SIZE_BYTES
        const maxAvatarUploadSizeBytes = Number.isFinite(
          config.maxAvatarUploadSizeBytes,
        )
          ? config.maxAvatarUploadSizeBytes
          : DEFAULT_MAX_AVATAR_UPLOAD_SIZE_BYTES
        const avatarAllowedExts =
          Array.isArray(config.avatarAllowedExts) &&
          config.avatarAllowedExts.length > 0
            ? config.avatarAllowedExts
            : DEFAULT_AVATAR_ALLOWED_EXTS

        set({
          publicAccess: config.publicAccess,
          editorLimit: config.editorLimit ?? 0,
          hideLinkMetadataSection: config.hideLinkMetadataSection,
          authDisabled: config.authDisabled,
          maxAssetUploadSizeBytes,
          maxAvatarUploadSizeBytes,
          avatarAllowedExts,
          enableRevision: config.enableRevision ?? false,
          enableLinkRefactor: config.enableLinkRefactor ?? false,
          enableApiKeyManagement: config.enableApiKeyManagement ?? false,
          gitBackupEnabled: config.gitBackupEnabled ?? false,
          snapshotEnabled: config.snapshotEnabled ?? false,
          smtpEnabled: config.smtpEnabled ?? false,
          totpAvailable: config.totpAvailable ?? false,
          httpRemoteUserEnabled: config.httpRemoteUserEnabled ?? false,
          loginUrl: config.loginUrl ?? '',
          logoutUrl: config.logoutUrl ?? '',
          userManagementUrl: config.userManagementUrl ?? '',
          defaultLanguage: config.defaultLanguage ?? '',
          error: null,
          hasLoaded: true,
          configLoadSucceeded: true,
          loading: false,
        })

        if (config.defaultLanguage) {
          const isKnownLanguage = getAvailableLanguages().some(
            (language) => language.code === config.defaultLanguage,
          )
          if (isKnownLanguage) {
            void i18next.changeLanguage(config.defaultLanguage)
          } else {
            console.warn(
              `Configured default language "${config.defaultLanguage}" is not shipped with the frontend, ignoring`,
            )
          }
        }
        return
      } catch (error) {
        if (attempt === CONFIG_LOAD_MAX_ATTEMPTS) {
          console.warn(
            `Error loading configuration (attempt ${attempt}/${CONFIG_LOAD_MAX_ATTEMPTS}, giving up):`,
            error,
          )
          set({
            error:
              error instanceof Error
                ? error.message
                : i18next.t('configLoad.errorFallback', { ns: 'common' }),
            hasLoaded: true,
            configLoadSucceeded: false,
            loading: false,
          })
          return
        }
        console.warn(
          `Error loading configuration (attempt ${attempt}/${CONFIG_LOAD_MAX_ATTEMPTS}, retrying):`,
          error,
        )
        await sleep(CONFIG_LOAD_RETRY_DELAYS_MS[attempt - 1])
      }
    }
  },
}))
