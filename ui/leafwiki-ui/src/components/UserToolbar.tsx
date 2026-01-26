import { Avatar, AvatarFallback } from '@/components/ui/avatar'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { DIALOG_CHANGE_OWN_PASSWORD } from '@/lib/registries'
import { useConfigStore } from '@/stores/config'
import { useDialogsStore } from '@/stores/dialogs'
import { useSessionStore } from '@/stores/session'
import { useNavigate } from 'react-router-dom'
import { RoleGuard } from './RoleGuard'

export default function UserToolbar() {
  const user = useSessionStore((s) => s.user)
  const logout = useSessionStore((s) => s.logout)
  const navigate = useNavigate()
  const openDialog = useDialogsStore((state) => state.openDialog)
  const authDisabled = useConfigStore((s) => s.authDisabled)

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

  const handleLogout = () => {
    logout()
    navigate('/login')
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
          </RoleGuard>
          <DropdownMenuItem
            className="cursor-pointer"
            onClick={() => openDialog(DIALOG_CHANGE_OWN_PASSWORD)}
          >
            Change Own Password
          </DropdownMenuItem>
          <DropdownMenuItem
            className="cursor-pointer"
            onClick={handleLogout}
            data-testid="user-toolbar-logout"
          >
            Logout
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  )
}
