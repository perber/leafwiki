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
import { DIALOG_SHORTCUTS_HELP } from '@/lib/registries'
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
import { useNavigate } from 'react-router'
import { RoleGuard } from './RoleGuard'

const isMacOS =
  typeof navigator !== 'undefined' &&
  /Mac|iPhone|iPad|iPod/.test(navigator.platform)
const shortcutsDialogHotkeyLabel = getShortcutDisplayLabel(
  'shortcuts.help.open',
  isMacOS,
)

export default function UserMenu() {
  const { t } = useTranslation('auth')
  const supportPageUrl = 'https://leafwiki.com/support/'
  const user = useSessionStore((s) => s.user)
  const logout = useSessionStore((s) => s.logout)
  const navigate = useNavigate()
  const openDialog = useDialogsStore((state) => state.openDialog)
  const authDisabled = useConfigStore((s) => s.authDisabled)
  const readOnly = useIsReadOnly()
  const httpRemoteUserEnabled = useConfigStore((s) => s.httpRemoteUserEnabled)
  const registerHotkey = useHotKeysStore((state) => state.registerHotkey)
  const unregisterHotkey = useHotKeysStore((state) => state.unregisterHotkey)
  const logoutUrl = useConfigStore((s) => s.logoutUrl)
  const loginUrl = useConfigStore((s) => s.loginUrl)

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
      <div className="user-menu">
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
      <div className="user-menu">
        <span className="user-menu__not-logged-in">
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
    <div className="user-menu">
      <DropdownMenu>
        <DropdownMenuTrigger className="user-menu__dropdown-trigger">
          <Avatar className="user-menu__avatar" data-testid="user-menu-avatar">
            <AvatarFallback className="user-menu__avatar-fallback">
              {user?.username[0].toUpperCase()}
            </AvatarFallback>
          </Avatar>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          <DropdownMenuItem
            className="cursor-pointer"
            onClick={() => navigate('/settings')}
            data-testid="user-menu-settings"
          >
            {t('userMenu.settings')}
          </DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuLabel className="text-muted-foreground text-xs font-normal">
            {t('userMenu.version', { version: __APP_VERSION__ })}
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
          {(!httpRemoteUserEnabled || logoutUrl) && (
            <DropdownMenuItem
              className="cursor-pointer"
              onClick={handleLogout}
              data-testid="user-menu-logout"
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
                <span>{t('userMenu.supportLeafWiki')}</span>
              </a>
            </DropdownMenuItem>
          </RoleGuard>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  )
}
