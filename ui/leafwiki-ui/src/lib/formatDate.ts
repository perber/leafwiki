import { formatDistanceToNow } from 'date-fns'

export function formatRelativeTime(value?: string) {
  if (!value) return ''
  const date = new Date(value)
  if (isNaN(date.getTime())) return ''
  return formatDistanceToNow(date, { addSuffix: true })
}
