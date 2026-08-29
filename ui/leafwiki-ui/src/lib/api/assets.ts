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

export async function getPageAttachments(
  pageId: string,
): Promise<PageAttachment[]> {
  const files = await getAssets(pageId)
  return listNonImageAttachments(files)
}

export async function uploadAsset(
  pageId: string,
  file: File,
): Promise<UploadAssetResponse> {
  const form = new FormData()
  form.append('file', file)
  return (await fetchWithAuth(`/api/pages/${pageId}/assets`, {
    method: 'POST',
    body: form,
  })) as UploadAssetResponse
}

export async function getAssets(pageId: string): Promise<string[]> {
  const data = await fetchWithAuth(`/api/pages/${pageId}/assets`, {})
  const typedData = data as { files: string[] }
  return typedData.files
}

export async function deleteAsset(pageId: string, filename: string) {
  return await fetchWithAuth(
    `/api/pages/${pageId}/assets/${encodeURIComponent(filename)}`,
    {
      method: 'DELETE',
    },
  )
}

export async function renameAsset(
  pageId: string,
  oldFilename: string,
  newFilename: string,
) {
  return await fetchWithAuth(`/api/pages/${pageId}/assets/rename`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      old_filename: oldFilename,
      new_filename: newFilename,
    }),
  })
}
