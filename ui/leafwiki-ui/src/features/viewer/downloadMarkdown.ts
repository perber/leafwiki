import type { Page } from '@/lib/api/pages'

function sanitizeMarkdownFilename(value: string): string {
  const sanitized = value
    .trim()
    .replace(/\.md$/i, '')
    .replace(/[\\/:*?"<>|]+/g, '-')
    .replace(/\s+/g, '-')
    .replace(/-+/g, '-')
    .replace(/^-|-$/g, '')
    .toLowerCase()

  return sanitized || 'page'
}

export function getMarkdownDownloadFilename(page: Page): string {
  const pathSegment = page.path.split('/').filter(Boolean).pop()
  return `${sanitizeMarkdownFilename(pathSegment || page.slug || page.title)}.md`
}

export function downloadPageMarkdown(page: Page) {
  const blob = new Blob([page.content], {
    type: 'text/markdown;charset=utf-8',
  })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')

  link.href = url
  link.download = getMarkdownDownloadFilename(page)
  link.style.display = 'none'

  document.body.appendChild(link)
  link.click()
  link.remove()

  URL.revokeObjectURL(url)
}
