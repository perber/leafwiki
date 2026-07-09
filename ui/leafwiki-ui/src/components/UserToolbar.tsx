import { LanguageSwitcher } from '@/components/LanguageSwitcher'
import { Avatar, AvatarFallback } from '@/components/ui/avatar'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuShortcut,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import * as authAPI from '@/lib/api/auth'
import i18next from '@/lib/i18n'
import {
  DIALOG_CHANGE_OWN_PASSWORD,
  DIALOG_SHORTCUTS_HELP,
} from '@/lib/registries'
import { useTranslation } from 'react-i18next'
import { redirectToExternal } from '@/lib/redirectToExternal'
import {
  createHotkeyDefinition,
  getShortcutDisplayLabel,
} from '@/lib/shortcuts/shortcutCatalog'
import { useIsReadOnly } from '@/lib/useIsReadOnly'
import { useConfigStore } from '@/stores/config'
import { useDialogsStore } from '@/stores/dialogs'
import { useHotKeysStore } from '@/stores/hotkeys'
import { useSessionStore } from '@/stores/session'
import { Heart } from 'lucide-react'
import { useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { RoleGuard } from './RoleGuard'

const isMacOS =
  typeof navigator !== 'undefined' &&
  /Mac|iPhone|iPad|iPod/.test(navigator.platform)
const shortcutsDialogHotkeyLabel = getShortcutDisplayLabel(
  'shortcuts.help.open',
  isMacOS,
)

export default function UserToolbar() {
  const { t } = useTranslation(['auth', 'backup', 'common', 'users'])
  const supportPageUrl = 'https://leafwiki.com/support/'
  const user = useSessionStore((s) => s.user)
  const logout = useSessionStore((s) => s.logout)
  const navigate = useNavigate()
  const openDialog = useDialogsStore((state) => state.openDialog)
  const authDisabled = useConfigStore((s) => s.authDisabled)
  const readOnly = useIsReadOnly()
  const backupEnabled = useConfigStore((s) => s.gitBackupEnabled)
  const httpRemoteUserEnabled = useConfigStore((s) => s.httpRemoteUserEnabled)
  const registerHotkey = useHotKeysStore((state) => state.registerHotkey)
  const unregisterHotkey = useHotKeysStore((state) => state.unregisterHotkey)
  const logoutUrl = useConfigStore((s) => s.logoutUrl)
  const loginUrl = useConfigStore((s) => s.loginUrl)
  const userManagementUrl = useConfigStore((s) => s.userManagementUrl)

  useEffect(() => {
    if (!authDisabled && (!user || readOnly)) {
      return
    }

    const hotkey = createHotkeyDefinition('shortcuts.help.open', () =>
      openDialog(DIALOG_SHORTCUTS_HELP),
    )

    registerHotkey(hotkey)
    return () => unregisterHotkey(hotkey.keyCombo)
  }, [
    authDisabled,
    openDialog,
    readOnly,
    registerHotkey,
    unregisterHotkey,
    user,
  ])

  if (!user && !authDisabled) {
    return (
      <div className="user-toolbar">
        <LanguageSwitcher />
        <Button
          size="sm"
          onClick={() =>
            loginUrl ? redirectToExternal(loginUrl) : navigate('/login')
          }
        >
          {t('login.loginButton')}
        </Button>
      </div>
    )
  }

  if (authDisabled) {
    return (
      <div className="user-toolbar">
        <LanguageSwitcher />
        <span className="user-toolbar__not-logged-in">
          {t('login.publicEditor')}
        </span>
      </div>
    )
  }

  const handleLogout = async () => {
    if (logoutUrl) {
      // Redirect immediately instead of clearing local session state first —
      // clearing it here would flash the local login screen before the
      // browser navigates away (see plans/logout-flash-and-external-user-management.md).
      authAPI.logout().catch(() => {})
      redirectToExternal(logoutUrl)
      return
    }
    await logout()
    navigate('/login')
  }

  return (
    <div className="user-toolbar">
      <LanguageSwitcher />
      <DropdownMenu>
        <DropdownMenuTrigger className="user-toolbar__dropdown-trigger">
          <Avatar
            className="user-toolbar__avatar"
            data-testid="user-toolbar-avatar"
          >
            <AvatarFallback className="user-toolbar__avatar-fallback">
              {user?.username[0].toUpperCase()}
            </AvatarFallback>
          </Avatar>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          <RoleGuard roles={['admin']}>
            {userManagementUrl ? (
              <DropdownMenuItem asChild className="cursor-pointer">
                <a
                  href={userManagementUrl}
                  target="_blank"
                  rel="noopener noreferrer"
                >
                  {t('userMenu.userManagement')}
                </a>
              </DropdownMenuItem>
            ) : (
              <DropdownMenuItem
                className="cursor-pointer"
                onClick={() => navigate('/users')}
              >
                {t('userMenu.userManagement')}
              </DropdownMenuItem>
            )}
            <DropdownMenuItem
              className="cursor-pointer"
              onClick={() => navigate('/settings/branding')}
            >
              {t('settings.branding', { ns: 'common' })}
            </DropdownMenuItem>
            <DropdownMenuItem
              className="cursor-pointer"
              onClick={() => navigate('/settings/importer')}
            >
              {t('settings.import', { ns: 'common' })}
            </DropdownMenuItem>
            {backupEnabled && (
              <DropdownMenuItem
                className="cursor-pointer"
                onClick={() => navigate('/settings/backup')}
              >
                {t('menuLabel', { ns: 'backup' })}
              </DropdownMenuItem>
            )}
            <DropdownMenuItem
              className="cursor-pointer"
              onClick={() => navigate('/settings/maintenance')}
            >
              {t('settings.maintenance', { ns: 'common' })}
            </DropdownMenuItem>
            <DropdownMenuSeparator />
          </RoleGuard>
          <DropdownMenuLabel className="text-muted-foreground text-xs font-normal">
            {t('version', { ns: 'common', version: __APP_VERSION__ })}
          </DropdownMenuLabel>
          <RoleGuard roles={['admin', 'editor']}>
            <DropdownMenuSeparator />
            <DropdownMenuItem
              className="cursor-pointer"
              onClick={() => openDialog(DIALOG_SHORTCUTS_HELP)}
            >
              {i18next.t('shortcutsHelp.menuItem', { ns: 'viewer' })}
              <DropdownMenuShortcut>
                {shortcutsDialogHotkeyLabel}
              </DropdownMenuShortcut>
            </DropdownMenuItem>
          </RoleGuard>
          <DropdownMenuItem
            className="cursor-pointer"
            onClick={() => openDialog(DIALOG_CHANGE_OWN_PASSWORD)}
          >
            {t('changeOwnPasswordTitle', { ns: 'users' })}
          </DropdownMenuItem>
          {(!httpRemoteUserEnabled || logoutUrl) && (
            <DropdownMenuItem
              className="cursor-pointer"
              onClick={handleLogout}
              data-testid="user-toolbar-logout"
            >
              {t('userMenu.logout')}
            </DropdownMenuItem>
          )}
          <RoleGuard roles={['admin']}>
            <DropdownMenuSeparator />
            <DropdownMenuItem
              asChild
              className="text-muted-foreground hover:text-foreground cursor-pointer gap-2"
            >
              <a
                href={supportPageUrl}
                target="_blank"
                rel="noopener noreferrer"
              >
                <Heart className="size-3.5 shrink-0" />
                <span>{t('supportLeafWiki', { ns: 'common' })}</span>
              </a>
            </DropdownMenuItem>
          </RoleGuard>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  )
}
