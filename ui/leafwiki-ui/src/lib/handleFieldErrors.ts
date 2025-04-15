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
  err: any,
  setFieldErrors?: (errors: Record<string, string>) => void,
  fallbackMessage = 'Something went wrong',
) {
  const error = err as APIError

  console.log('Error:', error)

  if (error.error === 'validation_error' && Array.isArray(error.fields)) {
    const errorMap: Record<string, string> = {}
    for (const e of error.fields) {
      errorMap[e.field] = e.message
    }
    setFieldErrors?.(errorMap)
    toast.error('Validation failed')
  } else if (error.error) {
    toast.error(error.error)
  } else {
    toast.error(fallbackMessage)
  }
}
