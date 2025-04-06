import { Button } from '@/components/ui/button'
import { Loader2 } from 'lucide-react'

export function FormActions({
  onCancel,
  onSave,
  saveLabel = 'Save',
  loading = false,
  disabled = false,
}: {
  onCancel: () => void
  onSave: () => void
  saveLabel?: string
  loading?: boolean
  disabled?: boolean
}) {
  return (
    <div className="mt-6 flex justify-end gap-2">
      <Button variant="outline" onClick={onCancel} disabled={loading}>
        Cancel
      </Button>
      <Button onClick={onSave} disabled={loading || disabled}>
        {loading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
        {saveLabel}
      </Button>
    </div>
  )
}
