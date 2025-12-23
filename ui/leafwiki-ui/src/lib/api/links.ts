import { fetchWithAuth } from './auth'

export type Backlink = {
  from_page_id: string
  from_path: string
  to_page_id: string
  from_title: string
  broken: boolean
}

export type OutgoingLink = {
  from_page_id: string
  to_page_id: string
  to_path: string
  to_page_title: string
  broken: boolean
}

export type LinkStatusCounts = {
  backlinks: number
  broken_incoming: number
  outgoings: number
  broken_outgoings: number
}

export type LinkStatusResult = {
  backlinks: Backlink[]
  broken_incoming: Backlink[]
  outgoings: OutgoingLink[]
  broken_outgoings: OutgoingLink[]
  counts: LinkStatusCounts
}

export async function fetchLinkStatus(pageId: string): Promise<LinkStatusResult> {
  if (!pageId) throw new Error('Page ID is required')
  return (await fetchWithAuth(`/api/pages/${pageId}/links`)) as LinkStatusResult
}