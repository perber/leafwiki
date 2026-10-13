import { fetchWithAuth } from './auth'

export const NODE_KIND_PAGE = 'page'
export const NODE_KIND_SECTION = 'section'

export type PageMetadata = {
  createdAt: string
  updatedAt: string
  creatorId: string
  lastAuthorId: string
  creator?: {
    id: string
    username: string
  }
  lastAuthor?: {
    id: string
    username: string
  }
}

export type PageNode = {
  id: string
  title: string
  slug: string
  path: string
  version: string
  parentId?: string | null
  children: PageNode[] | null
  kind: 'page' | 'section'
  pinned?: boolean
  metadata?: PageMetadata // optional metadata, because older API responses may not have it
}

export interface Page {
  id: string
  slug: string
  path: string
  title: string
  content: string
  tags?: string[]
  properties?: Record<string, string>
  version: string
  kind: 'page' | 'section'
  metadata?: PageMetadata // optional metadata, because older API responses may not have it
}

export type PermalinkTarget = {
  id: string
  slug: string
  path: string
}

export type PageRefactorKind = 'rename' | 'move'

export type PageRefactorAffectedPage = {
  fromPageId: string
  fromTitle: string
  fromPath: string
  matchedPaths: string[]
  warnings: string[]
}

export type PageRefactorPreview = {
  kind: PageRefactorKind
  pageId: string
  oldPath: string
  newPath: string
  affectedPages: PageRefactorAffectedPage[]
  counts: {
    affectedPages: number
    matchedLinks: number
  }
  warnings: string[]
}

export async function fetchTree(): Promise<PageNode> {
  return await fetchWithAuth<PageNode>(`/api/tree`)
}

export async function suggestSlug(
  parentId: string,
  title: string,
  currentId?: string,
): Promise<string> {
  if (!currentId) currentId = ''

  const data = await fetchWithAuth<{ slug: string }>(
    `/api/pages/slug-suggestion?parentId=${parentId}&title=${encodeURIComponent(title)}${currentId ? `&currentId=${currentId}` : ''}`,
  )
  return data.slug
}

export async function getPageByPath(
  path: string,
  signal?: AbortSignal,
): Promise<Page> {
  return await fetchWithAuth<Page>(
    `/api/pages/by-path?path=${encodeURIComponent(path)}`,
    { signal },
  )
}

export async function getPermalinkTarget(id: string): Promise<PermalinkTarget> {
  return await fetchWithAuth<PermalinkTarget>(
    `/api/pages/permalink/${encodeURIComponent(id)}`,
  )
}

export async function createPage({
  title,
  slug,
  parentId,
  kind,
}: {
  title: string
  slug: string
  parentId: string | null
  kind: 'page' | 'section'
}) {
  if (parentId === '') parentId = null

  return await fetchWithAuth(`/api/pages`, {
    method: 'POST',
    body: JSON.stringify({ title, slug, parentId, kind }),
  })
}

export async function copyPage(
  id: string,
  targetParentId: string | null,
  targetTitle: string,
  targetSlug: string,
) {
  if (targetParentId === '' || targetParentId === 'root') targetParentId = null
  return await fetchWithAuth(`/api/pages/copy/${id}`, {
    method: 'POST',
    body: JSON.stringify({
      targetParentId,
      title: targetTitle,
      slug: targetSlug,
    }),
  })
}

export async function updatePage(
  id: string,
  version: string,
  title: string,
  slug: string,
  content: string,
  tags: string[],
  properties: Record<string, string>,
): Promise<Page | null> {
  return await fetchWithAuth<Page | null>(`/api/pages/${id}`, {
    method: 'PUT',
    body: JSON.stringify({ version, title, slug, content, tags, properties }),
  })
}

export async function deletePage(
  id: string,
  recursive: boolean,
  version: string,
) {
  if (recursive === undefined) recursive = false

  const params = new URLSearchParams({
    recursive: recursive ? 'true' : 'false',
  })
  if (version) params.set('version', version)

  return await fetchWithAuth(`/api/pages/${id}?${params.toString()}`, {
    method: 'DELETE',
  })
}

export async function movePage(
  id: string,
  version: string,
  parentId: string | null,
  position?: number,
) {
  if (parentId === '' || parentId == 'root') parentId = null

  return await fetchWithAuth(`/api/pages/${id}/move`, {
    method: 'PUT',
    body: JSON.stringify(
      position === undefined
        ? { version, parentId }
        : { version, parentId, position },
    ),
  })
}

export async function previewPageRefactor(
  id: string,
  payload:
    | {
        kind: 'rename'
        title: string
        slug: string
      }
    | {
        kind: 'move'
        parentId: string | null
      },
): Promise<PageRefactorPreview> {
  return await fetchWithAuth<PageRefactorPreview>(
    `/api/pages/${id}/refactor/preview`,
    {
      method: 'POST',
      body: JSON.stringify(payload),
    },
  )
}

export async function applyPageRefactor(
  id: string,
  payload:
    | {
        kind: 'rename'
        version: string
        title: string
        slug: string
        content: string
        rewriteLinks: boolean
      }
    | {
        kind: 'move'
        version: string
        parentId: string | null
        rewriteLinks: boolean
        position?: number
      },
): Promise<Page | null> {
  return await fetchWithAuth<Page | null>(`/api/pages/${id}/refactor/apply`, {
    method: 'POST',
    body: JSON.stringify(payload),
  })
}

export async function pinPage(
  id: string,
  version: string,
  pinned: boolean,
): Promise<Page> {
  return await fetchWithAuth<Page>(`/api/pages/${id}/pin`, {
    method: 'PUT',
    body: JSON.stringify({ version, pinned }),
  })
}

export async function sortPages(parentId: string, orderedIDs: string[]) {
  if (parentId === '') parentId = 'root'

  return await fetchWithAuth(`/api/pages/${parentId}/sort`, {
    method: 'PUT',
    body: JSON.stringify({ orderedIDs }),
  })
}

export async function convertPage(
  id: string,
  targetKind: 'page' | 'section',
  version: string,
) {
  return await fetchWithAuth(`/api/pages/convert/${id}`, {
    method: 'POST',
    body: JSON.stringify({ targetKind, version }),
  })
}

export type PathLookupResult = {
  path: string
  exists: boolean
  canCreate: boolean
  segments: { slug: string; id?: string; exists: boolean }[]
}

export async function lookupPath(path: string): Promise<PathLookupResult> {
  return await fetchWithAuth<PathLookupResult>(
    `/api/pages/lookup?path=${encodeURIComponent(path)}`,
  )
}

export async function ensurePage(path: string, targetTitle: string) {
  return await fetchWithAuth(`/api/pages/ensure`, {
    method: 'POST',
    body: JSON.stringify({ path, title: targetTitle }),
  })
}
