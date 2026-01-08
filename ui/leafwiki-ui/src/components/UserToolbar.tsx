import { Avatar, AvatarFallback } from '@/components/ui/avatar'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { DIALOG_CHANGE_OWN_PASSWORD } from '@/lib/registries'
import { useAuthStore } from '@/stores/auth'
import { useDialogsStore } from '@/stores/dialogs'
import { useNavigate } from 'react-router-dom'
import { RoleGuard } from './RoleGuard'

export default function UserToolbar() {
  const user = useAuthStore((s) => s.user)
  const logout = useAuthStore((s) => s.logout)
  const navigate = useNavigate()
  const openDialog = useDialogsStore((state) => state.openDialog)

  if (!user) {
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
              {user.username[0].toUpperCase()}
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
