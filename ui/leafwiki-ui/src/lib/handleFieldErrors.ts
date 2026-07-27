import i18next from './i18n'
import { mapApiError } from './api/errors'
import { toast } from 'sonner'

type FieldError = {
  field: string
  message: string
}

type APIError = {
  error?: string
  fields?: FieldError[]
}

/**
 * Handles a validation error response and optionally maps field errors.
 */
export function handleFieldErrors(
  err: unknown,
  setFieldErrors: ((errors: Record<string, string>) => void) | undefined,
  fallbackMessage: string,
) {
  const error = err as APIError

  console.warn('Error:', error)

  if (error.error === 'validation_error' && Array.isArray(error.fields)) {
    const errorMap: Record<string, string> = {}
    for (const e of error.fields) {
      errorMap[e.field] = e.message
    }
    setFieldErrors?.(errorMap)
    toast.error(i18next.t('validation.failedToast', { ns: 'common' }))
  } else {
    const mapped = mapApiError(err, fallbackMessage)
    toast.error(mapped.message)
  }
}
