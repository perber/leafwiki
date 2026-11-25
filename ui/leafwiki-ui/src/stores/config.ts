import { getConfig } from '@/lib/api/config'
import {
  setPublicAccess as setPublicAccessApi,
  updatePageDisplay as updatePageDisplayApi,
} from '@/lib/api/instanceSettings'
import {
  DEFAULT_AVATAR_ALLOWED_EXTS,
  DEFAULT_MAX_ASSET_UPLOAD_SIZE_BYTES,
  DEFAULT_MAX_AVATAR_UPLOAD_SIZE_BYTES,
} from '@/lib/config'
import i18next, { getAvailableLanguages } from '@/lib/i18n'
import {
  DEFAULT_PAGE_DISPLAY,
  parsePageDisplay,
  type PageDisplay,
  type PageDisplayPatch,
} from '@/lib/pageDisplay'
import { sleep } from '@/lib/sleep'
import { create } from 'zustand'

type ConfigStore = {
  publicAccess: boolean
  publicAccessEnvManaged: boolean
  // Instance-wide page display settings (Settings > Display).
  pageDisplay: PageDisplay
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
  gitBackupEnvManaged: boolean
  gitBackupConfigured: boolean
  snapshotEnabled: boolean
  smtpEnabled: boolean
  totpAvailable: boolean
  httpRemoteUserEnabled: boolean
  loginUrl: string
  logoutUrl: string
  userManagementUrl: string
  defaultLanguage: string
  oidcEnabled: boolean
  oidcButtonLabel: string
  oidcOnly: boolean
  oidcClientId: string
  oidcEndSessionUrl: string
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
  // Runtime toggle of public mode (admin only, settings-managed instances).
  // Updates the store in place on success — no reload needed for this tab;
  // other open tabs / anonymous visitors pick it up on their next
  // /api/config load.
  setPublicAccess: (enabled: boolean) => Promise<void>
  // Runtime change of any page display setting (admin only). Stores the
  // server-confirmed values of the patched fields; on failure the old ones
  // stay.
  updatePageDisplay: (patch: PageDisplayPatch) => Promise<void>
}

// A single failed attempt would otherwise leave configLoadSucceeded false —
// and the auth mode unconfirmed — for the tab's whole life (loadConfig only
// ever runs once, from App.tsx's mount effect). Bounded retry shrinks that
// window to a few seconds for the common transient-blip case.
const CONFIG_LOAD_MAX_ATTEMPTS = 3
const CONFIG_LOAD_RETRY_DELAYS_MS = [500, 1000]

export const useConfigStore = create<ConfigStore>((set) => ({
  publicAccess: false,
  publicAccessEnvManaged: false,
  pageDisplay: DEFAULT_PAGE_DISPLAY,
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
  gitBackupEnvManaged: false,
  gitBackupConfigured: false,
  snapshotEnabled: false,
  smtpEnabled: false,
  totpAvailable: false,
  httpRemoteUserEnabled: false,
  loginUrl: '',
  logoutUrl: '',
  userManagementUrl: '',
  defaultLanguage: '',
  oidcEnabled: false,
  oidcButtonLabel: '',
  oidcOnly: false,
  oidcClientId: '',
  oidcEndSessionUrl: '',
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
          publicAccessEnvManaged: config.publicAccessEnvManaged ?? false,
          pageDisplay: parsePageDisplay(config),
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
          gitBackupEnvManaged: config.gitBackupEnvManaged ?? false,
          gitBackupConfigured: config.gitBackupConfigured ?? false,
          snapshotEnabled: config.snapshotEnabled ?? false,
          smtpEnabled: config.smtpEnabled ?? false,
          totpAvailable: config.totpAvailable ?? false,
          httpRemoteUserEnabled: config.httpRemoteUserEnabled ?? false,
          loginUrl: config.loginUrl ?? '',
          logoutUrl: config.logoutUrl ?? '',
          userManagementUrl: config.userManagementUrl ?? '',
          defaultLanguage: config.defaultLanguage ?? '',
          oidcEnabled: config.oidcEnabled ?? false,
          oidcButtonLabel: config.oidcButtonLabel ?? '',
          oidcOnly: config.oidcOnly ?? false,
          oidcClientId: config.oidcClientId ?? '',
          oidcEndSessionUrl: config.oidcEndSessionUrl ?? '',
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

  setPublicAccess: async (enabled: boolean) => {
    const res = await setPublicAccessApi(enabled)
    set({ publicAccess: res.enabled })
  },

  updatePageDisplay: async (patch: PageDisplayPatch) => {
    const confirmed = parsePageDisplay(await updatePageDisplayApi(patch))
    // Take only the patched fields from the reply. The settings page can
    // have a TOC and a width save in flight at once; a reply that arrives
    // late carries a stale copy of the other field and must not undo it.
    const fromReply = Object.fromEntries(
      (Object.keys(patch) as (keyof PageDisplay)[])
        .filter((key) => patch[key] !== undefined)
        .map((key) => [key, confirmed[key]]),
    ) as PageDisplayPatch
    set((state) => ({ pageDisplay: { ...state.pageDisplay, ...fromReply } }))
  },
}))
