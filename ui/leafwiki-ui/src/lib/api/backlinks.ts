import { fetchWithAuth } from './auth'

export type BacklinkResult = {
  count: number
  backlinks: Backlinks[]
}

export type Backlinks = {
  from_page_id: string
  from_path: string
  to_page_id: string
  from_title: string
}

export async function fetchBacklinks(pageId: string): Promise<BacklinkResult> {
  if (!pageId) throw new Error('Page ID is required')
  return (await fetchWithAuth(
    `/api/pages/${pageId}/backlinks`,
  )) as Promise<BacklinkResult>
}
