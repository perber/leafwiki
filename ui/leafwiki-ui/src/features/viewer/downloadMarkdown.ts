import { downloadPageFile, NODE_KIND_SECTION, type Page } from '@/lib/api/pages'

function sanitizeDownloadFilename(value: string): string {
  const sanitized = value
    .trim()
    .replace(/\.(md|zip)$/i, '')
    .replace(/[\\/:*?"<>|]+/g, '-')
    .replace(/\s+/g, '-')
    .replace(/-+/g, '-')
    .replace(/^-|-$/g, '')
    .toLowerCase()

  return sanitized || 'page'
}

// getDownloadFallbackFilename derives a client-side filename used only when the
// server response omits a Content-Disposition header. Sections are downloaded
// as a ZIP archive of their subtree, pages as Markdown.
export function getDownloadFallbackFilename(page: Page): string {
  const pathSegment = page.path.split('/').filter(Boolean).pop()
  const base = sanitizeDownloadFilename(pathSegment || page.slug || page.title)
  const extension = page.kind === NODE_KIND_SECTION ? 'zip' : 'md'
  return `${base}.${extension}`
}

function triggerBlobDownload(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')

  link.href = url
  link.download = filename
  link.style.display = 'none'

  document.body.appendChild(link)
  link.click()
  link.remove()

  URL.revokeObjectURL(url)
}

// downloadPageMarkdown downloads a node through the API: a page is saved as a
// Markdown (.md) file, a section as a ZIP (.zip) archive of its whole subtree.
// The file is saved with the filename advertised by the server's
// Content-Disposition header, falling back to a client-derived name.
export async function downloadPageMarkdown(page: Page): Promise<void> {
  const { blob, filename } = await downloadPageFile(page.id)
  triggerBlobDownload(blob, filename || getDownloadFallbackFilename(page))
}
