// Page content widths an admin can choose under Settings > Display. Mirrors
// the backend enum in internal/pagedisplay/content_width.go; each value maps
// to a [data-content-width] rule in index.css. A new width is added in all
// three places (and gets an option label in the `display` locale namespace).
export const CONTENT_WIDTHS = ['default', 'wide'] as const

export type ContentWidth = (typeof CONTENT_WIDTHS)[number]

export const DEFAULT_CONTENT_WIDTH: ContentWidth = 'default'

export function isContentWidth(value: unknown): value is ContentWidth {
  return (
    typeof value === 'string' &&
    (CONTENT_WIDTHS as readonly string[]).includes(value)
  )
}

// Falls back to the default for a missing field (older server) or a value
// this frontend doesn't know.
export function toContentWidth(value: unknown): ContentWidth {
  return isContentWidth(value) ? value : DEFAULT_CONTENT_WIDTH
}
