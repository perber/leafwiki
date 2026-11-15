import { mapApiError } from '@/lib/api/errors'
import type { PageDisplayPatch } from '@/lib/pageDisplay'
import { useConfigStore } from '@/stores/config'
import { useState } from 'react'
import { toast } from 'sonner'

type Messages = {
  // Success toast.
  saved: string
  // Fallback error toast when the server error has no translation.
  failed: string
}

// One section of the Display settings page: saves its patch through the
// config store, toasts the outcome and tracks its own saving flag, so each
// section can be in flight independently of the others.
export function useDisplaySettingSave(messages: Messages) {
  const updatePageDisplay = useConfigStore((s) => s.updatePageDisplay)
  const [saving, setSaving] = useState(false)

  const save = async (patch: PageDisplayPatch) => {
    setSaving(true)
    try {
      await updatePageDisplay(patch)
      toast.success(messages.saved)
    } catch (err) {
      toast.error(mapApiError(err, messages.failed).message)
    } finally {
      setSaving(false)
    }
  }

  return { saving, save }
}
