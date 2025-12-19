import { fetchWithAuth } from './auth'

export type BacklinkResult = {
  count: number
  backlinks: Backlink[]
}

export type Backlink = {
  from_page_id: string
  from_path: string
  to_page_id: string
  from_title: string
  broken: boolean
}

export async function fetchBacklinks(pageId: string): Promise<BacklinkResult> {
  if (!pageId) throw new Error('Page ID is required')
  return (await fetchWithAuth(
    `/api/pages/${pageId}/backlinks`,
  )) as Promise<BacklinkResult>
}

export type OutgoingResult = {
  count: number
  outgoings: OutgoingLinks[]
}

export type OutgoingLinks = {
  from_page_id: string
  to_page_id: string
  to_path: string
  to_page_title: string
  broken: boolean
}
export async function fetchOutgoingLinks(
  pageId: string,
): Promise<OutgoingResult> {
  if (!pageId) throw new Error('Page ID is required')
  return (await fetchWithAuth(
    `/api/pages/${pageId}/outgoing-links`,
  )) as Promise<OutgoingResult>
}
