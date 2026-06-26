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
import i18next from '@/lib/i18n'
import {
  DIALOG_CHANGE_OWN_PASSWORD,
  DIALOG_SHORTCUTS_HELP,
} from '@/lib/registries'
import { useTranslation } from 'react-i18next'
import {
  createHotkeyDefinition,
  getShortcutDisplayLabel,
} from '@/lib/shortcuts/shortcutCatalog'
import { useIsReadOnly } from '@/lib/useIsReadOnly'
import { useBackupStore } from '@/stores/backup'
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
  const { t } = useTranslation('auth')
  const supportPageUrl = 'https://leafwiki.com/support/'
  const user = useSessionStore((s) => s.user)
  const logout = useSessionStore((s) => s.logout)
  const navigate = useNavigate()
  const openDialog = useDialogsStore((state) => state.openDialog)
  const authDisabled = useConfigStore((s) => s.authDisabled)
  const readOnly = useIsReadOnly()
  const backupEnabled = useBackupStore((s) => s.enabled)
  const httpRemoteUserEnabled = useConfigStore((s) => s.httpRemoteUserEnabled)
  const registerHotkey = useHotKeysStore((state) => state.registerHotkey)
  const unregisterHotkey = useHotKeysStore((state) => state.unregisterHotkey)
  const httpRemoteUserLogoutUrl = useConfigStore(
    (s) => s.httpRemoteUserLogoutUrl,
  )

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
        <Button size="sm" onClick={() => navigate('/login')}>
          {t('login.loginButton')}
        </Button>
      </div>
    )
  }

  if (authDisabled) {
    return (
      <div className="user-toolbar">
        <span className="user-toolbar__not-logged-in">
          {t('login.publicEditor')}
        </span>
      </div>
    )
  }

  const handleLogout = async () => {
    await logout()
    if (httpRemoteUserLogoutUrl) {
      window.location.href = httpRemoteUserLogoutUrl
    } else {
      navigate('/login')
    }
  }

  return (
    <div className="user-toolbar">
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
            <DropdownMenuItem
              className="cursor-pointer"
              onClick={() => navigate('/users')}
            >
              User Management
            </DropdownMenuItem>
            <DropdownMenuItem
              className="cursor-pointer"
              onClick={() => navigate('/settings/branding')}
            >
              Branding Settings
            </DropdownMenuItem>
            <DropdownMenuItem
              className="cursor-pointer"
              onClick={() => navigate('/settings/importer')}
            >
              Import
            </DropdownMenuItem>
            {backupEnabled && (
              <DropdownMenuItem
                className="cursor-pointer"
                onClick={() => navigate('/settings/backup')}
              >
                Backup Settings
              </DropdownMenuItem>
            )}
            <DropdownMenuItem
              className="cursor-pointer"
              onClick={() => navigate('/settings/maintenance')}
            >
              Maintenance
            </DropdownMenuItem>
            <DropdownMenuSeparator />
          </RoleGuard>
          <DropdownMenuLabel className="text-muted-foreground text-xs font-normal">
            Version {__APP_VERSION__}
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
            Change Own Password
          </DropdownMenuItem>
          {(!httpRemoteUserEnabled || httpRemoteUserLogoutUrl) && (
            <DropdownMenuItem
              className="cursor-pointer"
              onClick={handleLogout}
              data-testid="user-toolbar-logout"
            >
              Logout
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
                <span>Support LeafWiki</span>
              </a>
            </DropdownMenuItem>
          </RoleGuard>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  )
}
