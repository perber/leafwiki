import { IMAGE_EXTENSIONS } from '@/lib/config'
import { fetchWithAuth } from './auth'

export type UploadAssetResponse = {
  file: string
}

export type PageAttachment = {
  name: string
  url: string
}

function normalizeAssetUrl(path: string): string {
  if (path.startsWith('/assets/')) return path
  if (path.startsWith('assets/')) return `/${path}`
  return `/assets/${path}`
}

export function listNonImageAttachments(files: string[]): PageAttachment[] {
  const seen = new Set<string>()
  const attachments: PageAttachment[] = []
  for (const file of files) {
    const url = normalizeAssetUrl(file)
    const name = url.split('/').pop() ?? url
    const ext = name.split('.').pop()?.toLowerCase() ?? ''
    if (IMAGE_EXTENSIONS.includes(ext) || seen.has(url)) continue
    seen.add(url)
    attachments.push({ name, url })
  }
  return attachments
}

// Session-scoped cache for the viewer's attachment panel. The SPA remounts
// PageViewer on every visit, so navigating back and forth between pages would
// otherwise refetch GET /api/pages/:id/assets each time even though the result
// almost never changes between visits. Entries are busted whenever an asset is
// uploaded, renamed, or deleted through the mutating helpers below.
const attachmentCache = new Map<string, PageAttachment[]>()
const attachmentRequests = new Map<string, Promise<PageAttachment[]>>()

/** Drops any cached attachment list for a page so the next read refetches. */
export function invalidatePageAttachments(pageId: string): void {
  attachmentCache.delete(pageId)
  attachmentRequests.delete(pageId)
}

export async function getPageAttachments(
  pageId: string,
): Promise<PageAttachment[]> {
  const cached = attachmentCache.get(pageId)
  if (cached) return cached

  const inFlight = attachmentRequests.get(pageId)
  if (inFlight) return inFlight

  const request = getAssets(pageId)
    .then((files) => {
      const attachments = listNonImageAttachments(files)
      attachmentCache.set(pageId, attachments)
      return attachments
    })
    .finally(() => {
      attachmentRequests.delete(pageId)
    })

  attachmentRequests.set(pageId, request)
  return request
}

export async function uploadAsset(
  pageId: string,
  file: File,
): Promise<UploadAssetResponse> {
  const form = new FormData()
  form.append('file', file)
  const res = (await fetchWithAuth(`/api/pages/${pageId}/assets`, {
    method: 'POST',
    body: form,
  })) as UploadAssetResponse
  invalidatePageAttachments(pageId)
  return res
}

export async function getAssets(pageId: string): Promise<string[]> {
  const data = await fetchWithAuth(`/api/pages/${pageId}/assets`, {})
  const typedData = data as { files: string[] }
  return typedData.files
}

export async function deleteAsset(pageId: string, filename: string) {
  const res = await fetchWithAuth(
    `/api/pages/${pageId}/assets/${encodeURIComponent(filename)}`,
    {
      method: 'DELETE',
    },
  )
  invalidatePageAttachments(pageId)
  return res
}

export async function renameAsset(
  pageId: string,
  oldFilename: string,
  newFilename: string,
) {
  const res = await fetchWithAuth(`/api/pages/${pageId}/assets/rename`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      old_filename: oldFilename,
      new_filename: newFilename,
    }),
  })
  invalidatePageAttachments(pageId)
  return res
}
