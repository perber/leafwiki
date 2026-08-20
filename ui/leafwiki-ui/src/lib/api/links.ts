import i18next from '@/lib/i18n'
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

export async function fetchLinkStatus(
  pageId: string,
): Promise<LinkStatusResult> {
  if (!pageId)
    throw new Error(i18next.t('backlinks.pageIdRequired', { ns: 'viewer' }))
  return (await fetchWithAuth(`/api/pages/${pageId}/links`)) as LinkStatusResult
}

export type BrokenLink = {
  from_page_id: string
  from_path: string
  from_title: string

  to_path: string
  to_title: string
}

export type BrokenLinksResult = {
  links: BrokenLink[]
}

export async function fetchBrokenLinks(): Promise<BrokenLinksResult> {
  return (await fetchWithAuth('/api/links/broken')) as BrokenLinksResult
}
