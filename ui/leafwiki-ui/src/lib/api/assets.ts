import { fetchWithAuth } from './auth'

export type UploadAssetResponse = {
  file: string
}

export async function uploadAsset(
  pageId: string,
  file: File,
): Promise<UploadAssetResponse> {
  const form = new FormData()
  form.append('file', file)
  try {
    return (await fetchWithAuth(`/api/pages/${pageId}/assets`, {
      method: 'POST',
      body: form,
    })) as UploadAssetResponse
  } catch {
    throw new Error('Asset upload failed')
  }
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
