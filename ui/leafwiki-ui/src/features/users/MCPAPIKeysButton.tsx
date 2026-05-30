import { Button } from '@/components/ui/button'
import { User } from '@/lib/api/users'
import { DIALOG_MCP_API_KEYS } from '@/lib/registries'
import { useDialogsStore } from '@/stores/dialogs'
import { KeyRound } from 'lucide-react'

type MCPAPIKeysButtonProps = {
  user: User
}

export function MCPAPIKeysButton({ user }: MCPAPIKeysButtonProps) {
  const openDialog = useDialogsStore((s) => s.openDialog)

  return (
    <Button
      size="sm"
      variant="outline"
      onClick={() => openDialog(DIALOG_MCP_API_KEYS, { mode: 'admin', user })}
    >
      <KeyRound size={16} />
      MCP API Keys
    </Button>
  )
}
