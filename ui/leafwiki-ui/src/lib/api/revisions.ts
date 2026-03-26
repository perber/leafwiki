import { fetchWithAuth } from './auth'

export type RevisionUserLabel = {
  id: string
  username: string
}

export type Revision = {
  id: string
  pageId: string
  parentId?: string
  type: string
  authorId: string
  author?: RevisionUserLabel
  createdAt: string
  title: string
  slug: string
  kind: string
  path: string
  contentHash: string
  assetManifestHash: string
  pageCreatedAt?: string
  pageUpdatedAt?: string
  creatorId?: string
  lastAuthorId?: string
  summary?: string
}

export type RevisionAsset = {
  name: string
  sha256: string
  sizeBytes: number
  mimeType?: string
}

export type RevisionSnapshot = {
  revision: Revision
  content: string
  assets: RevisionAsset[]
}

export type RevisionAssetChange = {
  name: string
  status: 'added' | 'removed' | 'modified'
}

export type RevisionComparison = {
  base: RevisionSnapshot
  target: RevisionSnapshot
  contentChanged: boolean
  assetChanges: RevisionAssetChange[]
}

export type RevisionListResponse = {
  revisions: Revision[]
  nextCursor: string
}

export async function listRevisions(
  pageId: string,
  cursor = '',
  limit = 50,
): Promise<RevisionListResponse> {
  const params = new URLSearchParams()
  if (cursor) params.set('cursor', cursor)
  params.set('limit', String(limit))
  const query = params.toString()
  return (await fetchWithAuth(
    `/api/pages/${pageId}/revisions${query ? `?${query}` : ''}`,
  )) as RevisionListResponse
}

export async function getLatestRevision(pageId: string): Promise<Revision> {
  return (await fetchWithAuth(`/api/pages/${pageId}/revisions/latest`)) as Revision
}

export async function getRevisionSnapshot(
  pageId: string,
  revisionId: string,
): Promise<RevisionSnapshot> {
  return (await fetchWithAuth(
    `/api/pages/${pageId}/revisions/${revisionId}`,
  )) as RevisionSnapshot
}

export async function compareRevisions(
  pageId: string,
  baseRevisionId: string,
  targetRevisionId: string,
): Promise<RevisionComparison> {
  const params = new URLSearchParams({
    base: baseRevisionId,
    target: targetRevisionId,
  })
  return (await fetchWithAuth(
    `/api/pages/${pageId}/revisions/compare?${params.toString()}`,
  )) as RevisionComparison
}
