import BaseDialog from '@/components/BaseDialog'
import { FormInput } from '@/components/FormInput'
import { Button } from '@/components/ui/button'
import {
  MCPAPIKey,
  User,
  createOwnMCPAPIKey,
  createUserMCPAPIKey,
  getOwnMCPAPIKeys,
  getUserMCPAPIKeys,
  revokeOwnMCPAPIKey,
  revokeUserMCPAPIKey,
} from '@/lib/api/users'
import { handleFieldErrors } from '@/lib/handleFieldErrors'
import { DIALOG_MCP_API_KEYS } from '@/lib/registries'
import { useSessionStore } from '@/stores/session'
import copy from 'copy-to-clipboard'
import { Copy, KeyRound, Trash2 } from 'lucide-react'
import { useCallback, useEffect, useMemo, useState } from 'react'
import { toast } from 'sonner'

const DIALOG_INPUT_ALLOWED_HOTKEYS = 'Enter'

type MCPAPIKeysDialogProps = {
  mode: 'admin' | 'self'
  user?: User
  selfCreateDisabled?: boolean
}

export function MCPAPIKeysDialog({
  mode,
  user,
  selfCreateDisabled = false,
}: MCPAPIKeysDialogProps) {
  const currentUser = useSessionStore((s) => s.user)
  const isSelf = mode === 'self'
  const owner = isSelf ? currentUser : user
  const canCreate = !isSelf || !selfCreateDisabled
  const [keys, setKeys] = useState<MCPAPIKey[]>([])
  const [loadingKeys, setLoadingKeys] = useState(true)
  const [loadError, setLoadError] = useState('')
  const [creating, setCreating] = useState(false)
  const [revokingKeyId, setRevokingKeyId] = useState<string | null>(null)
  const [name, setName] = useState('')
  const [currentPassword, setCurrentPassword] = useState('')
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({})
  const [secret, setSecret] = useState('')

  const title = useMemo(() => {
    if (isSelf) return 'MCP API Keys'
    return owner ? `MCP API Keys: ${owner.username}` : 'MCP API Keys'
  }, [isSelf, owner])

  const loadKeys = useCallback(async () => {
    if (!owner) return
    setLoadingKeys(true)
    setLoadError('')
    try {
      const loaded = isSelf
        ? await getOwnMCPAPIKeys()
        : await getUserMCPAPIKeys(owner.id)
      setKeys(loaded)
    } catch (err) {
      console.warn(err)
      setLoadError('Could not load API keys.')
      toast.error('Error loading API keys')
    } finally {
      setLoadingKeys(false)
    }
  }, [isSelf, owner])

  useEffect(() => {
    void loadKeys()
  }, [loadKeys])

  const resetForm = useCallback((): boolean => {
    if (creating) return false
    setName('')
    setCurrentPassword('')
    setFieldErrors({})
    setSecret('')
    return true
  }, [creating])

  if (!owner) return null

  const handleCreate = async (): Promise<boolean> => {
    setCreating(true)
    try {
      const created = isSelf
        ? await createOwnMCPAPIKey(name, currentPassword)
        : await createUserMCPAPIKey(owner.id, name)
      setKeys((prev) => [created.key, ...prev])
      setSecret(created.secret)
      setName('')
      setCurrentPassword('')
      setFieldErrors({})
      toast.success('API key created')
      return false
    } catch (err) {
      console.warn(err)
      handleFieldErrors(err, setFieldErrors, 'Error creating API key')
      return false
    } finally {
      setCreating(false)
    }
  }

  const handleRevoke = async (keyId: string) => {
    setRevokingKeyId(keyId)
    try {
      if (isSelf) {
        await revokeOwnMCPAPIKey(keyId)
      } else {
        await revokeUserMCPAPIKey(owner.id, keyId)
      }
      setKeys((prev) => prev.filter((key) => key.id !== keyId))
      toast.success('API key revoked')
    } catch (err) {
      console.warn(err)
      toast.error('Error revoking API key')
    } finally {
      setRevokingKeyId(null)
    }
  }

  const handleCopySecret = () => {
    if (!secret || !copy(secret)) {
      toast.error('Could not copy API key')
      return
    }
    toast.success('API key copied')
  }

  return (
    <BaseDialog
      dialogType={DIALOG_MCP_API_KEYS}
      dialogTitle={title}
      dialogDescription="Manage Bearer credentials for local MCP clients."
      onClose={resetForm}
      onConfirm={async (actionType): Promise<boolean> => {
        if (actionType === 'create' || actionType === 'confirm') {
          return await handleCreate()
        }
        return false
      }}
      testidPrefix="mcp-api-keys-dialog"
      cancelButton={{ label: 'Close', variant: 'outline', disabled: creating }}
      buttons={
        canCreate
          ? [
              {
                label: creating ? 'Creating...' : 'Create',
                actionType: 'create',
                loading: creating,
                disabled:
                  creating ||
                  loadingKeys ||
                  !!loadError ||
                  !name.trim() ||
                  (isSelf && currentPassword.trim() === ''),
              },
            ]
          : []
      }
      contentClassName="sm:max-w-2xl"
    >
      <div className="space-y-5 pt-2">
        {canCreate && (
          <div className="grid gap-3 md:grid-cols-[1fr_auto] md:items-end">
            <FormInput
              autoFocus={true}
              label="Name"
              name="mcp-api-key-name"
              value={name}
              testid="mcp-api-keys-dialog-name-input"
              onChange={(val) => {
                setName(val)
                setFieldErrors((prev) => ({ ...prev, name: '' }))
              }}
              placeholder="Codex desktop"
              error={fieldErrors.name}
              allowedHotkeys={DIALOG_INPUT_ALLOWED_HOTKEYS}
            />
            {isSelf && (
              <FormInput
                label="Current Password"
                name="current-password"
                type="password"
                value={currentPassword}
                testid="mcp-api-keys-dialog-current-password-input"
                onChange={(val) => {
                  setCurrentPassword(val)
                  setFieldErrors((prev) => ({ ...prev, currentPassword: '' }))
                }}
                placeholder="Current password"
                autoComplete="current-password"
                error={fieldErrors.currentPassword}
                allowedHotkeys={DIALOG_INPUT_ALLOWED_HOTKEYS}
              />
            )}
          </div>
        )}
        {!canCreate && (
          <p className="text-muted text-sm">
            Creating keys is unavailable for HTTP remote-user sign-in.
          </p>
        )}

        {secret && (
          <div className="space-y-2">
            <FormInput
              label="New API Key"
              value={secret}
              onChange={() => {}}
              readOnly={true}
              testid="mcp-api-keys-dialog-secret-input"
            />
            <div className="flex justify-end">
              <Button
                type="button"
                variant="outline"
                onClick={handleCopySecret}
                data-testid="mcp-api-keys-dialog-copy-secret"
              >
                <Copy size={16} />
                Copy
              </Button>
            </div>
          </div>
        )}

        <div className="space-y-2">
          {loadingKeys && <p className="text-muted text-sm">Loading keys...</p>}
          {!loadingKeys && loadError && (
            <div className="border-surface-border flex items-center justify-between gap-3 border-t py-3 first:border-t-0">
              <p className="text-muted text-sm">{loadError}</p>
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={() => void loadKeys()}
              >
                Retry
              </Button>
            </div>
          )}
          {!loadingKeys && !loadError && keys.length === 0 && (
            <p className="text-muted text-sm">No active keys.</p>
          )}
          {!loadingKeys &&
            !loadError &&
            keys.map((key) => (
              <div
                key={key.id}
                className="border-surface-border flex items-center justify-between gap-3 border-t py-3 first:border-t-0"
                data-testid={`mcp-api-key-row-${key.id}`}
              >
                <div className="min-w-0">
                  <div className="text-interface-text flex items-center gap-2 text-sm font-medium">
                    <KeyRound size={16} />
                    <span className="truncate">{key.name}</span>
                  </div>
                  <div className="text-muted mt-1 text-xs">
                    {key.prefix}...{key.last4}
                    {key.lastUsedAt
                      ? ` · Last used ${formatDate(key.lastUsedAt)}`
                      : ''}
                  </div>
                </div>
                <Button
                  type="button"
                  size="sm"
                  variant="destructive"
                  disabled={revokingKeyId === key.id}
                  onClick={() => void handleRevoke(key.id)}
                  aria-label={`Revoke API key ${key.name}`}
                  data-testid={`mcp-api-key-revoke-${key.id}`}
                >
                  <Trash2 size={16} />
                  Revoke
                </Button>
              </div>
            ))}
        </div>
      </div>
    </BaseDialog>
  )
}

function formatDate(value: string) {
  return new Date(value).toLocaleString()
}
