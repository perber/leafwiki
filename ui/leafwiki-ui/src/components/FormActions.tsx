import { Button } from '@/components/ui/button'
import { Loader2 } from 'lucide-react'

export function FormActions({
  children,
  onCancel,
  onSave,
  testidPrefix,
  saveLabel = 'Save',
  saveVariant = 'default',
  loading = false,
  disabled = false,
  autoFocus = '',
}: {
  children?: React.ReactNode
  onCancel: () => void
  onSave: () => void
  saveVariant?: string
  saveLabel?: string
  loading?: boolean
  disabled?: boolean
  testidPrefix?: string
  autoFocus?: 'cancel' | 'save' | ''
}) {
  return (
    <div className="mt-6 flex justify-end gap-2">
      <Button
        variant="outline"
        onClick={onCancel}
        disabled={loading}
        autoFocus={autoFocus == 'cancel'}
        data-testid={testidPrefix ? `${testidPrefix}-cancel-button` : undefined}
      >
        Cancel
      </Button>
      {children}
      <Button
        onClick={onSave}
        disabled={loading || disabled}
        variant={
          saveVariant as
            | 'default'
            | 'destructive'
            | 'outline'
            | 'ghost'
            | 'link'
            | 'secondary'
        }
        autoFocus={autoFocus == 'save'}
        data-testid={testidPrefix ? `${testidPrefix}-save-button` : undefined}
      >
        {loading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
        {saveLabel}
      </Button>
    </div>
  )
}
