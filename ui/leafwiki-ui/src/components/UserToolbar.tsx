import { Avatar, AvatarFallback } from '@/components/ui/avatar'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { ChangeOnwnPasswordDialog } from '@/features/users/ChangeOwnPasswordDialog'
import { useAuthStore } from '@/stores/auth'
import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { RoleGuard } from './RoleGuard'

export default function UserToolbar() {
  const user = useAuthStore((s) => s.user)
  const logout = useAuthStore((s) => s.logout)
  const navigate = useNavigate()
  const [dialogOpen, setDialogOpen] = useState(false)

  if (!user) return null

  const handleChangePasswordDialog = () => {
    setDialogOpen(!dialogOpen)
  }

  const handleLogout = () => {
    logout()
    navigate('/login')
  }

  return (
    <div className="ml-auto flex items-center gap-4">
      <DropdownMenu>
        <DropdownMenuTrigger className="flex items-center space-x-2 focus:outline-none">
          <Avatar className="h-8 w-8">
            <AvatarFallback>{user.username[0].toUpperCase()}</AvatarFallback>
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
          </RoleGuard>
          <DropdownMenuItem
            className="cursor-pointer"
            onClick={() => handleChangePasswordDialog()}
          >
            Change Own Password
          </DropdownMenuItem>
          <DropdownMenuItem className="cursor-pointer" onClick={handleLogout}>
            Logout
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
      <ChangeOnwnPasswordDialog
        open={dialogOpen}
        onOpenChange={handleChangePasswordDialog}
      />
    </div>
  )
}
