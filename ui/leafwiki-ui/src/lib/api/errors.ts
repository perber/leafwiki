export type ApiLocalizedErrorDetail = {
  code: string
  message: string
  template: string
  args?: string[]
}

export type ApiLocalizedErrorResponse = {
  error: ApiLocalizedErrorDetail
}

export type ApiUiError = {
  message: string
  detail?: string
  code?: string
}

export class ApiLocalizedError extends Error {
  code: string
  template: string
  args: string[]

  constructor(detail: ApiLocalizedErrorDetail) {
    super(detail.message)
    this.name = 'ApiLocalizedError'
    this.code = detail.code
    this.template = detail.template
    this.args = detail.args ?? []
  }
}

export function isApiLocalizedErrorResponse(
  value: unknown,
): value is ApiLocalizedErrorResponse {
  if (!value || typeof value !== 'object') return false

  const error = (value as { error?: unknown }).error
  if (!error || typeof error !== 'object') return false

  const detail = error as Partial<ApiLocalizedErrorDetail>
  return (
    typeof detail.code === 'string' &&
    typeof detail.message === 'string' &&
    typeof detail.template === 'string'
  )
}

export function asApiLocalizedError(err: unknown): ApiLocalizedError | null {
  if (err instanceof ApiLocalizedError) {
    return err
  }
  return null
}

export function formatLocalizedErrorTemplate(
  template: string,
  args: string[] = [],
): string {
  if (!template) return ''

  let argIndex = 0
  const formatted = template.replace(/%s/g, () => {
    const nextArg = args[argIndex]
    argIndex += 1
    return nextArg ?? '%s'
  })

  if (argIndex >= args.length) {
    return formatted
  }

  return `${formatted} (${args.slice(argIndex).join(', ')})`
}

export function mapApiError(err: unknown, fallback: string): ApiUiError {
  const localized = asApiLocalizedError(err)
  if (localized) {
    const detail = formatLocalizedErrorTemplate(
      localized.template,
      localized.args,
    )

    return {
      message: localized.message || fallback,
      detail: detail && detail !== localized.message ? detail : undefined,
      code: localized.code,
    }
  }

  return {
    message: getErrorMessage(err, fallback),
  }
}

export function getErrorMessage(err: unknown, fallback: string): string {
  if (err instanceof ApiLocalizedError) {
    return err.message
  }
  if (err instanceof Error && err.message) {
    return err.message
  }
  return fallback
}
