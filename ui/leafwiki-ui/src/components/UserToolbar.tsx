import { Avatar, AvatarFallback } from '@/components/ui/avatar'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { DIALOG_CHANGE_OWN_PASSWORD } from '@/lib/registries'
import { useConfigStore } from '@/stores/config'
import { useDialogsStore } from '@/stores/dialogs'
import { useSessionStore } from '@/stores/session'
import { Heart } from 'lucide-react'
import { useNavigate } from 'react-router-dom'
import { RoleGuard } from './RoleGuard'

export default function UserToolbar() {
  const supportPageUrl = 'https://leafwiki.com/support/'
  const user = useSessionStore((s) => s.user)
  const logout = useSessionStore((s) => s.logout)
  const navigate = useNavigate()
  const openDialog = useDialogsStore((state) => state.openDialog)
  const authDisabled = useConfigStore((s) => s.authDisabled)
  const httpRemoteUserEnabled = useConfigStore((s) => s.httpRemoteUserEnabled)
  const httpRemoteUserLogoutUrl = useConfigStore(
    (s) => s.httpRemoteUserLogoutUrl,
  )

  if (!user && !authDisabled) {
    // renders the login
    return (
      <div className="user-toolbar">
        <span className="user-toolbar__not-logged-in">Not logged in</span>
        <button
          type="button"
          className="user-toolbar__login-button"
          onClick={() => navigate('/login')}
        >
          Login
        </button>
      </div>
    )
  }

  if (authDisabled) {
    return (
      <div className="user-toolbar">
        <span className="user-toolbar__not-logged-in">Public editor</span>
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
            <DropdownMenuItem
              className="cursor-pointer"
              onClick={() => navigate('/settings/backup')}
            >
              Backup Settings
            </DropdownMenuItem>
            <DropdownMenuSeparator />
          </RoleGuard>
          <DropdownMenuLabel className="text-muted-foreground text-xs font-normal">
            Version {__APP_VERSION__}
          </DropdownMenuLabel>
          <DropdownMenuSeparator />
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
          <RoleGuard roles={['admin', 'editor']}>
            <DropdownMenuSeparator />
            <DropdownMenuLabel className="text-muted-foreground max-w-56 px-2 pb-1 text-xs leading-relaxed font-normal whitespace-normal">
              Support ongoing development.
            </DropdownMenuLabel>
            <DropdownMenuItem
              className="bg-primary text-primary-foreground focus:bg-primary/90 focus:text-primary-foreground hover:bg-primary/90 hover:text-primary-foreground cursor-pointer justify-center gap-2 rounded-md py-2 font-medium shadow-sm"
              onClick={() =>
                window.open(supportPageUrl, '_blank', 'noopener,noreferrer')
              }
            >
              <Heart className="size-4 fill-current" />
              <span>Support LeafWiki</span>
            </DropdownMenuItem>
          </RoleGuard>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  )
}
