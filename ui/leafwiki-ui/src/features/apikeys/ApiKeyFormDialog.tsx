import BaseDialog from '@/components/BaseDialog'
import { Button } from '@/components/ui/button'
import { FormInput } from '@/components/FormInput'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import type { CreateApiKeyResult } from '@/lib/api/apikeys'
import { handleFieldErrors } from '@/lib/handleFieldErrors'
import { DIALOG_API_KEY_FORM } from '@/lib/registries'
import { useApiKeyStore } from '@/stores/apikeys'
import { useUserStore } from '@/stores/users'
import { useState } from 'react'
import { toast } from 'sonner'

const DIALOG_INPUT_ALLOWED_HOTKEYS = 'Enter'

// expiresAt holds a plain "YYYY-MM-DD" from the date input — the user's
// LOCAL calendar date, with no timezone attached (that's what a date picker
// means: "the day I clicked," not a UTC date). The backend expects RFC3339
// and rejects a non-future expiry, so this normalizes to the end of that day
// — but end of day in the user's OWN timezone, not UTC. Anchoring to UTC
// directly (`${dateOnly}T23:59:59Z`) is wrong two ways: for timezones west of
// UTC it can still land in the past for "today", and for timezones east of
// UTC the resulting instant lands in the small hours of the *next* local day,
// so a key the admin picked "Jan 1" for shows as expiring "Jan 2" once
// rendered back in their own timezone. Parsing the string WITHOUT a "Z"
// suffix makes the browser treat it as local time (a plain ECMA-262
// date-time-without-offset string is always local, unlike a date-only string
// which is UTC — the exact opposite of what's needed here), so this
// constructs local end-of-day and only then converts to the UTC instant the
// wire format needs.
function toExpiresAtRFC3339(dateOnly: string): string | undefined {
  if (!dateOnly) return undefined
  return new Date(`${dateOnly}T23:59:59`).toISOString()
}

export function ApiKeyFormDialog() {
  const [name, setName] = useState('')
  const [userId, setUserId] = useState('')
  const [expiresAt, setExpiresAt] = useState('')
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({})
  const [loading, setLoading] = useState(false)
  const [result, setResult] = useState<CreateApiKeyResult | null>(null)

  const { createApiKey } = useApiKeyStore()
  const users = useUserStore((s) => s.users)

  const handleCreate = async (): Promise<boolean> => {
    if (!name || !userId) return false // Should not happen due to button disabling

    setLoading(true)
    try {
      const created = await createApiKey({
        name,
        userId,
        // Role is intentionally omitted — the create dialog only mints
        // viewer-scoped keys for now (see Select removal below); the
        // backend defaults to viewer when role is absent. Editor/admin
        // roles are accepted by the API for future/direct use, but every
        // Bearer-authenticated write is currently blocked by CSRF
        // regardless of role, so offering them here would suggest a
        // capability the backend can't yet honor.
        expiresAt: toExpiresAtRFC3339(expiresAt),
      })
      toast.success('API key created successfully')
      setResult(created)
      return false // Keep the dialog open to reveal the secret
    } catch (err) {
      console.warn(err)
      handleFieldErrors(err, setFieldErrors, 'Error creating API key')
      return false
    } finally {
      setLoading(false)
    }
  }

  const handleCopy = async () => {
    if (!result) return
    await navigator.clipboard.writeText(result.secret)
    toast.success('Copied to clipboard')
  }

  return (
    <BaseDialog
      dialogType={DIALOG_API_KEY_FORM}
      dialogTitle={result ? 'API Key Created' : 'New API Key'}
      dialogDescription={
        result
          ? 'Copy this key now — it will not be shown again.'
          : 'Create a new API key for automation or agent access.'
      }
      onClose={() => true}
      onConfirm={handleCreate}
      testidPrefix="api-key-form-dialog"
      cancelButton={
        result
          ? { label: 'Done', variant: 'default', autoFocus: true }
          : { label: 'Cancel', variant: 'outline', disabled: loading }
      }
      buttons={
        result
          ? []
          : [
              {
                label: 'Create',
                actionType: 'confirm',
                loading,
                disabled: loading || !name || !userId,
              },
            ]
      }
    >
      {result ? (
        <div className="space-y-4 pt-2">
          <FormInput
            label="secret"
            name="secret"
            value={result.secret}
            onChange={() => {}}
            readOnly
            testid="api-key-secret"
          />
          <Button
            type="button"
            variant="outline"
            onClick={handleCopy}
            data-testid="api-key-secret-copy"
          >
            Copy
          </Button>
          <p className="text-muted text-sm">
            This is the only time the full key will be shown. Store it securely.
          </p>
        </div>
      ) : (
        <div className="space-y-4 pt-2">
          <FormInput
            autoFocus={true}
            label="name"
            name="name"
            value={name}
            onChange={(val) => {
              setName(val)
              setFieldErrors((prev) => ({ ...prev, name: '' }))
            }}
            placeholder="e.g. research-agent"
            error={fieldErrors.name}
            allowedHotkeys={DIALOG_INPUT_ALLOWED_HOTKEYS}
          />
          <Select
            value={userId}
            onValueChange={(val) => {
              setUserId(val)
              setFieldErrors((prev) => ({ ...prev, userId: '' }))
            }}
          >
            <SelectTrigger data-testid="api-key-owner-select">
              <SelectValue placeholder="Select an owning user" />
            </SelectTrigger>
            <SelectContent>
              {users.map((user) => (
                <SelectItem key={user.id} value={user.id}>
                  {user.username}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <FormInput
            label="expires at (optional)"
            name="expiresAt"
            type="date"
            value={expiresAt}
            onChange={(val) => {
              setExpiresAt(val)
              setFieldErrors((prev) => ({ ...prev, expiresAt: '' }))
            }}
            error={fieldErrors.expiresAt}
            testid="api-key-expires-at"
          />
          <p className="text-muted text-sm">
            New keys are viewer (read-only) for now.
          </p>
        </div>
      )}
    </BaseDialog>
  )
}
